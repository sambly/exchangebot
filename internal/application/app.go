package application

import (
	"context"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	pb "github.com/sambly/exchangeService/pkg/pb"
	"github.com/sambly/exchangeService/pkg/telemetry"
	"github.com/sambly/exchangebot/internal/account"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/database"
	"github.com/sambly/exchangebot/internal/depth"
	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/paperwallet"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type Application struct {
	Settings model.Settings
	Config   *config.Config

	Notification *notification.Notification

	exchange exchange.Exchange
	dataFeed *exchange.DataFeed

	Account      *account.Account
	AssetsPrices *prices.AssetsPrices
	AssetsDepth  *depth.AssetsDepth
	AssetsSetup  *entrysetup.AssetsSetup

	OrderController    *order.OrderService
	PaperWallet        *paperwallet.PaperWallet
	ControllerStrategy *strategy.ControllerStrategy

	// statusClient читает статус подписки на пару прямо из exchange_service
	// (RPC GetAllMarketPairsStatus). nil в режиме прямого подключения к бирже.
	statusClient pb.ExchangeServiceClient
}

var appLogger = logger.AddFieldsEmpty()

func NewApp(
	exch exchange.Exchange,
	dataFeed *exchange.DataFeed,
	settings model.Settings,
	db *gorm.DB,
	socketsMessage *notification.SocketsMessage,
	cfg *config.Config,
	notification *notification.Notification,
	statusClient pb.ExchangeServiceClient) (*Application, error) {

	orderDB := database.NewOrderDb(db)
	pricesDB := database.NewPricesDb(db)

	assetsPrices, err := prices.NewAssetsPrices(settings.Pairs, settings.ChangePeriods, settings.DeltaPeriods, pricesDB)
	if err != nil {
		return nil, err
	}
	assetsDepth := depth.NewAssetsDepth(depthPairs(cfg, settings.Pairs))
	assetsSetup := entrysetup.NewAssetsSetup(assetsPrices, assetsDepth)

	account, err := account.NewAccount(exch, assetsPrices)
	if err != nil {
		return nil, err
	}
	paperWallet := paperwallet.NewPaperWallet(assetsPrices)
	orderController, err := order.NewOrderService(orderDB, paperWallet, socketsMessage, assetsPrices)
	if err != nil {
		return nil, err
	}

	controllerStrategy, err := strategy.NewControllerStrategy(cfg, assetsPrices, settings.ChangePeriods, settings.Pairs, notification, orderController, assetsSetup)
	if err != nil {
		return nil, err
	}

	app := &Application{
		Settings: settings,
		Config:   cfg,
		exchange: exch,
		dataFeed: dataFeed,

		Notification: notification,

		AssetsPrices:       assetsPrices,
		AssetsDepth:        assetsDepth,
		AssetsSetup:        assetsSetup,
		Account:            account,
		OrderController:    orderController,
		PaperWallet:        paperWallet,
		ControllerStrategy: controllerStrategy,
		statusClient:       statusClient,
	}

	return app, nil
}

// depthPairs решает, по каким парам держать L2-стакан: либо все настроенные
// пары (cfg.Depth.AllPairs), либо явный список из cfg.Depth.Pairs. Список
// фильтруется по settings.Pairs, чтобы опечатка в конфиге не пыталась
// подписаться на пару, которую бот вообще не отслеживает.
func depthPairs(cfg *config.Config, allPairs []string) []string {
	if cfg.Depth.AllPairs {
		return allPairs
	}
	if len(cfg.Depth.Pairs) == 0 {
		return nil
	}

	known := make(map[string]bool, len(allPairs))
	for _, pair := range allPairs {
		known[pair] = true
	}

	pairs := make([]string, 0, len(cfg.Depth.Pairs))
	for _, pair := range cfg.Depth.Pairs {
		if known[pair] {
			pairs = append(pairs, pair)
		} else {
			appLogger.Warnf("depth: пара %q из конфига не найдена среди отслеживаемых пар, пропущена", pair)
		}
	}
	return pairs
}

func (app *Application) Run(ctx context.Context) error {

	appLogger.Info("Ожидание предварительной загрузки данных")

	timeStart := time.Now()
	shouldBreak := false

	// Ожидание, пока текущее время не попадет в интервал от 10 до 50 секунд
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeNow := time.Now()
			seconds := timeNow.Second()
			if seconds >= 10 && seconds <= 50 {
				shouldBreak = true
				break
			}
			time.Sleep(1 * time.Second)
		}
		if shouldBreak {
			break
		}
	}

	observers := []func(market exModel.MarketsStat){
		func(market exModel.MarketsStat) {
			app.AssetsPrices.OnMarket(market)
			app.OrderController.OnMarket(market)
			app.ControllerStrategy.OnMarket(market)
		},
	}

	ctx, rootSpan := telemetry.Tracer.Start(ctx, "app")
	defer rootSpan.End()

	g, gCtx := errgroup.WithContext(ctx)

	for _, pair := range app.Settings.Pairs {

		app.dataFeed.SubscribeMarketsStat(pair)
		for _, observer := range observers {
			if err := app.dataFeed.SubscribeObserverMarkets(gCtx, "exchangebot", pair, observer); err != nil {
				appLogger.Error(err)
			}
		}
	}

	g.Go(func() error {
		return app.dataFeed.StartMarketsStatFeeder(gCtx, "exchangebot")
	})

	// Depth-подписка - только по парам из app.AssetsDepth.Pairs (см. depthPairs):
	// по умолчанию список пуст, и StartDepthFeeder на пустом наборе пар сразу
	// вернул бы ошибку "all depth-feeder subscription ended", уронив весь g.Wait().
	if len(app.AssetsDepth.Pairs) > 0 {
		for _, pair := range app.AssetsDepth.Pairs {
			app.dataFeed.SubscribeDepth(pair)
			if err := app.dataFeed.SubscribeObserverDepth(gCtx, "exchangebot", pair, app.AssetsDepth.OnDepth); err != nil {
				appLogger.Error(err)
			}
		}

		g.Go(func() error {
			return app.dataFeed.StartDepthFeeder(gCtx, "exchangebot")
		})
	}

	g.Go(func() error {
		return app.ControllerStrategy.StartAll(gCtx)
	})

	duration := time.Since(timeStart)
	appLogger.Infof("Время выполнения предварительной загрузки данных: %v ", duration)
	appLogger.Infof("Время старта: %v ", timeStart)

	return g.Wait()
}

// GetMarketPairsStatus читает статус подписки на каждую пару прямо из
// exchange_service (Active/Inactive). Вызывается по запросу из веб-хендлера,
// а не фоновой горутиной: статус нужен ровно тогда, когда фронт обновляет цены.
//
// Статус нельзя получить из самого потока MarketsStat: когда пара Inactive,
// по ней не идёт ни одного сообщения, поэтому источник - отдельный unary-RPC.
//
// В режиме прямого подключения к бирже (statusClient == nil) и при любой
// ошибке возвращается пустая карта: индикатор статуса - подсказка, а не то,
// ради чего стоит ронять весь ответ с ценами.
func (app *Application) GetMarketPairsStatus(ctx context.Context) map[string]string {
	if app.statusClient == nil {
		return map[string]string{}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := app.statusClient.GetAllMarketPairsStatus(ctx, &pb.Empty{})
	if err != nil {
		appLogger.Errorf("get all market pairs status: %v", err)
		return map[string]string{}
	}

	out := make(map[string]string, len(resp.Pairs))
	for _, p := range resp.Pairs {
		out[p.Pair] = p.Status
	}
	return out
}
