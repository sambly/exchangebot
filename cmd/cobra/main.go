/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
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
	"github.com/sambly/exchangebot/internal/strategy"
	"github.com/sambly/exchangebot/internal/telegram"
	"github.com/sambly/exchangebot/internal/web"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

var cfg *config.Config

func init() {
	rootCmd.PersistentFlags().Bool("debug-log", false, "debug log mode")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Enable environment variable binding, the env vars are not overloaded yet.
	viper.AutomaticEnv()

	// setup the config paths for looking up the config file
	/*
		viper.AddConfigPath("config")
		viper.AddConfigPath("$HOME/.bbgo")
		viper.AddConfigPath("/etc/bbgo")

		// set the config file name and format for loading the config file.
		viper.SetConfigName("bbgo")
		viper.SetConfigType("yaml")

		err := viper.ReadInConfig()
		if err != nil {
			log.WithError(err).Fatal("failed to load config file")
		}
	*/
	// Once the flags are defined, we can bind config keys with flags.
	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.Fatalf("failed to bind persistent flags. please check the flag settings. %v", err)
		return
	}

	var err error
	cfg, err = config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	logger.InitLogger(cfg.DebugLog, cfg.ProductionLog)

}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "exchangebot",
	Short: "exchangebot",
	Long:  `exchangebot`,

	SilenceUsage: true,

	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("Helloo")

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

		dataFeed := exchange.NewDataFeedWithExchange(
			binance,
			logadapter.NewLogrusAdapter(logger.AddFieldsEmpty()),
		)

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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func main() {

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("cannot execute command. %v", err)
	}
}
