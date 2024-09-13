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

var RootCmd = &cobra.Command{
	Use:   "exchangebot",
	Short: "exchangebot",

	Run: func(cmd *cobra.Command, args []string) {

		var err error
		cfg, err = config.NewConfigV2()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(cfg.String())

		logger.InitLogger(cfg.DebugLog, cfg.ProductionLog)

		mainLogger := logger.AddFields(map[string]interface{}{
			"package": "main",
		})

		mainLogger.Info("запуск приложения exchangebot-app")

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(cfg.APIKey, cfg.SecretKey))
		if err != nil {
			mainLogger.Fatalf("failed to create exchange instance: %v", err)
		}

		// pairs, err := binance.GetPairsToUSDT()
		// if err != nil {
		// 	mainLogger.Fatal(err)
		// }

		pairs := []string{"BTCUSDT"}

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

		if cfg.BuildTarget == "exchange" {
			dataFeed = exchange.NewDataFeedWithExchange(
				binance,
				logadapter.NewLogrusAdapter(logger.AddFieldsEmpty()),
			)
		} else if cfg.BuildTarget == "grpc" {

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

	},
}

func init() {

	RootCmd.PersistentFlags().Bool("debug-log", false, "debug log mode")
	RootCmd.PersistentFlags().String("build-target", "exchange", "select build target exchange or grpc")

	if err := viper.BindPFlags(RootCmd.PersistentFlags()); err != nil {
		log.Fatalf("failed to bind persistent flags. please check the flag settings. %v", err)
		return
	}

	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Error loading .env file, %s", err)
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatalf("cannot execute command")
	}
}
