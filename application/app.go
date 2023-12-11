package application

import (
	"database/sql"
	"fmt"
	"main/account"
	"main/exchange"
	"main/model"
	"main/notification"
	"main/prices"
	"main/service"
	"time"
)

type Application struct {
	settings model.Settings
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
	database *sql.DB

	AssetsPrices    *prices.AsetsPrices
	Account         *account.Account
	BaseAmountAsset float64
}

func NewApp(exch service.Exchange, settings model.Settings, notification *notification.Notification) (*Application, error) {

	assetsPrices := prices.NewAssetsPrices(settings.Pairs, settings.ChangePeriods, settings.WeightProcents, notification)

	account, err := account.NewAccount(exch, assetsPrices, notification)
	if err != nil {
		return nil, err
	}

	app := &Application{
		settings: settings,
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, settings.Pairs),
		//database: db,

		Account:         account,
		AssetsPrices:    assetsPrices,
		BaseAmountAsset: 1,
	}

	return app, nil
}

func (app *Application) Run() error {

	for _, pair := range app.settings.Pairs {
		app.dataFeed.Subscribe(pair, app.AssetsPrices.OnMarket)
	}
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
			app.AssetsPrices.UpdateChanges("")
			err := app.Account.UpdateAssets()
			if err != nil {
				fmt.Printf("%v", err)
				return err
			}
			ticker_Init.Stop()

		case <-ticker_3m.C:
			app.AssetsPrices.UpdateChanges("ch3m")
			err := app.Account.UpdateAssets()
			if err != nil {
				fmt.Printf("%v", err)
				return err
			}
		case <-ticker_15m.C:
			app.AssetsPrices.UpdateChanges("ch15m")
			err := app.Account.UpdateAssets()
			if err != nil {
				fmt.Printf("%v", err)
				return err
			}

		case <-ticker_1h.C:
			app.AssetsPrices.UpdateChanges("ch1h")
			err := app.Account.UpdateAssets()
			if err != nil {
				fmt.Printf("%v", err)
				return err
			}

		case <-ticker_4h.C:
			app.AssetsPrices.UpdateChanges("ch4h")
			err := app.Account.UpdateAssets()
			if err != nil {
				fmt.Printf("%v", err)
				return err
			}
		}
	}
}
