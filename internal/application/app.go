package application

import (
	"context"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangeService/pkg/telemetry"
	"github.com/sambly/exchangebot/internal/account"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type Application struct {
	Settings model.Settings
	Config   *config.Config
	database *gorm.DB

	Notification *notification.Notification

	exchange exchange.Exchange
	dataFeed *exchange.DataFeed

	Account            *account.Account
	AssetsPrices       *prices.AsetsPrices
	OrderController    *order.Controller
	PaperWallet        *exchange.PaperWallet
	ControllerStrategy *strategy.ControllerStrategy
}

var appLogger = logger.AddFieldsEmpty()

func NewApp(
	ctx context.Context,
	exch exchange.Exchange,
	dataFeed *exchange.DataFeed,
	settings model.Settings,
	db *gorm.DB,
	socketsMessage *notification.SocketsMessage,
	cfg *config.Config,
	notification *notification.Notification) (*Application, error) {

	assetsPrices := prices.NewAssetsPrices(settings.Pairs, settings.ChangePeriods, settings.DeltaPeriods, db)

	baseLimitAsset := 1.0

	account, err := account.NewAccount(exch, assetsPrices, baseLimitAsset)
	if err != nil {
		return nil, err
	}
	paperWallet := exchange.NewPaperWallet(ctx)
	orderController, err := order.NewController(ctx, paperWallet, db, socketsMessage, assetsPrices)
	if err != nil {
		return nil, err
	}

	controllerStrategy, err := strategy.NewControllerStrategy(assetsPrices, settings.ChangePeriods, settings.Pairs, notification, orderController, paperWallet)
	if err != nil {
		return nil, err
	}

	app := &Application{
		Settings: settings,
		Config:   cfg,
		exchange: exch,
		dataFeed: dataFeed,
		database: db,

		AssetsPrices:       assetsPrices,
		Account:            account,
		OrderController:    orderController,
		PaperWallet:        paperWallet,
		ControllerStrategy: controllerStrategy,
	}

	app.PaperWallet.MarketsStat = assetsPrices.MarketsStat

	return app, nil
}

func (app *Application) Run(ctx context.Context) error {

	appLogger.Info("Ожидание предварительной загрузки данных")

	timeStart := time.Now()

	shouldBreak := false
	// Ожидание, пока текущее время не попадет в интервал от 10 до 50 секунд
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeNow := time.Now()
			seconds := timeNow.Second()
			if seconds >= 10 && seconds <= 50 {
				shouldBreak = true
				break
			}
			time.Sleep(1 * time.Second) // Ждем одну секунду перед повторной проверкой
		}
		if shouldBreak {
			break
		}
	}

	timeRounding := time.Now().Truncate(time.Minute)

	app.AssetsPrices.UpdateTime = timeRounding
	app.AssetsPrices.InitChangePrices()
	app.AssetsPrices.InitChangeDelta()

	observers := []func(market exModel.MarketsStat){
		func(market exModel.MarketsStat) {
			app.AssetsPrices.OnMarket(market)
			app.OrderController.OnMarket(market)
			//app.Strategy.OnMarket(market)
		},
	}

	ctx, rootSpan := telemetry.Tracer.Start(ctx, "app")
	defer rootSpan.End()

	g, gCtx := errgroup.WithContext(ctx)

	for _, pair := range app.Settings.Pairs {

		app.dataFeed.SubscribeMarketsStat(pair)
		for _, observer := range observers {
			if err := app.dataFeed.SubscribeObserverMarkets(gCtx, "exchangebot", pair, observer); err != nil {
				appLogger.Error(err)
			}
		}
	}

	g.Go(func() error {
		return app.dataFeed.StartMarketsStatFeeder(gCtx, "exchangebot")
	})

	g.Go(func() error {
		return app.ControllerStrategy.StartAll(gCtx)
	})

	duration := time.Since(timeStart)
	appLogger.Infof("Время выполнения предварительной загрузки данных: %v ", duration)
	appLogger.Infof("Время старта: %v ", timeStart)

	return g.Wait()
}
