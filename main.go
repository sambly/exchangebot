package main

import (
	"context"
	"log"
	"main/application"
	"main/config"
	"main/database"
	"main/exchange"
	mylog "main/log"
	"main/model"
	"main/notification"
	"main/telegram"
	"main/web"
	"time"
)

func main() {

	ctx := context.Background()
	mylog.InitLogger()

	config, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(config.ApiKey, config.SecretKey))
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		log.Fatal(err)
	}

	// TODO добавить сюда еще периоды PeriodsDelta:   []string{"1m", "5m", "30m", "1h", "4h", "1d"},

	changePeriods := map[string]time.Duration{
		"ch1m":  time.Second * 60,
		"ch3m":  time.Minute * 3,
		"ch15m": time.Minute * 15,
		"ch1h":  time.Hour,
		"ch4h":  time.Hour * 4,
		"ch12h": time.Hour * 12,
	}

	settings := model.Settings{
		Pairs:          pairs,
		Timeframe:      "1m",
		ChangePeriods:  changePeriods,
		WeightProcents: map[string]float64{"ch3m": 0.7, "ch15m": 1.2, "ch1h": 2, "ch4h": 4},
		LengthOfTime:   1080,
	}

	db, err := database.DbConnection(config.NameDb, config.HostNameDb, config.UserNameDb, config.PasswordDb)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = database.CreateOrdersTable(db)
	if err != nil {
		log.Fatal(err)
	}

	err = database.CreateOrdersInfoTable(db)
	if err != nil {
		log.Fatal(err)
	}

	notify := &notification.Notification{Message: make(chan string)}
	socketsMessage := &notification.SocketsMessage{Message: make(chan []byte)}

	app, err := application.NewApp(
		ctx,
		binance,
		settings,
		db,
		notify,
		socketsMessage,
	)
	if err != nil {
		log.Fatal(err)
	}

	appTelegram, err := telegram.NewTelegram(app, config.TlgToken, config.TlgUser, notify)
	if err != nil {
		log.Fatal(err)
	}
	appTelegram.Start()

	web := web.NewWeb(app, socketsMessage, config)
	go web.Run()

	app.Run()
}
