/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cobra

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/sambly/exchangeService/pkg/logadapter"
	"github.com/sambly/exchangebot"
	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/database"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/strategy"
	"github.com/sambly/exchangebot/internal/telegram"
	"github.com/sambly/exchangebot/internal/web"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

var cfg *config.Config
var filename = "config.yaml"

var RootCmd = &cobra.Command{
	Use:    "exchangebot",
	Short:  "exchangebot",
	PreRun: preRun,
	RunE:   run,
}

func init() {

	RootCmd.PersistentFlags().String("exchange-type", "exchange", "select exchange type exchange or grpc")

	// Web
	RootCmd.PersistentFlags().Bool("production", false, "запуск сервера в режиме production, запуск возможен напрямую или через proxy")
	RootCmd.PersistentFlags().Bool("proxy-server", false, "запуск через proxy, необходимо также чтобы production = true")
	RootCmd.PersistentFlags().String("proxy-port", "444", "proxy port")
	RootCmd.PersistentFlags().String("host-web", "", "host-web")
	RootCmd.PersistentFlags().Bool("content-embed", false, "в режиме production=true выставить тоже в true, все web файлы объединяет в один бинарник")
	RootCmd.PersistentFlags().String("username-auth", "", "username-auth")
	RootCmd.PersistentFlags().String("password-auth", "", "password-auth")

	// Exchange
	RootCmd.PersistentFlags().String("api-key-binance", "", "api-key-binance")
	RootCmd.PersistentFlags().String("api-secret-binance", "", "api-secret-binance")

	// TLG
	RootCmd.PersistentFlags().String("telegram-token", "", "telegram-token")
	RootCmd.PersistentFlags().String("telegram-user", "", "telegram-user")

	// DB
	RootCmd.PersistentFlags().String("db-name", "datafeeder", "db-name")
	RootCmd.PersistentFlags().String("db-password", "q1w2e3", "db-password")
	RootCmd.PersistentFlags().String("db-port", "3306", "db-port")
	RootCmd.PersistentFlags().String("db-user", "root", "db-user")
	RootCmd.PersistentFlags().String("db-host-local", "127.0.0.1", "db-host-local")

	// Log
	RootCmd.PersistentFlags().Bool("debug-log", false, "debug log mode")
	RootCmd.PersistentFlags().Bool("production-log", false, "production-log log mode")

	// Grpc
	RootCmd.PersistentFlags().String("grpc-port", "50051", "grpc-port")
	RootCmd.PersistentFlags().String("grpc-host-local", "0.0.0.0", "grpc-host-local")

	viper.SetConfigFile(filename)
	if err := viper.BindPFlags(RootCmd.PersistentFlags()); err != nil {
		log.Fatalf("failed to bind persistent flags. please check the flag settings. %v", err)
	}
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatalf("cannot execute command")
	}
}

func preRun(cmd *cobra.Command, args []string) {

	if os.Getenv("ENVIRONMENT") != "docker" {
		if err := godotenv.Load(".env"); err != nil {
			log.Printf("Error loading .env file, %s", err)
		}
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

}

func run(cmd *cobra.Command, args []string) error {

	var err error
	cfg, err = config.NewConfigV3()
	if err != nil {
		log.Fatal(err)
	}

	mainLogger := logger.AddFields(map[string]interface{}{
		"package": "main",
	})

	if err := logger.InitLogger(cfg.DebugLog, cfg.ProductionLog); err != nil {
		log.Fatalf("failed to InitLogger: %v", err)
	}

	if err := reloadConfig(); err != nil {
		mainLogger.Fatal(err)
	}

	mainLogger.Info("запуск приложения exchangebot-app")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(cfg.APIKey, cfg.SecretKey))
	if err != nil {
		mainLogger.Fatalf("failed to create exchange instance: %v", err)
	}

	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		mainLogger.Fatal(err)
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
		ServerName:     cfg.ServerName,
		Pairs:          pairs,
		Timeframe:      "1m",
		ChangePeriods:  periods,
		DeltaPeriods:   periods,
		WeightProcents: map[string]float64{"ch3m": 0.7, "ch15m": 1.2, "ch1h": 2, "ch4h": 4},
	}
	db, err := database.DbConnection(cfg.NameDb, cfg.HostDb, cfg.PortDb, cfg.UserDb, cfg.PasswordDb)
	if err != nil {
		mainLogger.Fatal(err)
	}
	defer db.Close()

	err = database.CreateOrdersTable(db)
	if err != nil {
		mainLogger.Fatal(err)
	}

	err = database.CreateOrdersInfoTable(db)
	if err != nil {
		mainLogger.Fatal(err)
	}

	notify := &notification.Notification{Message: make(chan string)}
	socketsMessage := &notification.SocketsMessage{Message: make(chan []byte)}

	var dataFeed exchange.RouterDataFeed

	if cfg.ExchangeType == "exchange" {
		dataFeed = exchange.NewDataFeedWithExchange(
			binance,
			logadapter.NewLogrusAdapter(logger.AddFieldsEmpty()),
		)
	} else if cfg.ExchangeType == "grpc" {

		c, conn, err := exchange.NewClientGrpc(fmt.Sprintf("%s:%s", cfg.GrpcHost, cfg.GrpcPort))
		if err != nil {
			mainLogger.Fatalf("did not connect to grpc: %v", err)
		}

		defer conn.Close()

		dataFeed = exchange.NewDataFeed(
			c,
			logadapter.NewLogrusAdapter(logger.AddFieldsEmpty()),
		)
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

	appTelegram, err := telegram.NewTelegram(app, cfg.TlgToken, cfg.TlgUser, notify)
	if err != nil {
		mainLogger.Fatal(err)
	}
	web := web.NewWeb(app, socketsMessage, cfg, exchangebot.Content)

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
	return nil
}
