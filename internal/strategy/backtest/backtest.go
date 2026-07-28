// Package backtest - прогон стратегии по историческим свечам.
//
// Ключевой принцип: бэктестер НЕ СОДЕРЖИТ ни правил детекции, ни правил входа,
// ни правил выхода. Всё это он берёт у боевых компонентов:
//
//	детекция - Detector (anomaly отдаёт своё ядро через DetectCandle);
//	вход     - executor.Config.SideFor (тот же minLevel/kinds/maxExtension);
//	выход    - sales.Sales.ShouldExit (тот же тейк/стоп/таймаут).
//
// Бэктестер добавляет только то, чего в бою нет: подачу свечей по времени,
// симуляцию исполнения по high/low бара, комиссии и отчёт. Поэтому новая
// стратегия подключается сюда реализацией Detector - без единой строчки здесь.
package backtest

import (
	"sort"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/executor"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

// Detector - то, что бэктестер требует от стратегии-детектора: посмотреть на
// очередную закрытую свечу и сказать, есть ли сигнал. Историю, статистику и
// прочее состояние детектор ведёт сам - walk-forward обеспечивается тем, что
// свечи приходят строго по времени и только один раз.
type Detector interface {
	DetectCandle(pair, period string, prev, current exModel.Candle) (signal.Signal, bool)
}

// Options - параметры симуляции. Это параметры ИСПОЛНЕНИЯ, а не стратегии:
// стратегия целиком приходит через Detector/entry/exits.
type Options struct {
	// Period - сигнальный период ("15m"), PeriodDuration - его длительность
	Period         string
	PeriodDuration time.Duration

	// Periods - карта всех периодов приложения (нужна, чтобы перевести
	// extension.lookbackPeriod в длительность)
	Periods map[string]time.Duration

	// FeePercent - комиссия за ОДНУ сторону, % (Binance spot taker = 0.1).
	// Начисляется дважды: вход + выход.
	FeePercent float64

	// SlippagePercent - проскальзывание на сторону, %. Вход по рынку после
	// аномалии - это покупка в движение, идеального исполнения не бывает.
	SlippagePercent float64

	// EntryOffsetVol - ЛИМИТНЫЙ вход в откат вместо маркета в движение.
	//
	// 0 - вход по рынку на закрытии сигнальной свечи (как в бою сейчас).
	// N > 0 - лимитная заявка на N волатильностей пары ЛУЧШЕ закрытия
	// (для лонга - ниже, для шорта - выше). Гипотеза: половина минуса
	// стратегии - это цена входа; движение после аномалии в среднем
	// откатывается, и лимитка в откат покупает этот откат вместо вершины.
	// Цена: часть сигналов не исполняется вовсе (цена ушла без отката).
	//
	// Проскальзывание к лимитному входу не применяется - лимитка не может
	// исполниться хуже своей цены.
	EntryOffsetVol float64

	// EntryTTLBars - сколько баров живёт лимитная заявка. По истечении
	// снимается: откат, который не случился за пару периодов, уже не откат.
	EntryTTLBars int

	// MaxAnomalousShare - фильтр режима: если аномальна БОЛЬШАЯ доля пар
	// одновременно (0.05 = 5%), это движение всего рынка, а не событие в
	// одной паре, - и входы этого такта пропускаются. Локальная статистика
	// пары ничего не знает про обвал рынка; эта доля - знает.
	// 0 - фильтр выключен.
	MaxAnomalousShare float64
}

// Engine связывает детектор, правила входа и политику выхода с историей свечей
type Engine struct {
	detector Detector
	entry    *executor.Config
	exits    sales.Sales
	opt      Options
}

func New(detector Detector, entry *executor.Config, exits sales.Sales, opt Options) *Engine {
	return &Engine{detector: detector, entry: entry, exits: exits, opt: opt}
}

// simPosition - открытая позиция симуляции.
type simPosition struct {
	position sales.Position
	openTime time.Time

	// remainingFraction - доля ОТ ПЕРВОНАЧАЛЬНОГО объёма, ещё не закрытая
	// частичными тейками (см. tryPartialTakeProfit). 1.0, пока частичных
	// тейков не было - тогда весь финальный расчёт в closeTrade сводится
	// ровно к тому, что было и раньше.
	remainingFraction float64
	// realizedGross/realizedNet - взвешенный (по доле, закрытой на каждом
	// частичном срабатывании) вклад уже закрытых частей в итоговый % сделки.
	// Складывается с результатом финального закрытия в closeTrade, чтобы вся
	// позиция - даже закрытая в несколько приёмов - попала в отчёт ОДНОЙ
	// записью Trade (см. Report: "все проценты на сделку", а не на кусок
	// сделки - иначе partial+final считались бы как два разных "трейда" и
	// искажали winrate/среднюю сделку).
	realizedGross, realizedNet float64
	// partialFills - сколько раз уже сработал частичный тейк для этой
	// позиции. sales.Sales.PartialTakeProfit сама не хранит "уже сработало" -
	// это состояние здесь, и pos.partialFills>0 останавливает повторные
	// срабатывания (один уровень - максимум одно срабатывание на позицию).
	partialFills int
}

// limitOrder - лимитная заявка на вход в откат: сигнал уже был, входа ещё нет
type limitOrder struct {
	sig      signal.Signal
	side     order.SideType
	price    float64
	placedAt time.Time
	barsLeft int
	done     bool
}

// placeLimit выставляет заявку на EntryOffsetVol волатильностей ЛУЧШЕ закрытия
// сигнальной свечи: лонг покупает откат вниз, шорт - откат вверх.
func (e *Engine) placeLimit(sig signal.Signal, side order.SideType, bar exModel.Candle) *limitOrder {
	offset := e.opt.EntryOffsetVol * sig.Volatility / 100

	price := bar.Close * (1 - offset)
	if side == order.SideTypeSell {
		price = bar.Close * (1 + offset)
	}

	ttl := e.opt.EntryTTLBars
	if ttl < 1 {
		ttl = 1
	}

	return &limitOrder{
		sig:      sig,
		side:     side,
		price:    price,
		placedAt: bar.Time,
		barsLeft: ttl,
	}
}

// tryFill исполняет лимитку, если бар дотянулся до её цены.
//
// Риск-правила проверяются В МОМЕНТ ИСПОЛНЕНИЯ, а не выставления: между ними
// могла открыться позиция по другой заявке или исчерпаться общий лимит.
// Цена исполнения - цена заявки (лимитка не исполняется хуже своей цены;
// лучше - тоже не засчитываем, бэктест консервативен).
func (e *Engine) tryFill(
	ord *limitOrder,
	pair string,
	bar exModel.Candle,
	open map[string]*simPosition,
	lastClose map[string]time.Time,
	cooldown time.Duration,
	report *Report,
) {
	ord.barsLeft--

	touched := bar.Low <= ord.price
	if ord.side == order.SideTypeSell {
		touched = bar.High >= ord.price
	}
	if !touched {
		return
	}

	// Цена дошла, но входить уже нельзя - заявка снимается, а не ждёт дальше:
	// её цена посчитана от сигнальной свечи и с каждым баром устаревает.
	ord.done = true
	if _, busy := open[pair]; busy {
		report.Rejected["по паре уже есть открытая позиция"]++
		return
	}
	if len(open) >= e.entry.MaxPositions {
		report.Rejected["достигнут лимит одновременных позиций"]++
		return
	}
	if last, ok := lastClose[pair]; ok && bar.Time.Sub(last) < cooldown {
		report.Rejected["пара на cooldown после закрытия"]++
		return
	}

	takeProfit, stopLoss, hold := e.exits.Plan(ord.sig)

	position := sales.Position{
		Order: order.Order{
			Pair:         pair,
			Side:         ord.side,
			PriceCreated: ord.price,
		},
		Signal:            ord.sig,
		TakeProfitPercent: takeProfit,
		StopLossPercent:   stopLoss,
	}
	if hold > 0 {
		// Дедлайн отсчитывается от исполнения: план выхода живёт от входа,
		// а не от сигнала.
		position.Deadline = bar.Time.Add(hold)
	}

	open[pair] = &simPosition{position: position, openTime: bar.Time, remainingFraction: 1.0}
	report.LimitFilled++
}

// Run прогоняет свечи через стратегию и возвращает отчёт.
//
// Свечи - сигнального периода, по всем парам вперемешку (как отдаёт
// SelectCandlesFromPeriod). Порядок обработки строго по времени, внутри одного
// момента - по имени пары: результат детерминирован.
func (e *Engine) Run(candles []exModel.Candle) *Report {
	byPair := groupByPair(candles)

	pairs := make([]string, 0, len(byPair))
	for pair := range byPair {
		pairs = append(pairs, pair)
	}
	sort.Strings(pairs)

	timeline := buildTimeline(byPair)

	report := &Report{
		Period:   e.opt.Period,
		Pairs:    len(pairs),
		Rejected: make(map[string]int),
	}
	if len(timeline) > 0 {
		report.From = timeline[0]
		report.To = timeline[len(timeline)-1]
	}

	// Состояние симуляции - ровно те же риск-правила, что в Executor.riskAllows:
	// одна позиция на пару, общий лимит, cooldown пары после закрытия.
	open := make(map[string]*simPosition)
	pending := make(map[string]*limitOrder)
	lastClose := make(map[string]time.Time)
	cooldown := time.Duration(e.entry.PairCooldownMinutes) * time.Minute

	// Индекс текущей свечи по каждой паре
	cursor := make(map[string]int, len(pairs))

	// Сигнал такта: детекция уже прошла, вход ещё нет. Развести их по двум
	// проходам необходимо для фильтра режима: доля аномальных пар известна
	// только ПОСЛЕ детекции по всем парам, а входить надо с её учётом - иначе
	// пары в начале алфавита входили бы вслепую.
	type tickSignal struct {
		sig signal.Signal
		bar exModel.Candle
	}

	for _, t := range timeline {
		signals := make([]tickSignal, 0)
		activePairs := 0

		// ПРОХОД 1: выходы, исполнение лимиток, детекция.
		for _, pair := range pairs {
			series := byPair[pair]
			i := cursor[pair]
			if i >= len(series) || !series[i].Time.Equal(t) {
				continue
			}
			cursor[pair] = i + 1
			bar := series[i]
			activePairs++

			// Выходы: позиция живёт по правилам политики выхода независимо
			// от того, что детектор думает про эту свечу.
			if pos, ok := open[pair]; ok && bar.Time.After(pos.openTime) {
				// Частичный тейк - ДО полного выхода: если бар зацепил и
				// частичный уровень, и полный тейк/стоп, сначала фиксируем
				// часть по своей цене, потом уже проверяем финальный выход -
				// та же логика, что цена внутри бара идёт от частичного
				// уровня дальше, а не наоборот.
				e.tryPartialTakeProfit(pos, bar, report)

				if trade, closed := e.tryExit(pos, bar); closed {
					report.addTrade(trade)
					delete(open, pair)
					lastClose[pair] = bar.Time
				}
			}

			// Лимитная заявка: исполнилась, если бар дотянулся до её цены
			if order, ok := pending[pair]; ok && bar.Time.After(order.placedAt) {
				e.tryFill(order, pair, bar, open, lastClose, cooldown, report)
				if order.done || order.barsLeft <= 0 {
					if !order.done {
						report.LimitExpired++
					}
					delete(pending, pair)
				}
			}

			// Первая свеча пары - не с чем сравнивать
			if i == 0 {
				continue
			}
			prev := series[i-1]

			// Разрыв в данных: отношение через дырку - другой горизонт.
			// Ровно то же правило, что при сидировании боевой истории.
			gap := bar.Time.Sub(prev.Time)
			if gap < e.opt.PeriodDuration/2 || gap > e.opt.PeriodDuration*3/2 {
				continue
			}

			if sig, found := e.detector.DetectCandle(pair, e.opt.Period, prev, bar); found {
				report.Signals++
				signals = append(signals, tickSignal{sig: sig, bar: bar})
			}
		}

		// Фильтр режима: слишком много аномальных пар разом - это движение
		// рынка, а не события в отдельных парах. Локальная статистика пары
		// про обвал не знает; эта доля - знает.
		if e.opt.MaxAnomalousShare > 0 && activePairs > 0 {
			share := float64(len(signals)) / float64(activePairs)
			if share > e.opt.MaxAnomalousShare {
				report.Rejected["regime-filter"] += len(signals)
				continue
			}
		}

		// ПРОХОД 2: входы - через ТЕ ЖЕ правила, что в бою.
		for _, ts := range signals {
			pair := ts.sig.Pair

			side, reject, allowed := e.entry.SideFor(ts.sig)
			if !allowed {
				// Агрегируем по стабильному коду: Reason содержит подставленные
				// числа и дал бы по строке отчёта на каждый отказ.
				key := reject.Code
				if key == "" {
					key = reject.Reason
				}
				report.Rejected[key]++
				continue
			}
			if _, busy := open[pair]; busy {
				report.Rejected["по паре уже есть открытая позиция"]++
				continue
			}
			if len(open) >= e.entry.MaxPositions {
				report.Rejected["достигнут лимит одновременных позиций"]++
				continue
			}
			if last, ok := lastClose[pair]; ok && ts.bar.Time.Sub(last) < cooldown {
				report.Rejected["пара на cooldown после закрытия"]++
				continue
			}

			// Лимитный режим: вместо входа по рынку выставляем заявку в откат.
			// Свежий сигнал по паре заменяет уже висящую заявку - её цена
			// посчитана от устаревшей свечи.
			if e.opt.EntryOffsetVol > 0 && ts.sig.Volatility > 0 {
				pending[pair] = e.placeLimit(ts.sig, side, ts.bar)
				report.LimitPlaced++
				continue
			}

			open[pair] = e.openPosition(ts.sig, side, ts.bar)
		}
	}

	// Позиции, не закрытые к концу истории, закрываем по последней цене:
	// иначе отчёт молча выкинет худшие (долго висящие) сделки.
	for pair, pos := range open {
		series := byPair[pair]
		last := series[len(series)-1]
		report.addTrade(e.closeTrade(pos, last.Close, last.Time, "end-of-data"))
	}

	report.finalize()
	return report
}

func (e *Engine) openPosition(sig signal.Signal, side order.SideType, bar exModel.Candle) *simPosition {
	takeProfit, stopLoss, hold := e.exits.Plan(sig)

	// Вход по цене закрытия сигнальной свечи: в бою вход происходит в течение
	// минуты после детекции, закрытие бара - ближайший честный аналог.
	// Проскальзывание всегда ПРОТИВ нас: лонг входит дороже, шорт - дешевле.
	entryPrice := bar.Close
	if side == order.SideTypeBuy {
		entryPrice *= 1 + e.opt.SlippagePercent/100
	} else {
		entryPrice *= 1 - e.opt.SlippagePercent/100
	}

	position := sales.Position{
		Order: order.Order{
			Pair:         sig.Pair,
			Side:         side,
			PriceCreated: entryPrice,
		},
		Signal:            sig,
		TakeProfitPercent: takeProfit,
		StopLossPercent:   stopLoss,
	}
	if hold > 0 {
		position.Deadline = bar.Time.Add(hold)
	}

	return &simPosition{position: position, openTime: bar.Time, remainingFraction: 1.0}
}

// tryExit проверяет позицию на баре через боевой ShouldExit.
//
// Внутри бара порядок цен неизвестен, поэтому порядок проверок КОНСЕРВАТИВНЫЙ:
// сначала стоп по худшей цене бара, потом тейк по лучшей, потом таймаут по
// закрытию. Если бар зацепил и стоп и тейк - засчитывается стоп: бэктест должен
// занижать результат, а не завышать.
//
// Цена исполнения стопа/тейка - уровень порога, а не экстремум бара: мы
// выходим по своему уровню, а не по лучшей цене свечи.
func (e *Engine) tryExit(pos *simPosition, bar exModel.Candle) (Trade, bool) {
	worst, best := bar.Low, bar.High
	if pos.position.Order.Side == order.SideTypeSell {
		worst, best = bar.High, bar.Low
	}

	if reason, exit := e.exits.ShouldExit(worst, bar.Time, pos.position); exit && reason == sales.ExitStopLoss {
		return e.closeTrade(pos, e.stopPrice(pos), bar.Time, string(reason)), true
	}
	if reason, exit := e.exits.ShouldExit(best, bar.Time, pos.position); exit && reason == sales.ExitTakeProfit {
		return e.closeTrade(pos, e.takePrice(pos), bar.Time, string(reason)), true
	}
	if reason, exit := e.exits.ShouldExit(bar.Close, bar.Time, pos.position); exit {
		return e.closeTrade(pos, bar.Close, bar.Time, string(reason)), true
	}

	return Trade{}, false
}

// partialTakeProfitPrice переводит дистанцию из sales.Sales.PartialTakeProfit
// (% от входа) в абсолютную цену - симметрично stopPrice/takePrice.
func (e *Engine) partialTakeProfitPrice(pos *simPosition) (price, fraction float64, ok bool) {
	distancePercent, fraction, ok := e.exits.PartialTakeProfit(pos.position)
	if !ok {
		return 0, 0, false
	}

	entry := pos.position.Order.PriceCreated
	if pos.position.Order.Side == order.SideTypeSell {
		return entry * (1 - distancePercent/100), fraction, true
	}
	return entry * (1 + distancePercent/100), fraction, true
}

// tryPartialTakeProfit проверяет и, если пора, фиксирует частичное закрытие
// позиции - ОДИН раз за всю её жизнь (pos.partialFills>0 останавливает
// повторные срабатывания; у самого sales.Sales.PartialTakeProfit состояния
// нет, оно здесь). Не убирает позицию из open - в отличие от tryExit, здесь
// нет понятия "закрыта", только "стала меньше".
func (e *Engine) tryPartialTakeProfit(pos *simPosition, bar exModel.Candle, report *Report) {
	if pos.partialFills > 0 {
		return
	}

	price, fraction, ok := e.partialTakeProfitPrice(pos)
	if !ok || fraction <= 0 || fraction >= 1 {
		return
	}

	best := bar.High
	touched := best >= price
	if pos.position.Order.Side == order.SideTypeSell {
		best = bar.Low
		touched = best <= price
	}
	if !touched {
		return
	}

	entry := pos.position.Order.PriceCreated
	exitPrice := price
	if pos.position.Order.Side == order.SideTypeBuy {
		exitPrice *= 1 - e.opt.SlippagePercent/100
	} else {
		exitPrice *= 1 + e.opt.SlippagePercent/100
	}

	gross := 0.0
	if entry > 0 {
		gross = (exitPrice/entry)*100 - 100
		if pos.position.Order.Side == order.SideTypeSell {
			gross = -gross
		}
	}
	net := gross - 2*e.opt.FeePercent

	// Взвешиваем ПО ЗАКРЫВАЕМОЙ доле - складывается с финальным куском в
	// closeTrade (см. её комментарий).
	pos.realizedGross += gross * fraction
	pos.realizedNet += net * fraction
	pos.remainingFraction -= fraction
	pos.partialFills++
	report.PartialFills++
}

func (e *Engine) stopPrice(pos *simPosition) float64 {
	entry := pos.position.Order.PriceCreated
	if pos.position.Order.Side == order.SideTypeSell {
		return entry * (1 + pos.position.StopLossPercent/100)
	}
	return entry * (1 - pos.position.StopLossPercent/100)
}

func (e *Engine) takePrice(pos *simPosition) float64 {
	entry := pos.position.Order.PriceCreated
	if pos.position.Order.Side == order.SideTypeSell {
		return entry * (1 - pos.position.TakeProfitPercent/100)
	}
	return entry * (1 + pos.position.TakeProfitPercent/100)
}

// closeTrade закрывает ОСТАВШУЮСЯ долю позиции и сворачивает весь её путь
// (возможно, с 1+ частичными тейками до этого - см. tryPartialTakeProfit) в
// ОДНУ запись Trade. Финальный кусок взвешивается по pos.remainingFraction
// (1.0, если частичных тейков не было вовсе - тогда результат ровно такой
// же, как до появления частичных тейков), и складывается с уже реализованным
// раньше - иначе одна логическая позиция превратилась бы в отчёте в
// несколько "сделок" и искажала бы winrate/среднюю сделку (см. Report:
// "все проценты на сделку", а не на кусок сделки).
func (e *Engine) closeTrade(pos *simPosition, exitPrice float64, at time.Time, reason string) Trade {
	entry := pos.position.Order.PriceCreated

	// Проскальзывание выхода - тоже против нас
	if pos.position.Order.Side == order.SideTypeBuy {
		exitPrice *= 1 - e.opt.SlippagePercent/100
	} else {
		exitPrice *= 1 + e.opt.SlippagePercent/100
	}

	gross := 0.0
	if entry > 0 {
		gross = (exitPrice/entry)*100 - 100
		if pos.position.Order.Side == order.SideTypeSell {
			gross = -gross
		}
	}
	net := gross - 2*e.opt.FeePercent

	totalGross := pos.realizedGross + gross*pos.remainingFraction
	totalNet := pos.realizedNet + net*pos.remainingFraction

	return Trade{
		Pair:      pos.position.Order.Pair,
		Side:      string(pos.position.Order.Side),
		Level:     pos.position.Signal.Level,
		Direction: string(pos.position.Signal.Direction),
		OpenTime:  pos.openTime,
		CloseTime: at,
		Reason:    reason,
		GrossPct:  totalGross,
		NetPct:    totalNet,
	}
}

func groupByPair(candles []exModel.Candle) map[string][]exModel.Candle {
	byPair := make(map[string][]exModel.Candle)
	for _, c := range candles {
		byPair[c.Pair] = append(byPair[c.Pair], c)
	}
	for pair := range byPair {
		series := byPair[pair]
		sort.Slice(series, func(i, j int) bool { return series[i].Time.Before(series[j].Time) })
	}
	return byPair
}

// buildTimeline - отсортированные уникальные времена свечей всех пар
func buildTimeline(byPair map[string][]exModel.Candle) []time.Time {
	seen := make(map[time.Time]struct{})
	for _, series := range byPair {
		for _, c := range series {
			seen[c.Time] = struct{}{}
		}
	}
	timeline := make([]time.Time, 0, len(seen))
	for t := range seen {
		timeline = append(timeline, t)
	}
	sort.Slice(timeline, func(i, j int) bool { return timeline[i].Before(timeline[j]) })
	return timeline
}
