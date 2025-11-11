package application

import (
	"context"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangeService/pkg/telemetry"
	"github.com/sambly/exchangebot/internal/account"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/database"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/paperwallet"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type Application struct {
	Settings model.Settings
	Config   *config.Config

	Notification *notification.Notification

	exchange exchange.Exchange
	dataFeed *exchange.DataFeed

	Account      *account.Account
	AssetsPrices *prices.AssetsPrices

	OrderController    *order.OrderService
	PaperWallet        *paperwallet.PaperWallet
	ControllerStrategy *strategy.ControllerStrategy
}

var appLogger = logger.AddFieldsEmpty()

func NewApp(
	exch exchange.Exchange,
	dataFeed *exchange.DataFeed,
	settings model.Settings,
	db *gorm.DB,
	socketsMessage *notification.SocketsMessage,
	cfg *config.Config,
	notification *notification.Notification) (*Application, error) {

	orderDB := database.NewOrderDb(db)
	pricesDB := database.NewPricesDb(db)

	assetsPrices, err := prices.NewAssetsPrices(settings.Pairs, settings.ChangePeriods, settings.DeltaPeriods, pricesDB)
	if err != nil {
		return nil, err
	}
	account, err := account.NewAccount(exch, assetsPrices)
	if err != nil {
		return nil, err
	}
	paperWallet := paperwallet.NewPaperWallet(assetsPrices)
	orderController, err := order.NewOrderService(orderDB, paperWallet, socketsMessage, assetsPrices)
	if err != nil {
		return nil, err
	}

	controllerStrategy, err := strategy.NewControllerStrategy(assetsPrices, settings.ChangePeriods, settings.Pairs, notification, orderController, cfg)
	if err != nil {
		return nil, err
	}

	app := &Application{
		Settings: settings,
		Config:   cfg,
		exchange: exch,
		dataFeed: dataFeed,

		AssetsPrices:       assetsPrices,
		Account:            account,
		OrderController:    orderController,
		PaperWallet:        paperWallet,
		ControllerStrategy: controllerStrategy,
	}

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
			time.Sleep(1 * time.Second)
		}
		if shouldBreak {
			break
		}
	}

	observers := []func(market exModel.MarketsStat){
		func(market exModel.MarketsStat) {
			app.AssetsPrices.OnMarket(market)
			app.OrderController.OnMarket(market)
			app.ControllerStrategy.OnMarket(market)
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
