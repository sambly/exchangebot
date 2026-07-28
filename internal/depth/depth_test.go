package depth

import (
	"math"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/order"
)

// До первого апдейта пара не готова, и снапшот/best bid-ask её не отдают -
// потребитель не должен принять пустой стакан за "нет ликвидности".
func TestAssetsDepthNotReadyBeforeFirstUpdate(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	if ad.IsReady("BTCUSDT") {
		t.Fatal("пара не должна быть готова до первого апдейта")
	}
	if _, _, ok := ad.GetBestBidAsk("BTCUSDT"); ok {
		t.Fatal("GetBestBidAsk не должен отдавать данные до готовности")
	}
	if snap, ok := ad.GetSnapshot("BTCUSDT"); !ok || snap.Ready {
		t.Fatalf("GetSnapshot: ожидали Ready=false, получили %+v (ok=%v)", snap, ok)
	}
}

// Пара вне списка отслеживаемых - OnDepth должен молча игнорировать апдейт,
// а не создавать новую запись на лету.
func TestAssetsDepthIgnoresUnknownPair(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "ETHUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 1}},
	})

	if ad.IsReady("ETHUSDT") {
		t.Fatal("ETHUSDT не отслеживается и не должен становиться готовым")
	}
	if _, ok := ad.GetSnapshot("ETHUSDT"); ok {
		t.Fatal("GetSnapshot не должен находить неотслеживаемую пару")
	}
}

// Основной сценарий: первый апдейт (снапшот) заполняет стакан, второй
// (diff) добавляет/меняет уровни, а количество 0 удаляет уровень целиком -
// именно так работает diff-протокол Binance depth.
func TestAssetsDepthApplyUpdateMergeAndDelete(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	now := time.Now()
	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Time: now,
		Bids: []exModel.DepthLevel{
			{Price: 100, Quantity: 1},
			{Price: 99, Quantity: 2},
		},
		Asks: []exModel.DepthLevel{
			{Price: 101, Quantity: 1},
			{Price: 102, Quantity: 3},
		},
		LastUpdateID: 10,
	})

	if !ad.IsReady("BTCUSDT") {
		t.Fatal("пара должна быть готова после первого апдейта")
	}

	snap, ok := ad.GetSnapshot("BTCUSDT")
	if !ok {
		t.Fatal("GetSnapshot: пара не найдена")
	}
	if len(snap.Bids) != 2 || len(snap.Asks) != 2 {
		t.Fatalf("ожидали 2 бида и 2 аска, получили bids=%v asks=%v", snap.Bids, snap.Asks)
	}
	if snap.LastUpdateID != 10 {
		t.Fatalf("LastUpdateID: ожидали 10, получили %d", snap.LastUpdateID)
	}

	// diff: меняем количество на 99, добавляем новый уровень 98, и убираем 101 (qty=0)
	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Time: now.Add(time.Second),
		Bids: []exModel.DepthLevel{
			{Price: 99, Quantity: 5},
			{Price: 98, Quantity: 1},
		},
		Asks: []exModel.DepthLevel{
			{Price: 101, Quantity: 0},
		},
		LastUpdateID: 11,
	})

	snap, ok = ad.GetSnapshot("BTCUSDT")
	if !ok {
		t.Fatal("GetSnapshot: пара не найдена после diff")
	}
	if len(snap.Bids) != 3 {
		t.Fatalf("ожидали 3 бида (100, 99 обновлён, 98 новый), получили %v", snap.Bids)
	}
	if snap.Bids[99] != 5 {
		t.Fatalf("ожидали обновлённое количество 5 на цене 99, получили %v", snap.Bids[99])
	}
	if _, exists := snap.Asks[101]; exists {
		t.Fatalf("уровень 101 с qty=0 должен быть удалён, получили %v", snap.Asks)
	}
	if len(snap.Asks) != 1 {
		t.Fatalf("ожидали 1 аск после удаления 101, получили %v", snap.Asks)
	}
}

