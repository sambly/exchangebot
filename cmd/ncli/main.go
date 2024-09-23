package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/sambly/exchangeService/pkg/logadapter"
	"github.com/sambly/exchangebot"
	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/database"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/service"
	"github.com/sambly/exchangebot/internal/strategy"
	"github.com/sambly/exchangebot/internal/telegram"
	"github.com/sambly/exchangebot/internal/web"
	"golang.org/x/sync/errgroup"
)

func main() {

	config, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	if err := logger.InitLogger(config.DebugLog, config.ProductionLog); err != nil {
		log.Fatalf("failed to InitLogger: %v", err)
	}

	mainLogger := logger.AddFields(map[string]interface{}{
		"package": "main",
	})

	mainLogger.Info("запуск приложения exchangebot-app")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(config.APIKey, config.SecretKey))
	if err != nil {
		mainLogger.Fatalf("failed to create exchange instance: %v", err)
	}

	pairs, err := service.GetPairs(config.PairsFromFile, binance)
	if err != nil {
		mainLogger.Fatalf("failed get pairs: %v", err)
	}

	mainLogger.Infof("колличество пар: %v", len(pairs))

	periods := map[string]time.Duration{
		"1m":  time.Second * 60,
		"3m":  time.Minute * 3,
		"15m": time.Minute * 15,
		"1h":  time.Hour,
		"4h":  time.Hour * 4,
		"1d":  time.Hour * 12,
	}

	periodsStrategy := map[string]time.Duration{
		"1h": time.Hour,
		"4h": time.Hour * 4,
		"1d": time.Hour * 12,
	}

	settings := model.Settings{
		ServerName:     config.ServerName,
		Pairs:          pairs,
		Timeframe:      "1m",
		ChangePeriods:  periods,
		DeltaPeriods:   periods,
		WeightProcents: map[string]float64{"ch3m": 0.7, "ch15m": 1.2, "ch1h": 2, "ch4h": 4},
	}

	db, err := database.DbInit(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
	if err != nil {
		mainLogger.Fatal(err)
	}
	defer db.Close()

	notify := &notification.Notification{Message: make(chan string)}
	socketsMessage := &notification.SocketsMessage{Message: make(chan []byte)}

	dataFeed, conn, err := exchange.InitDataFeed(ctx,
		config.ExchangeType,
		config.GrpcHost,
		config.GrpcPort,
		binance,
		logadapter.NewLogrusAdapter(logger.AddFieldsEmpty()))
	if err != nil {
		mainLogger.Fatalf("failed to initialize data feed: %v", err)
	}
	if conn != nil {
		defer conn.Close()
	}

	strategy, err := strategy.NewControllerStrategy(
		strategy.WithLocalExtremes(strategy.NewLocalExtremes(pairs, periodsStrategy)),
	)

	if err != nil {
		mainLogger.Fatal(err)
	}

	app, err := application.NewApp(
		ctx,
		binance,
		dataFeed,
		settings,
		db,
		notify,
		socketsMessage,
		strategy,
	)
	if err != nil {
		mainLogger.Fatal(err)
	}

	appTelegram, err := telegram.NewTelegram(app, config.TlgToken, config.TlgUser, notify)
	if err != nil {
		mainLogger.Fatal(err)
	}
	web := web.NewWeb(app, socketsMessage, config, exchangebot.Content)

	g, gCtx := errgroup.WithContext(ctx)

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
		mainLogger.Fatalf("приложение exchangebot-app завершено с ошибкой: %v", err)
	} else {
		mainLogger.Info("приложение exchangebot-app завершено")
	}
}
