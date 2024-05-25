package exchange

import (
	"context"
	"errors"
	"fmt"
	"main/logging"
	"main/model"
	"main/service"
	"sync"

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

	//d.Pairs = append(d.Pairs, pair)
	d.SubscriptionsByDataFeed[pair] = append(d.SubscriptionsByDataFeed[pair], Subscription{
		consumer: consumer,
	})
}

func (d *DataFeedSubscription) Connect() {
	for _, pair := range d.Pairs {
		cmarket, cerr := d.exchange.MarketsSubscription(context.Background(), pair)
		d.MarketsStatFeeds[pair] = &MarketsStatFeed{
			Data: cmarket,
			Err:  cerr,
		}
	}
}

func (d *DataFeedSubscription) Start(loadSync bool) {
	d.Connect()
	wg := new(sync.WaitGroup)
	for key, feed := range d.MarketsStatFeeds {
		wg.Add(1)
		go func(key string, feed *MarketsStatFeed) {
			for {
				select {
				case cmarket, ok := <-feed.Data:
					if !ok {
						wg.Done()
						return
					}
					for _, subscription := range d.SubscriptionsByDataFeed[key] {
						subscription.consumer(cmarket)
					}
				case err := <-feed.Err:
					if err != nil {
						logging.MyLogger.ErrorOut(fmt.Errorf("error MarketsStatFeed : %v", err))
						fmt.Println(err)
					}
				}
			}
		}(key, feed)
	}
	if loadSync {
		wg.Wait()
	}
}