// GetBestBidAsk должен находить максимальный бид и минимальный аск, а не
// первый попавшийся элемент карты.
func TestAssetsDepthGetBestBidAsk(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{
			{Price: 99, Quantity: 2},
			{Price: 100, Quantity: 1}, // лучший бид - самая высокая цена
			{Price: 98, Quantity: 4},
		},
		Asks: []exModel.DepthLevel{
			{Price: 105, Quantity: 2},
			{Price: 101, Quantity: 1}, // лучший аск - самая низкая цена
			{Price: 103, Quantity: 4},
		},
	})

	bid, ask, ok := ad.GetBestBidAsk("BTCUSDT")
	if !ok {
		t.Fatal("GetBestBidAsk: ожидался успех")
	}
	if bid.Price != 100 || bid.Quantity != 1 {
		t.Fatalf("лучший бид: ожидали {100 1}, получили %+v", bid)
	}
	if ask.Price != 101 || ask.Quantity != 1 {
		t.Fatalf("лучший аск: ожидали {101 1}, получили %+v", ask)
	}
}

// DepthState{Ready: false} должен полностью очищать стакан (ре-синк начинается
// с чистого листа) и сбрасывать готовность.
func TestAssetsDepthNotReadyClearsBook(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 1}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 1}},
	})
	if !ad.IsReady("BTCUSDT") {
		t.Fatal("пара должна быть готова после апдейта")
	}

	ad.OnDepth(exModel.DepthState{Pair: "BTCUSDT", Ready: false})

	if ad.IsReady("BTCUSDT") {
		t.Fatal("пара не должна быть готова после DepthState{Ready: false}")
	}
	snap, ok := ad.GetSnapshot("BTCUSDT")
	if !ok {
		t.Fatal("GetSnapshot: пара должна оставаться известной")
	}
	if len(snap.Bids) != 0 || len(snap.Asks) != 0 {
		t.Fatalf("стакан должен быть очищен, получили bids=%v asks=%v", snap.Bids, snap.Asks)
	}
}

// GetTopLevels должен отдавать bids по убыванию цены, asks по возрастанию, и
// обрезать до n уровней с каждой стороны - именно в таком порядке cumulative
// depth chart строит накопление от спреда наружу.
func TestAssetsDepthGetTopLevels(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{
			{Price: 98, Quantity: 3},
			{Price: 100, Quantity: 1},
			{Price: 99, Quantity: 2},
		},
		Asks: []exModel.DepthLevel{
			{Price: 103, Quantity: 3},
			{Price: 101, Quantity: 1},
			{Price: 102, Quantity: 2},
		},
	})

	bids, asks, ready := ad.GetTopLevels("BTCUSDT", 2)
	if !ready {
		t.Fatal("GetTopLevels: ожидался ready=true")
	}

	wantBids := []Level{{Price: 100, Quantity: 1}, {Price: 99, Quantity: 2}}
	if len(bids) != len(wantBids) {
		t.Fatalf("bids: ожидали %d уровня, получили %d: %v", len(wantBids), len(bids), bids)
	}
	for i, want := range wantBids {
		if bids[i] != want {
			t.Fatalf("bids[%d]: ожидали %+v, получили %+v", i, want, bids[i])
		}
	}

	wantAsks := []Level{{Price: 101, Quantity: 1}, {Price: 102, Quantity: 2}}
	if len(asks) != len(wantAsks) {
		t.Fatalf("asks: ожидали %d уровня, получили %d: %v", len(wantAsks), len(asks), asks)
	}
	for i, want := range wantAsks {
		if asks[i] != want {
			t.Fatalf("asks[%d]: ожидали %+v, получили %+v", i, want, asks[i])
		}
	}
}

