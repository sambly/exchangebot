/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cobra

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	_ "net/http/pprof"

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

	RootCmd.PersistentFlags().Bool("web-use-tls", false, "запуск через встроенный tls")
	RootCmd.PersistentFlags().String("web-listen-port", "80", "isten-port")
	RootCmd.PersistentFlags().String("web-host", "", "host-web")
	RootCmd.PersistentFlags().Bool("web-content-embed", false, "все web файлы объединяет в один бинарник")
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
	if os.Getenv("ENVIRONMENT") != "docker" {
		// Загружаем файл .env, если запуск происходит локально
		if err := godotenv.Load(".env"); err != nil {
			log.Fatalf("Error loading .env file, %s", err)
		}
	}
	viper.SetEnvPrefix("EXCHANGEBOT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
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

	if cfg.EnableOpenTelemetry {
		err = telemetry.SetupOpenTelemetry(ctx, cfg.OtelExporterEndpoint, cfg.OtelServiceName)
		if err != nil {
			mainLogger.Fatalf("failed to initialize OpenTelemetry: %v", err)
		}
	} else {
		telemetry.SetupOpenTelemetryNoop()
		mainLogger.Info("OpenTelemetry отключен")
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

	web := web.NewWeb(app, socketsMessage, &cfg.Web, exchangebot.Content)

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

	if cfg.PprofEnable {
		runtime.SetMutexProfileFraction(1)
		runtime.SetBlockProfileRate(1)

		pprofServer := &http.Server{
			Addr: "localhost:6060",
		}

		go func() {
			mainLogger.Info("pprof доступен по адресу http://localhost:6060/debug/pprof/")
			if err := pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				mainLogger.Warnf("ошибка при запуске pprof: %v", err)
			}
		}()

		g.Go(func() error {
			<-gCtx.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mainLogger.Info("Завершение pprof-сервера...")
			if err := pprofServer.Shutdown(ctx); err != nil {
				mainLogger.Warnf("ошибка при завершении pprof-сервера: %v", err)
			}
			return nil
		})
	}

	mainLogger.Info("Приложение exchangebot запущено")
	fmt.Println("Приложение exchangebot запущено")
	err = g.Wait()

	telemetryErr := telemetry.OpenTelemetryWaitShutdown()
	if telemetryErr != nil {
		mainLogger.Errorf("telemetry shutdown error: %v", telemetryErr)
	}

	if err != nil && !errors.Is(err, context.Canceled) {
		mainLogger.Errorf("exchangebot error: %v", err)
	}

	mainLogger.Info("Приложение exchangebot завершено")
	fmt.Println("Приложение exchangebot завершено")
	return nil
}
