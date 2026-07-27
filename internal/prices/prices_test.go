package prices

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
)

// stubRepo отдаёт заранее заданные срезы свечей. Каждый вызов
// SelectMarketStateTimev2 берёт следующий срез из очереди (пустая очередь -> nil),
// что позволяет проигрывать последовательность минутных обновлений.
type stubRepo struct {
	batches [][]exModel.Candle
	calls   int

	// periodCandles - что отдавать на SelectCandlesFromPeriod (сидирование
	// волатильности и истории anomaly). Пусто по умолчанию - большинству
	// тестов сидирование не нужно и не должно ничего находить.
	periodCandles []exModel.Candle
}

func (r *stubRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) {
	if r.calls >= len(r.batches) {
		return nil, nil
	}
	batch := r.batches[r.calls]
	r.calls++
	return batch, nil
}

func (r *stubRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}

func (r *stubRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return r.periodCandles, nil
}

func newTestPrices(t *testing.T, pairs []string, periods map[string]time.Duration, repo Repository) *AssetsPrices {
	t.Helper()
	ap, err := NewAssetsPrices(pairs, periods, periods, repo)
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	return ap
}

func TestCheckValuesDividing(t *testing.T) {
	cases := []struct {
		name       string
		current    float64
		previous   float64
		wantResult float64
	}{
		{"рост вдвое", 200, 100, 100},
		{"падение вдвое", 50, 100, -50},
		{"без изменений", 100, 100, 0},
		// Ноль в любой из частей означает "изменения нет": иначе получили бы
		// деление на ноль или бесконечный процент роста от нуля.
		{"нулевой знаменатель", 100, 0, 0},
		{"нулевой числитель", 0, 100, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := checkValuesDividing(c.current, c.previous)
			if got != c.wantResult {
				t.Errorf("checkValuesDividing(%v, %v) = %v, ожидалось %v", c.current, c.previous, got, c.wantResult)
			}
		})
	}
}

// Каждый подписчик должен получить тик. Раньше UpdateChanel был один на всех,
// и обновление доставалось только одной стратегии из нескольких.
func TestSubscribeFanOut(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})

	first := ap.Subscribe()
	second := ap.Subscribe()

	ap.broadcastUpdate()

	for i, ch := range []<-chan struct{}{first, second} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("подписчик %d не получил обновление", i)
		}
	}
}

// Медленный подписчик не должен тормозить рассылку остальным.
func TestBroadcastDoesNotBlockOnSlowSubscriber(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})

	slow := ap.Subscribe() // никто не читает
	_ = slow

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			ap.broadcastUpdate()
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcastUpdate заблокировался на непрочитанном канале")
	}
}

// Скользящее окно цен: пока не набралось period значений, ChangePercent не
// считается; после заполнения он равен изменению за период, а не за минуту.
func TestUpdateChangePricesSlidingWindow(t *testing.T) {
	const period = "3m"
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 3 * time.Minute}, &stubRepo{})

	// Холодный старт: прогрева из БД нет, окно набирается по минуте.
	// Цены: 100, 110, 120, затем 240.
	minute := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	tick := func(price float64) {
		ap.MarketsStat["BTCUSDT"].Price = price
		ap.MarketsStat["BTCUSDT"].Time = minute
		minute = minute.Add(time.Minute)
		ap.updateChangePrices()
	}

	for _, price := range []float64{100, 110, 120} {
		tick(price)
	}

	dataset := ap.ChangePricesDataset["BTCUSDT"][period]
	if !dataset.Fill {
		t.Fatalf("после %d обновлений окно должно быть заполнено", 3)
	}

	// Окно хранится от новых к старым
	newest, _ := dataset.newest()
	oldest, _ := dataset.oldest()
	if newest.Price != 120 || oldest.Price != 100 {
		t.Fatalf("порядок окна нарушен: голова %v, хвост %v (ожидалось 120 и 100)", newest.Price, oldest.Price)
	}

	// LastPrice - цена в начале окна, то есть period минут назад
	if got := ap.ChangePrices["BTCUSDT"][period].LastPrice; got != 100 {
		t.Fatalf("LastPrice = %v, ожидалось 100 (цена в начале окна)", got)
	}

	// Следующее обновление: цена 240 против 100 в начале окна = +140%
	tick(240)

	if got := ap.ChangePrices["BTCUSDT"][period].ChangePercent; got < 139.99 || got > 140.01 {
		t.Fatalf("ChangePercent = %+.2f%%, ожидалось +140%% (240 против 100)", got)
	}

	// Окно сдвинулось: старейшая цена (100) выпала, теперь начало окна - 110
	if got := ap.ChangePrices["BTCUSDT"][period].LastPrice; got != 110 {
		t.Fatalf("после сдвига LastPrice = %v, ожидалось 110", got)
	}
}

// Если по паре перестали приходить тики, время в MarketsStat не растёт.
// Окно не должно набиваться повторами одной и той же цены: иначе оно
// "покроет" период, которого на самом деле не было.
func TestUpdateChangePricesIgnoresStaleTicks(t *testing.T) {
	const period = "3m"
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 3 * time.Minute}, &stubRepo{})

	at := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	// Первый тик проходит
	ap.MarketsStat["BTCUSDT"].Price = 100
	ap.MarketsStat["BTCUSDT"].Time = at
	ap.updateChangePrices()

	// Фид застыл: время то же самое
	for i := 0; i < 5; i++ {
		ap.updateChangePrices()
	}

	if got := len(ap.ChangePricesDataset["BTCUSDT"][period].values()); got != 1 {
		t.Fatalf("в окне %d значений, ожидалось 1: повторы одного тика не должны его набивать", got)
	}
	if ap.ChangePricesDataset["BTCUSDT"][period].Fill {
		t.Fatal("окно не должно считаться заполненным на застывшем фиде")
	}
}