// Сырой имбаланс должен корректно отражать перекос топ-уровней: больше
// объёма на бидах - положительное значение, больше на асках - отрицательное.
func TestAssetsDepthImbalanceSign(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 90}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 10}},
	})

	imbalance, _, ready := ad.GetImbalanceZScore("BTCUSDT")
	if !ready {
		t.Fatal("GetImbalanceZScore: ожидался ready=true")
	}
	want := (90.0 - 10.0) / (90.0 + 10.0)
	if math.Abs(imbalance-want) > 1e-9 {
		t.Fatalf("imbalance: ожидали %.4f, получили %.4f", want, imbalance)
	}
}

// Пока история имбаланса не набрала imbalanceMinSamples выборок, z-score
// должен быть 0 - как и у stat.MetricRecord, на котором это построено.
func TestAssetsDepthImbalanceZScoreZeroBeforeMinSamples(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 90}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 10}},
	})

	_, zScore, ready := ad.GetImbalanceZScore("BTCUSDT")
	if !ready {
		t.Fatal("GetImbalanceZScore: ожидался ready=true")
	}
	if zScore != 0 {
		t.Fatalf("zScore до накопления истории должен быть 0, получено %v", zScore)
	}
}

// Резкий перекос книги относительно её же обычного (сбалансированного)
// поведения должен давать большой z-score - в этом весь смысл: судить не по
// сырому имбалансу, а по отклонению от нормы для конкретной пары.
func TestAssetsDepthImbalanceZScoreDetectsSkew(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	// Бутстрап: нейтральная книга, чтобы пара стала ready.
	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 10}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 10}},
	})

	b := ad.books["BTCUSDT"]

	// Замораживаем окно сэмплирования в истории (иначе следующий апдейт сам
	// допишет в неё значение и смажет чистоту сравнения) и набиваем историю
	// "типичным" для пары нейтральным имбалансом - без этого пришлось бы
	// реально ждать imbalanceSampleInterval между апдейтами.
	b.nextImbalanceSampleAt = time.Now().Add(time.Hour)
	for i := 0; i < imbalanceMinSamples; i++ {
		b.imbalanceHistory.Add(0)
	}

	// Теперь резко перекашиваем книгу в сторону бидов.
	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 100}},
	})

	imbalance, zScore, ready := ad.GetImbalanceZScore("BTCUSDT")
	if !ready {
		t.Fatal("GetImbalanceZScore: ожидался ready=true")
	}
	if imbalance <= 0.5 {
		t.Fatalf("после перекоса книги ожидался явно положительный имбаланс, получено %.4f", imbalance)
	}
	if zScore < 3 {
		t.Fatalf("резкий перекос относительно нейтральной истории должен давать большой z-score, получено %.2f", zScore)
	}
}

// GetAllImbalanceZScore должен отдавать по каждой отслеживаемой паре свой
// результат, включая Ready=false для пар, по которым апдейтов ещё не было -
// а не пропускать их из карты молча.
func TestAssetsDepthGetAllImbalanceZScore(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT", "ETHUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 90}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 10}},
	})

	all := ad.GetAllImbalanceZScore()
	if len(all) != 2 {
		t.Fatalf("ожидались обе пары в результате, получено %d: %+v", len(all), all)
	}

	btc, ok := all["BTCUSDT"]
	if !ok || !btc.Ready {
		t.Fatalf("BTCUSDT: ожидался Ready=true, получено %+v (ok=%v)", btc, ok)
	}
	if btc.Imbalance <= 0.5 {
		t.Fatalf("BTCUSDT: ожидался явно положительный имбаланс, получено %.4f", btc.Imbalance)
	}

	eth, ok := all["ETHUSDT"]
	if !ok {
		t.Fatal("ETHUSDT: пара должна присутствовать в результате даже без апдейтов")
	}
	if eth.Ready {
		t.Fatalf("ETHUSDT: ожидался Ready=false (апдейтов не было), получено %+v", eth)
	}
}

