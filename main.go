package main

import (
	"context"
	"log"
	"os"

	"main/application"
	"main/exchange"
	"main/model"
	"main/telegram"

	"github.com/joho/godotenv"
)

func main() {

	ctx := context.Background()

	// loads values from .env into the system
	if err := godotenv.Load(); err != nil {
		log.Fatal("No .env file found")
	}
	apiKey, exists := os.LookupEnv("API_KEY")
	if !exists {
		log.Fatal("No .env str API_KEY found")
	}

	secretKey, exists := os.LookupEnv("API_SECRET")
	if !exists {
		log.Fatal("No .env str API_SECRET found")
	}

	tlgToken, exists := os.LookupEnv("TELEGRAM_TOKEN")
	if !exists {
		log.Fatal("No .env str TELEGRAM_TOKEN found")
	}

	tlgUser, exists := os.LookupEnv("TELEGRAM_USER")
	if !exists {
		log.Fatal("No .env str TELEGRAM_USER found")
	}

	binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(apiKey, secretKey))
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		log.Fatal(err)
	}

	//pairs := []string{"BTCUSDT", "ETHUSDT"}

	settings := model.Settings{
		Pairs:     pairs,
		Timeframe: "1m",
	}

	infoLogFile, err := os.OpenFile("log/info.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer infoLogFile.Close()

	errorLogFile, err := os.OpenFile("log/error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer errorLogFile.Close()

	// db, err := database.DbConnection()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer db.Close()

	app, err := application.NewApp(
		binance,
		settings,
		log.New(infoLogFile, "INFO\t", log.Ldate|log.Ltime),
		log.New(errorLogFile, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile),
	)
	if err != nil {
		log.Fatal(err)
	}

	appTelegram, err := telegram.NewTelegram(app, tlgToken, tlgUser)
	if err != nil {
		log.Fatal(err)
		return
	}
	appTelegram.Start()
	app.Run()
}
