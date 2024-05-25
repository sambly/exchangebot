package application

import (
	"context"
	"database/sql"
	"log"
	"main/internal/account"
	"main/internal/exchange"
	"main/internal/model"
	"main/internal/notification"
	"main/internal/order"
	"main/internal/prices"
	"main/internal/service"
	"time"
)

type Application struct {
	settings model.Settings
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
	database *sql.DB

	Account         *account.Account
	AssetsPrices    *prices.AsetsPrices
	OrderController *order.Controller
	PaperWallet     *exchange.PaperWallet

	BaseAmountAsset float64
}

func NewApp(ctx context.Context, exch service.Exchange, settings model.Settings, db *sql.DB, notification *notification.Notification, socketsMessage *notification.SocketsMessage) (*Application, error) {

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
		settings: settings,
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, settings.Pairs),
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

func (app *Application) Run() error {

	timeStart := time.Now()
	log.Println("Ожидание предварительной загрузки данных")
	// Ожидание, пока текущее время не попадет в интервал от 10 до 50 секунд
	for {
		timeStart := time.Now()
		seconds := timeStart.Second()
		if seconds >= 10 && seconds <= 50 {
			break
		}
		time.Sleep(1 * time.Second) // Ждем одну секунду перед повторной проверкой
	}

	timeRounding := timeStart.Truncate(60 * time.Second)

	app.AssetsPrices.UpdateTime = timeRounding
	app.AssetsPrices.InitChangePrices()
	app.AssetsPrices.InitDelta()

	for _, pair := range app.settings.Pairs {
		app.dataFeed.Subscribe(pair, app.AssetsPrices.OnMarket)
		app.dataFeed.Subscribe(pair, app.OrderController.OnMarket)
	}

	duration := time.Since(timeStart)

	log.Println("Время выполнения предварительной загрузки данных: ", duration)
	log.Println("Время старта: ", timeStart)

	go app.dataFeed.Start(true)

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