// Один резкий перекос не должен считаться "подтверждённым" сразу - нужно,
// чтобы он держался imbalanceConfirmMinStreak сэмплов подряд. Это и есть
// защита от спуфинга/шума одной заявки, ради которой счётчик заводился.
func TestAssetsDepthImbalanceConfirmedSideRequiresStreak(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 10}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 10}},
	})

	b := ad.books["BTCUSDT"]
	b.nextImbalanceSampleAt = time.Now().Add(time.Hour)
	for i := 0; i < imbalanceMinSamples; i++ {
		b.imbalanceHistory.Add(0)
	}

	// Один сэмпл с резким перекосом бидов - сторона ещё не должна считаться
	// подтверждённой, стрик только начался.
	sampleSkewedBook(ad, b)
	if _, confirmed := ad.GetImbalanceConfirmedSide("BTCUSDT"); confirmed {
		t.Fatal("после одного сэмпла сторона не должна считаться подтверждённой")
	}

	// Ещё два сэмпла того же перекоса подряд - теперь стрик достиг порога.
	sampleSkewedBook(ad, b)
	sampleSkewedBook(ad, b)

	side, confirmed := ad.GetImbalanceConfirmedSide("BTCUSDT")
	if !confirmed || side != order.SideTypeBuy {
		t.Fatalf("после %d сэмплов подряд ожидался Confirmed=BUY, получено side=%v confirmed=%v", imbalanceConfirmMinStreak, side, confirmed)
	}
}

// Разворот перекоса в другую сторону должен сбрасывать стрик - подтверждение
// не должно "унаследоваться" от предыдущей, уже неактуальной стороны.
func TestAssetsDepthImbalanceConfirmedSideResetsOnFlip(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 10}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 10}},
	})

	b := ad.books["BTCUSDT"]
	b.nextImbalanceSampleAt = time.Now().Add(time.Hour)
	for i := 0; i < imbalanceMinSamples; i++ {
		b.imbalanceHistory.Add(0)
	}

	for i := 0; i < imbalanceConfirmMinStreak; i++ {
		sampleSkewedBook(ad, b)
	}
	if _, confirmed := ad.GetImbalanceConfirmedSide("BTCUSDT"); !confirmed {
		t.Fatal("ожидался Confirmed=BUY перед проверкой сброса")
	}

	// Резкий перекос в противоположную сторону - один сэмпл, сброс стрика.
	b.nextImbalanceSampleAt = time.Now().Add(-time.Second)
	ad.applyUpdate(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 1}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 100}},
	})

	if _, confirmed := ad.GetImbalanceConfirmedSide("BTCUSDT"); confirmed {
		t.Fatal("после разворота в другую сторону подтверждение должно сброситься, а не остаться от BUY")
	}
}

// sampleSkewedBook - один "тик" резко перекошенного в сторону бидов стакана,
// с разморозкой троттлинга сэмплирования перед апдейтом (иначе имбаланс
// посчитается, но в историю/счётчик устойчивости не попадёт - см.
// nextImbalanceSampleAt).
func sampleSkewedBook(ad *AssetsDepth, b *book) {
	b.nextImbalanceSampleAt = time.Now().Add(-time.Second)
	ad.applyUpdate(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: []exModel.DepthLevel{{Price: 100, Quantity: 100}},
		Asks: []exModel.DepthLevel{{Price: 101, Quantity: 10}},
	})
}

// Неизвестный тип payload (не DepthUpdate/DepthState) должен игнорироваться,
// а не паниковать - OnDepth принимает any по контракту DataFeed.SubscribeObserverDepth.
func TestAssetsDepthOnDepthIgnoresUnknownType(t *testing.T) {
	ad := NewAssetsDepth([]string{"BTCUSDT"})

	ad.OnDepth("не тот тип")
	ad.OnDepth(42)

	if ad.IsReady("BTCUSDT") {
		t.Fatal("неизвестный payload не должен помечать пару готовой")
	}
}
