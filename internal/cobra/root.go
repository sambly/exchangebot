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
	"github.com/sambly/exchangeService/pkg/telemetry"
	"github.com/sambly/exchangebot"
	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/database"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/service"
	"github.com/sambly/exchangebot/internal/telegram"
	"github.com/sambly/exchangebot/internal/web"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var cfg *config.Config
var filenameConfig = "config.yaml"
var filenameConfigReload = "configs/config_reload.yaml"

var RootCmd = &cobra.Command{
	Use:    "exchangebot",
	Short:  "exchangebot",
	PreRun: preRun,
	RunE:   run,
}

func init() {

	RootCmd.PersistentFlags().String("app-exchange-type", "exchange", "select exchange type exchange or grpc")
	RootCmd.PersistentFlags().Bool("app-pairs-from-file", false, "брать пары из файла")

	RootCmd.PersistentFlags().Bool("web-production", true, "запуск сервера в режиме production, запуск возможен напрямую или через proxy")
	RootCmd.PersistentFlags().Bool("web-proxy-server", false, "запуск через proxy, необходимо также чтобы production = true")
	RootCmd.PersistentFlags().String("web-proxy-port", "444", "proxy port")
	RootCmd.PersistentFlags().String("web-host", "", "host-web")
	RootCmd.PersistentFlags().Bool("web-content-embed", false, "в режиме production=true выставить тоже в true, все web файлы объединяет в один бинарник")
	RootCmd.PersistentFlags().String("web-username-auth", "", "username-auth")
	RootCmd.PersistentFlags().String("web-password-auth", "", "password-auth")

	RootCmd.PersistentFlags().Bool("log-debug", false, "debug log mode")
	RootCmd.PersistentFlags().Bool("log-production", false, "production-log log mode")

	RootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		parts := strings.SplitN(flag.Name, "-", 2)
		key := strings.Join(parts, ".")
		viper.BindPFlag(key, flag)
	})

}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatalf("cannot execute command")
	}
}

func preRun(cmd *cobra.Command, args []string) {

	// Запись в viper конфигурационных значений из env
	// TODO здесь надо использовать какой то другой тэг, который бы обозначал что мы удаленно запускаем
	if os.Getenv("ENVIRONMENT") != "docker" {
		// Загружаем файл .env, если запуск происходит локально
		if err := godotenv.Load(".env"); err != nil {
			log.Fatalf("Error loading .env file, %s", err)
		}
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Запись в viper конфигурационных значений из файла
	viper.SetConfigFile(filenameConfig)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Unable to read config file: %v", err)
	}
}

func run(cmd *cobra.Command, args []string) error {

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	cfg, err = config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	if err := logger.InitLogger(cfg.Log); err != nil {
		log.Fatalf("failed to InitLogger: %v", err)
	}

	mainLogger := logger.AddFields(map[string]interface{}{
		"package": "main",
	})

	if err := reloadConfig(); err != nil {
		mainLogger.Fatal(err)
	}

	mainLogger.Info("запуск приложения exchangebot-app")

	err = telemetry.SetupOpenTelemetry(ctx, cfg.OtelExporterEndpoint, cfg.OtelServiceName)
	if err != nil {
		mainLogger.Fatalf("failed to initialize OpenTelemetry: %v", err)
	}

	binance, err := exchange.NewBinance(ctx,
		exchange.WithBinanceCredentials(cfg.APIKey, cfg.SecretKey),
		exchange.WithBinanceLogger(logadapter.NewLogrusAdapter(logger.AddFieldsEmpty())),
		exchange.WithBinanceTracer(telemetry.Tracer),
	)
	if err != nil {
		mainLogger.Fatalf("failed to create exchange instance: %v", err)
	}

	pairs, err := service.GetPairs(ctx, cfg.PairsFromFile, binance)
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

	settings := model.Settings{
		ServerName:    cfg.ServerName,
		Pairs:         pairs,
		Timeframe:     "1m",
		ChangePeriods: periods,
		DeltaPeriods:  periods,
	}

	db, err := database.DbInit(cfg.Database)
	if err != nil {
		mainLogger.Fatal(err)
	}

	notificationService := notification.NewNotificationService(cfg.NotificationEnable)
	socketsMessage := &notification.SocketsMessage{Message: make(chan []byte)}

	var exflow exchange.Exflow
	var conn *grpc.ClientConn
	switch cfg.ExchangeType {
	case "exchange":
		exflow = binance
	case "grpc":
		exflow, conn, err = exchange.NewClientGrpc(
			fmt.Sprintf("%s:%s", cfg.GRPC.Host, cfg.GRPC.Port),
			exchange.WithClientLogger(logadapter.NewLogrusAdapter(logger.AddFieldsEmpty())),
			exchange.WithClientTracer(telemetry.Tracer),
		)
		if err != nil {
			mainLogger.Fatalf("did not connect to grpc: %v", err)
		}
		defer conn.Close()
	}

	dataFeed := exchange.NewDataFeed(
		exflow,
		exchange.WithDataFeedLogger(logadapter.NewLogrusAdapter(logger.AddFieldsEmpty())),
		exchange.WithDataFeedTracer(telemetry.Tracer),
	)
	if err != nil {
		mainLogger.Fatalf("failed to initialize data feed: %v", err)
	}

	app, err := application.NewApp(
		binance,
		dataFeed,
		settings,
		db,
		socketsMessage,
		cfg,
		notificationService,
	)
	if err != nil {
		mainLogger.Fatal(err)
	}

	telegram, err := telegram.NewTelegram(app, cfg.Telegram)
	if err != nil {
		mainLogger.Fatal(err)
	}
	notificationService.AddService(telegram)

	web := web.NewWeb(app, socketsMessage, cfg.Web, exchangebot.Content)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return telegram.Start(gCtx)
	})

	g.Go(func() error {
		return notificationService.Start(gCtx)
	})

	g.Go(func() error {
		return web.Run(gCtx)
	})

	g.Go(func() error {
		return app.Run(gCtx)
	})

	if err := g.Wait(); err != nil && gCtx.Err() != context.Canceled {
		mainLogger.Fatalf("ошибка при завершении приложения exchangebot-app: %v", err)
	}

	if err := telemetry.OpenTelemetryWaitShutdown(); err != nil {
		mainLogger.Fatalf("ошибка при завершении open-telemetry: %v", err)
	}

	mainLogger.Info("приложение exchangebot-app завершено")

	return nil
}
