package application

import (
	"database/sql"
	"fmt"
	"log"
	"main/account"
	"main/exchange"
	"main/model"
	"main/prices"
	"main/service"
	"runtime/debug"
)

type Application struct {
	settings model.Settings
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
	database *sql.DB
	infoLog  *log.Logger
	errorLog *log.Logger

	AssetsPrices    *prices.AsetsPrices
	Account         *account.Account
	BaseAmountAsset float64
}

func NewApp(exch service.Exchange, settings model.Settings, infoLog, errorLog *log.Logger) (*Application, error) {

	assetsPrices, err := prices.NewAssetsPrices()
	if err != nil {
		return nil, err
	}
	account, err := account.NewAccount(exch, assetsPrices)
	if err != nil {
		return nil, err
	}

	app := &Application{
		settings: settings,
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, settings.Pairs),
		//database: db,
		infoLog:  infoLog,
		errorLog: errorLog,

		Account:         account,
		AssetsPrices:    assetsPrices,
		BaseAmountAsset: 10,
	}

	return app, nil
}

func (app *Application) Run() error {

	for _, pair := range app.settings.Pairs {
		app.dataFeed.Subscribe(pair, app.AssetsPrices.OnMarket)
	}
	app.dataFeed.Start(true)
	return nil
}

func (app *Application) logError(err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	log.Println(err.Error())
	app.errorLog.Output(2, trace)
}
