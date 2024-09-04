package application

import (
	"context"
	"database/sql"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/account"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy"

	"golang.org/x/sync/errgroup"
)

type Application struct {
	Settings model.Settings
	database *sql.DB

	exchange exchange.Exchange
	dataFeed exchange.RouterDataFeed

	Account         *account.Account
	AssetsPrices    *prices.AsetsPrices
	OrderController *order.Controller
	PaperWallet     *exchange.PaperWallet
	Strategy        *strategy.ControllerStrategy

	BaseAmountAsset float64
}

var appLogger = logger.AddFieldsEmpty()

func NewApp(ctx context.Context, exch exchange.Exchange, dataFeed exchange.RouterDataFeed, settings model.Settings, db *sql.DB, notification *notification.Notification, socketsMessage *notification.SocketsMessage, strategy *strategy.ControllerStrategy) (*Application, error) {

	assetsPrices := prices.NewAssetsPrices(settings.Pairs, settings.ChangePeriods, settings.DeltaPeriods, settings.WeightProcents, db, notification)
	account, err := account.NewAccount(exch, assetsPrices, notification)
	if err != nil {
		return nil, err
	}
	paperWallet := exchange.NewPaperWallet(ctx)
	orderController, err := order.NewController(ctx, paperWallet, db, socketsMessage, assetsPrices)
	if err != nil {
		return nil, err
	}

	app := &Application{
		Settings: settings,
		exchange: exch,
		dataFeed: dataFeed,
		database: db,

		AssetsPrices:    assetsPrices,
		Account:         account,
		OrderController: orderController,
		PaperWallet:     paperWallet,
		Strategy:        strategy,
		BaseAmountAsset: 1,
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

	for _, pair := range app.Settings.Pairs {

		app.dataFeed.SubscribeMarketsStat(ctx, pair, "exchangebot")
		app.dataFeed.SubscribeObserverMarkets(ctx, "exchangebot", pair, func(market exModel.MarketsStat) {
			app.AssetsPrices.OnMarket(market)
		})

		app.dataFeed.SubscribeObserverMarkets(ctx, "exchangebot", pair, func(market exModel.MarketsStat) {
			app.OrderController.OnMarket(market)
		})

		app.dataFeed.SubscribeObserverMarkets(ctx, "exchangebot", pair, func(market exModel.MarketsStat) {
			app.Strategy.OnMarket(market)
		})

	}

	duration := time.Since(timeStart)

	appLogger.Infof("Время выполнения предварительной загрузки данных: %v ", duration)
	appLogger.Infof("Время старта: %v ", timeStart)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.dataFeed.StartMarketsStatFeeder(ctx)
	})

	//Для предварительного заполения цен всех пар, может сделать меньше время, просто добавляет погрешность для 10m
	tickerIntervalInit := time.Second * 10 // Здесь выставить 40
	tickerInit := time.NewTicker(tickerIntervalInit)

	tickerInterval3m := time.Second * 60 * 3
	ticker3m := time.NewTicker(tickerInterval3m)

	tickerInterval15m := time.Second * 60 * 15
	ticker15m := time.NewTicker(tickerInterval15m)

	tickerInterval1h := time.Second * 60 * 60
	ticker1h := time.NewTicker(tickerInterval1h)

	tickerInterval4h := time.Second * 60 * 60 * 4
	ticker4h := time.NewTicker(tickerInterval4h)

	for {
		select {
		case <-ctx.Done():
			// Останавливаем все тикеры при завершении контекста
			tickerInit.Stop()
			ticker3m.Stop()
			ticker15m.Stop()
			ticker1h.Stop()
			ticker4h.Stop()
			return g.Wait()

		case <-tickerInit.C:
			// app.AssetsPrices.UpdateChanges("")
			tickerInit.Stop()

		case <-ticker3m.C:
			// app.AssetsPrices.UpdateChanges("ch3m")

		case <-ticker15m.C:
			// app.AssetsPrices.UpdateChanges("ch15m")

		case <-ticker1h.C:
			// app.AssetsPrices.UpdateChanges("ch1h")
			// err := app.Account.UpdateAssets()
			// if err != nil {
			// 	fmt.Printf("%v", err)
			// 	return err
			// }

		case <-ticker4h.C:
			// app.AssetsPrices.UpdateChanges("ch4h")
			// err := app.Account.UpdateAssets()
			// if err != nil {
			// 	fmt.Printf("%v", err)
			// 	return err
			// }
		}
	}
}
