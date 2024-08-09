package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sambly/exchangeBot"
	"github.com/sambly/exchangeBot/internal/application"
	"github.com/sambly/exchangeBot/internal/config"
	"github.com/sambly/exchangeBot/internal/database"
	"github.com/sambly/exchangeBot/internal/exchange"
	"github.com/sambly/exchangeBot/internal/logging"
	"github.com/sambly/exchangeBot/internal/model"
	"github.com/sambly/exchangeBot/internal/notification"
	"github.com/sambly/exchangeBot/internal/telegram"
	"github.com/sambly/exchangeBot/internal/web"
	"golang.org/x/sync/errgroup"
)

func main() {

	logging.InitLogger()
	logging.MyLogger.InfoLog.Println("Запуск приложения")

	config, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(config.ApiKey, config.SecretKey))
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		log.Fatal(err)
	}

	logging.MyLogger.InfoLog.Println("Колличество пар : ", len(pairs))

	periods := map[string]time.Duration{
		"1m":  time.Second * 60,
		"3m":  time.Minute * 3,
		"15m": time.Minute * 15,
		"1h":  time.Hour,
		"4h":  time.Hour * 4,
		"1d":  time.Hour * 12,
	}

	settings := model.Settings{
		ServerName:     config.ServerName,
		Pairs:          pairs,
		Timeframe:      "1m",
		ChangePeriods:  periods,
		DeltaPeriods:   periods,
		WeightProcents: map[string]float64{"ch3m": 0.7, "ch15m": 1.2, "ch1h": 2, "ch4h": 4},
	}
	db, err := database.DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
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

	g, gCtx := errgroup.WithContext(ctx)

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
	web := web.NewWeb(app, socketsMessage, config, exchangeBot.Content)

	g.Go(func() error {
		return appTelegram.Start(gCtx)
	})

	g.Go(func() error {
		return web.Run(gCtx)
	})

	g.Go(func() error {
		return app.Run(gCtx)
	})
	if err := g.Wait(); err != nil && gCtx.Err() != context.Canceled {
		log.Fatalf("Приложение завершено с ошибкой: %v", err)
	} else {
		logging.MyLogger.InfoLog.Println("Приложение завершено")
	}
}
