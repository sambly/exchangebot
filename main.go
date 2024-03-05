package main

import (
	"context"
	"log"
	"main/application"
	"main/database"
	"main/exchange"
	mylog "main/log"
	"main/model"
	"main/notification"
	"main/telegram"
	"main/web"
	"os"

	"github.com/joho/godotenv"
)

func main() {

	production := false

	ctx := context.Background()

	// loads values from .env into the system
	if err := godotenv.Load(); err != nil {
		log.Fatal("No .env file found")
	}
	// Exchange
	apiKey, exists := os.LookupEnv("API_KEY")
	if !exists {
		log.Fatal("No .env str API_KEY found")
	}
	secretKey, exists := os.LookupEnv("API_SECRET")
	if !exists {
		log.Fatal("No .env str API_SECRET found")
	}
	// TLG
	tlgToken, exists := os.LookupEnv("TELEGRAM_TOKEN")
	if !exists {
		log.Fatal("No .env str TELEGRAM_TOKEN found")
	}
	tlgUser, exists := os.LookupEnv("TELEGRAM_USER")
	if !exists {
		log.Fatal("No .env str TELEGRAM_USER found")
	}
	// Authentication
	usernameAuth, exists := os.LookupEnv("usernameAuth")
	if !exists {
		log.Fatal("No .env str usernameAuth found")
	}
	passwordAuth, exists := os.LookupEnv("passwordAuth")
	if !exists {
		log.Fatal("No .env str passwordAuth found")
	}
	// DB
	userNameDb, exists := os.LookupEnv("userNameDb")
	if !exists {
		log.Fatal("No .env str userNameDb found")
	}
	passwordDb, exists := os.LookupEnv("passwordDb")
	if !exists {
		log.Fatal("No .env str passwordDb found")
	}
	nameDb, exists := os.LookupEnv("nameDb")
	if !exists {
		log.Fatal("No .env str nameDb found")
	}
	hostNameDb, exists := os.LookupEnv("hostNameDb")
	if !exists {
		log.Fatal("No .env str hostNameDb found")
	}
	// httpPort
	httpPortProduction, exists := os.LookupEnv("httpPortProduction")
	if !exists {
		log.Fatal("No .env str httpPortProduction found")
	}

	mylog.InitLogger()

	binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(apiKey, secretKey))
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		log.Fatal(err)
	}

	//pairs := []string{"MBOXUSDT", "AUDIOUSDT", "PROMUSDT", "AIUSDT", "LDOUSDT", "ETCUSDT", "QNTUSDT"}

	// TODO добавить сюда еще периоды PeriodsDelta:   []string{"1m", "5m", "30m", "1h", "4h", "1d"},

	settings := model.Settings{
		Pairs:          pairs,
		Timeframe:      "1m",
		ChangePeriods:  []string{"ch3m", "ch15m", "ch1h", "ch4h"},
		WeightProcents: map[string]float64{"ch3m": 0.7, "ch15m": 1.2, "ch1h": 2, "ch4h": 4},
		LengthOfTime:   1080,
	}

	db, err := database.DbConnection(nameDb, hostNameDb, userNameDb, passwordDb)
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

	appTelegram, err := telegram.NewTelegram(app, tlgToken, tlgUser, notify)
	if err != nil {
		log.Fatal(err)
	}
	appTelegram.Start()

	web := web.NewWeb(app, socketsMessage, production, httpPortProduction, usernameAuth, passwordAuth)
	web.Run()

	app.Run()
}
