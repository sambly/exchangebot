package main

import (
	"context"
	"fmt"
	"log"
	"main/application"
	"main/config"
	"main/database"
	"main/exchange"
	mylog "main/logging"
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

	fmt.Println("Колличество пар:", len(pairs))

	periods := map[string]time.Duration{
		"1m":  time.Second * 60,
		"3m":  time.Minute * 3,
		"15m": time.Minute * 15,
		"1h":  time.Hour,
		"4h":  time.Hour * 4,
		"1d":  time.Hour * 12,
	}

	settings := model.Settings{
		Pairs:          pairs,
		Timeframe:      "1m",
		ChangePeriods:  periods,
		DeltaPeriods:   periods,
		WeightProcents: map[string]float64{"ch3m": 0.7, "ch15m": 1.2, "ch1h": 2, "ch4h": 4},
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
