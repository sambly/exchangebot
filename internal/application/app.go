package application

import (
	"context"
	"database/sql"
	"time"

	"github.com/sambly/exchangeBot/internal/account"
	"github.com/sambly/exchangeBot/internal/model"
	"github.com/sambly/exchangeBot/internal/notification"
	"github.com/sambly/exchangeBot/internal/order"
	"github.com/sambly/exchangeBot/internal/prices"
	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"

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

	BaseAmountAsset float64
}

func NewApp(ctx context.Context, exch exchange.Exchange, dataFeed exchange.RouterDataFeed, settings model.Settings, db *sql.DB, notification *notification.Notification, socketsMessage *notification.SocketsMessage) (*Application, error) {

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
		BaseAmountAsset: 1,
	}

	app.PaperWallet.MarketsStat = assetsPrices.MarketsStat

	return app, nil
}

func (app *Application) Run(ctx context.Context) error {

	// TODO
	//logging.MyLogger.InfoLog.Println("Ожидание предварительной загрузки данных")

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

		app.dataFeed.SubscribeTrade(ctx, pair, "exchangeBot")
		err := app.dataFeed.SubscribeObserverMarkets(ctx, "exchangeBot", pair, func(market exModel.MarketsStat) {
			app.AssetsPrices.OnMarket(market)
		})
		if err != nil {
			// TODO
			// appLogger.Errorf("error SubscribeObserverMarket: %v", err)
		}
		err = app.dataFeed.SubscribeObserverMarkets(ctx, "exchangeBot", pair, func(market exModel.MarketsStat) {
			app.OrderController.OnMarket(market)
		})
		if err != nil {
			// TODO
			// appLogger.Errorf("error SubscribeObserverMarket: %v", err)
		}

	}

	duration := time.Since(timeStart)

	_ = duration

	// TODO
	//logging.MyLogger.InfoLog.Println("Время выполнения предварительной загрузки данных: ", duration)
	//logging.MyLogger.InfoLog.Println("Время старта: ", timeStart)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.dataFeed.StartMarketsStatFeeder(ctx)
	})

	//Для предварительного заполения цен всех пар, может сделать меньше время, просто добавляет погрешность для 10m
	var tickerInterval_Init time.Duration = time.Second * 10 // Здесь выставить 40
	ticker_Init := time.NewTicker(tickerInterval_Init)

	var tickerInterval_3m time.Duration = time.Second * 60 * 3
	ticker_3m := time.NewTicker(tickerInterval_3m)

	var tickerInterval_15m time.Duration = time.Second * 60 * 15
	ticker_15m := time.NewTicker(tickerInterval_15m)

	var tickerInterval_1h time.Duration = time.Second * 60 * 60
	ticker_1h := time.NewTicker(tickerInterval_1h)

	var tickerInterval_4h time.Duration = time.Second * 60 * 60 * 4
	ticker_4h := time.NewTicker(tickerInterval_4h)

	for {
		select {
		case <-ctx.Done():
			// Останавливаем все тикеры при завершении контекста
			ticker_Init.Stop()
			ticker_3m.Stop()
			ticker_15m.Stop()
			ticker_1h.Stop()
			ticker_4h.Stop()
			return g.Wait()

		case <-ticker_Init.C:
			// app.AssetsPrices.UpdateChanges("")
			ticker_Init.Stop()

		case <-ticker_3m.C:
			// app.AssetsPrices.UpdateChanges("ch3m")

		case <-ticker_15m.C:
			// app.AssetsPrices.UpdateChanges("ch15m")

		case <-ticker_1h.C:
			// app.AssetsPrices.UpdateChanges("ch1h")
			// err := app.Account.UpdateAssets()
			// if err != nil {
			// 	fmt.Printf("%v", err)
			// 	return err
			// }

		case <-ticker_4h.C:
			// app.AssetsPrices.UpdateChanges("ch4h")
			// err := app.Account.UpdateAssets()
			// if err != nil {
			// 	fmt.Printf("%v", err)
			// 	return err
			// }
		}
	}
}