func candle(pair string, at time.Time, volume float64) exModel.Candle {
	return exModel.Candle{
		Pair:            pair,
		Time:            at,
		Close:           100,
		Volume:          volume,
		ActiveBuyVolume: volume / 2,
		AmountTrade:     10,
		AmountTradeBuy:  5,
	}
}

// Дельта = объём за последние period против предыдущих period.
// Запрос к БД берёт минуту с запасом, поэтому свечи приходят с перекрытием -
// дубликаты не должны попадать в окно, иначе оно покрывает меньше времени,
// чем должно, и дельта уезжает.
func TestUpdateChangeDeltaDeduplicatesCandles(t *testing.T) {
	const period = "2m" // окно = 2 * 2 = 4 минутные свечи
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	// Объёмы по минутам. Первые две свечи вытеснятся из окна: значение дельты
	// считается только СО СЛЕДУЮЩЕГО обновления после заполнения окна, поэтому
	// свечей нужно больше, чем размер окна.
	// Итоговое окно - последние 4 свечи: старая половина по 100, новая по 300.
	volumes := []float64{50, 50, 100, 100, 300, 300}

	repo := &stubRepo{}
	for i := range volumes {
		// Каждый батч: свежая свеча + повтор предыдущей (как отдаёт БД, DESC)
		batch := []exModel.Candle{candle("BTCUSDT", base.Add(time.Duration(i)*time.Minute), volumes[i])}
		if i > 0 {
			batch = append(batch, candle("BTCUSDT", base.Add(time.Duration(i-1)*time.Minute), volumes[i-1]))
		}
		repo.batches = append(repo.batches, batch)
	}

	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 2 * time.Minute}, repo)
	// NewAssetsPrices уже израсходовал батчи на init-запросы - откручиваем назад
	repo.calls = 0

	for range repo.batches {
		if err := ap.updateChangeDelta(); err != nil {
			t.Fatalf("updateChangeDelta: %v", err)
		}
	}

	dataset := ap.ChangeDeltaDataset["BTCUSDT"][period]
	items := dataset.values()

	if len(items) != 4 {
		t.Fatalf("в окне %d свечей, ожидалось 4 (дубликаты не отфильтрованы?)", len(items))
	}
	if !dataset.fill {
		t.Fatal("окно должно быть заполнено")
	}

	seen := make(map[time.Time]bool)
	for i, item := range items {
		if seen[item.Time] {
			t.Fatalf("дубликат свечи %v в окне", item.Time)
		}
		seen[item.Time] = true

		// Порядок: от новых к старым
		if i > 0 && !items[i-1].Time.After(item.Time) {
			t.Fatalf("порядок нарушен: [%d]=%v не новее [%d]=%v", i-1, items[i-1].Time, i, item.Time)
		}
	}

	// Новая половина 300+300=600 против старой 100+100=200 => +200%
	if got := ap.ChangeDelta["BTCUSDT"][period].Volume; got < 199.9 || got > 200.1 {
		t.Fatalf("дельта объёма = %+.2f%%, ожидалось +200%%", got)
	}
}

// Пока окно дельт не заполнено, значение не считается: иначе сравнивались бы
// половины разной длины.
func TestChangeDeltaNotReadyUntilWindowFilled(t *testing.T) {
	const period = "2m"
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	repo := &stubRepo{batches: [][]exModel.Candle{
		{candle("BTCUSDT", base, 100)},
		{candle("BTCUSDT", base.Add(time.Minute), 500)},
	}}

	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 2 * time.Minute}, repo)
	repo.calls = 0

	for range repo.batches {
		if err := ap.updateChangeDelta(); err != nil {
			t.Fatalf("updateChangeDelta: %v", err)
		}
	}

	if ap.ChangeDeltaDataset["BTCUSDT"][period].fill {
		t.Fatal("окно из 2 свечей не может быть заполнено (нужно 4)")
	}
	if got := ap.ChangeDelta["BTCUSDT"][period].Volume; got != 0 {
		t.Fatalf("до заполнения окна дельта должна быть 0, получено %v", got)
	}
}

// Геттеры отдают данные только по заполненным датасетам и не пропускают наружу
// указатели на внутренние структуры.
func TestGettersRespectFillFlag(t *testing.T) {
	const period = "2m"
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 2 * time.Minute}, &stubRepo{})

	if _, ok := ap.GetChangePrices("BTCUSDT", period); ok {
		t.Error("GetChangePrices не должен отдавать данные по незаполненному окну")
	}
	if _, ok := ap.GetChangeDelta("BTCUSDT", period); ok {
		t.Error("GetChangeDelta не должен отдавать данные по незаполненному окну")
	}
	if _, ok := ap.GetChangePrices("UNKNOWNUSDT", period); ok {
		t.Error("GetChangePrices не должен отдавать данные по неизвестной паре")
	}
}
