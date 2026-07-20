package order_test

import (
	"sync"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/paperwallet"
	"github.com/sambly/exchangebot/internal/prices"
)

type stubPricesRepo struct{}

func (stubPricesRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) {
	return nil, nil
}
func (stubPricesRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (stubPricesRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return nil, nil
}

type stubOrderRepo struct {
	mu     sync.Mutex
	closed int
}

func (*stubOrderRepo) GetAll() ([]*order.Order, error) { return nil, nil }
func (*stubOrderRepo) Create(*order.Order) error       { return nil }
func (r *stubOrderRepo) ClosePosition(int64, *order.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed++
	return nil
}
func (*stubOrderRepo) CreateInfo(*order.OrderInfo) error     { return nil }
func (*stubOrderRepo) ClearSalePolicyForActiveOrders() error { return nil }

func newTestService(t *testing.T) (*order.OrderService, *paperwallet.PaperWallet, *notification.SocketsMessage) {
	t.Helper()

	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{"BTCUSDT"}, periods, periods, stubPricesRepo{})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 100, Time: time.Now()})

	pw := paperwallet.NewPaperWallet(ap)
	sockets := notification.NewSocketsMessage()

	svc, err := order.NewOrderService(&stubOrderRepo{}, pw, sockets, ap)
	if err != nil {
		t.Fatalf("NewOrderService: %v", err)
	}
	return svc, pw, sockets
}

// OnMarket исполняется внутри горутины фида, а observer'ы в exchangeService
// вызываются синхронно при чтении потока. Значит блокировка здесь - это
// остановка чтения котировок с биржи, и её быть не должно, даже если очередь
// веб-сокетов никто не разбирает.
func TestOnMarketDoesNotBlockWithoutSocketConsumer(t *testing.T) {
	svc, pw, sockets := newTestService(t)

	if _, err := svc.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy}); err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Параллельное чтение ордеров: раньше OnMarket писал в них по
			// указателю мимо мьютекса кошелька
			for i := 0; i < 2000; i++ {
				_ = pw.GetOrdersActiveCopy()
			}
		}()

		for i := 0; i < 5000; i++ {
			svc.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 100 + float64(i%10), Time: time.Now()})
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("OnMarket заблокировался: очередь веб-сокетов никем не читается")
	}

	// Пуш троттлится до раза в секунду на пару, поэтому 5000 тиков не должны
	// превратиться в 10000 сообщений
	if got := len(sockets.Message); got > 20 {
		t.Fatalf("в очереди %d сообщений - троттлинг не работает", got)
	}
}

// Закрытие несуществующего ордера должно возвращать ошибку, а не ронять процесс
// на разыменовании nil.
func TestClosePositionUnknownID(t *testing.T) {
	svc, _, _ := newTestService(t)

	if err := svc.ClosePosition(999, order.Deal{Strategy: "manual"}); err == nil {
		t.Fatal("ожидалась ошибка при закрытии несуществующего ордера")
	}
}

// Два одновременных закрытия одного ордера: позиция должна закрыться ровно один
// раз, второй вызов - ошибка.
func TestClosePositionConcurrentSameID(t *testing.T) {
	svc, _, _ := newTestService(t)

	created, err := svc.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = svc.ClosePosition(created.ID, order.Deal{Strategy: "manual"})
		}(i)
	}
	wg.Wait()

	success := 0
	for _, err := range errs {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("позиция должна закрыться ровно один раз, успешных закрытий: %d", success)
	}
}
