package exchange

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sambly/exchangeBot/internal/logging"
	"github.com/sambly/exchangeBot/internal/model"
	"github.com/sambly/exchangeBot/internal/service"

	"golang.org/x/exp/slices"
)

var (
	ErrInvalidQuantity   = errors.New("invalid quantity")
	ErrInsufficientFunds = errors.New("insufficient funds or locked")
	ErrInvalidAsset      = errors.New("invalid asset")
)

type OrderError struct {
	Err      error
	Pair     string
	Quantity float64
}

func (o *OrderError) Error() string {
	return fmt.Sprintf("order error: %v", o.Err)
}

type MarketsStatFeed struct {
	Data chan model.MarketsStat
	Err  chan error
}

type DataFeedSubscription struct {
	wg                      *sync.WaitGroup
	exchange                service.Exchange
	Pairs                   []string
	MarketsStatFeeds        map[string]*MarketsStatFeed
	SubscriptionsByDataFeed map[string][]Subscription
}

type Subscription struct {
	consumer DataFeedConsumer
}
type DataFeedConsumer func(model.MarketsStat)

func NewDataFeed(exchange service.Exchange, pairs []string) *DataFeedSubscription {
	return &DataFeedSubscription{
		wg:                      &sync.WaitGroup{},
		exchange:                exchange,
		Pairs:                   pairs,
		MarketsStatFeeds:        make(map[string]*MarketsStatFeed),
		SubscriptionsByDataFeed: make(map[string][]Subscription),
	}
}
func (d *DataFeedSubscription) Subscribe(pair string, consumer DataFeedConsumer) {
	if idx := slices.Index(d.Pairs, pair); idx == -1 {
		d.Pairs = append(d.Pairs, pair)
	}
	d.SubscriptionsByDataFeed[pair] = append(d.SubscriptionsByDataFeed[pair], Subscription{
		consumer: consumer,
	})
}

func (d *DataFeedSubscription) Connect(ctx context.Context) {
	// Подписки на websocket
	for _, pair := range d.Pairs {
		d.wg.Add(1)
		cmarket, cerr := d.exchange.MarketsSubscription(ctx, pair, d.wg)
		d.MarketsStatFeeds[pair] = &MarketsStatFeed{
			Data: cmarket,
			Err:  cerr,
		}
	}
}

func (d *DataFeedSubscription) Start(ctx context.Context) error {
	d.Connect(ctx)
	for key, feed := range d.MarketsStatFeeds {
		d.wg.Add(1)
		go func(key string, feed *MarketsStatFeed) {
			defer d.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case cmarket, ok := <-feed.Data:
					if !ok {
						logging.MyLogger.InfoLog.Println("stopping data feed:", key)
						return
					}
					for _, subscription := range d.SubscriptionsByDataFeed[key] {
						subscription.consumer(cmarket)
					}

				case err := <-feed.Err:
					if err != nil {
						logging.MyLogger.ErrorOut(fmt.Errorf("error ws cmarket: %v", err))
					}
				}
			}
		}(key, feed)
	}
	// Завершение подписок по websocket
	d.wg.Wait()
	logging.MyLogger.InfoLog.Println("Все подписки завершены")
	return nil

}
