package entrysetup

import "github.com/sambly/exchangebot/internal/order"

// StrengthComponents - сырые составляющие "скрытой силы" пары: набор
// независимых наблюдений (order flow imbalance, уровни, режим волатильности,
// объёмная buy-активность), НЕ сведённых в одно число. Свод в композит и
// выбор, какие компоненты учитывать, - решение потребителя (веб-таблица с
// чекбоксами), не этого пакета: тот же принцип "сырое наблюдение / решение",
// что у Walls и Quality.
//
// Стены сюда сознательно НЕ включены (были раньше, убраны в рамках
// strategy_v2 - см. ARCHITECTURE.md §10.3): направленный сигнал по резервному
// объёму на уровне стакана легко подделывается спуфингом, а confirm-стрик
// защищает только от однотактного шума, не от целенаправленной подмены
// ордера. Сам примитив (Walls/Quality) никуда не делся - используется
// structsale для структурного стоп/тейка, где это не "куда идёт рынок", а
// "где физическое препятствие", и спуфинг там менее критичен.
type StrengthComponents struct {
	Pair   string
	Period string

	// HasOFI/OFIZScore - см. depth.GetOFIZScore (Order Flow Imbalance,
	// Cont-Kukanov-Stoikov). Заменил снапшот-имбаланс в этой таблице: OFI
	// событийный (копит вклад каждого изменения топа стакана), снапшот
	// переписывается одной перевыставленной заявкой. Знак - сторона:
	// положительный - перевес покупателей.
	HasOFI    bool
	OFIZScore float64
	// OFIConfirmed - см. depth.GetOFIConfirmedSide: сторона OFI держится уже
	// несколько закоммиченных окон подряд, а не мелькнула в последнем.
	// Проверяется, что подтверждённая сторона совпадает с ТЕКУЩИМ знаком
	// OFIZScore - если сторона только что развернулась, подтверждение от
	// предыдущей стороны не засчитывается (см. GetAllStrengthComponents).
	OFIConfirmed bool

	// HasLevels/LevelsSide/LevelsScore - тот же принцип, что у Quality, но по
	// PriceLevels (свечным разворотам) вместо стакана.
	HasLevels   bool
	LevelsSide  order.SideType
	LevelsScore float64

	// HasRegime/Regime - см. AssetsPrices.GetVolatilityRegime. Без стороны:
	// <1 - сжатие, само по себе не bullish и не bearish, а "сейчас интереснее
	// смотреть" - множитель для остальных компонентов, а не голосующий сигнал.
	HasRegime bool
	Regime    float64

	// HasVPIN/VPINZScore - см. AssetsPrices.GetVPINZScore (объёмно-
	// бакетированная доля покупок, VPIN-подобный). Заменил времяbucketed
	// AssetsPrices.GetBuyActivityZScore в этой таблице: тот же смысл (перевес
	// покупательской активности), но бакет копится по набранному объёму, а не
	// по часам - на всплеске закрывается быстро, в затишье медленно.
	HasVPIN    bool
	VPINZScore float64
	// VPINConfirmed - см. AssetsPrices.GetVPINConfirmedSide, та же сверка с
	// текущим знаком VPINZScore, что и у OFIConfirmed.
	VPINConfirmed bool

	// HasRelativeStrength/RelativeStrengthZScore - см.
	// AssetsPrices.GetRelativeStrengthZScore. В отличие от всех остальных
	// компонентов (смотрят на пару САМУ ПО СЕБЕ во времени), это
	// КРОСС-СЕКЦИОННЫЙ сигнал: пара относительно МЕДИАНЫ рынка за тот же
	// период - отличает идиосинкратическую аномалию пары от бета-движения
	// всего рынка (когда всё падает вслед за BTC). Единственный из
	// компонентов, зависящий от period так же, как остальные ПРЯМО ПО СМЫСЛУ
	// (не только через кэш) - "рынок" за 15m и за 4h - разные сравнения.
	HasRelativeStrength    bool
	RelativeStrengthZScore float64
	// RelativeStrengthConfirmed - та же сверка с текущим знаком, что и у
	// OFIConfirmed/VPINConfirmed.
	RelativeStrengthConfirmed bool
}

// LevelsSideScore - тот же принцип "ближе - в стоп, дальше - в тейк", что и у
// qualityFromWalls (score.go), только по PriceLevels вместо стакана: PriceLevels
// сам не даёт готовую сторону/score, только Support/Resistance с дистанциями.
// Экспортирован - используется не только сводом композита "Потенциал" здесь,
// но и стратегией-детектором на swing-уровнях (internal/strategy/levels).
func LevelsSideScore(levels PriceLevels) (order.SideType, float64, bool) {
	if !levels.HasSupport || !levels.HasResistance {
		return "", 0, false
	}

	stop, take := levels.Support.DistancePercent, levels.Resistance.DistancePercent
	side := order.SideTypeBuy
	if stop > take {
		stop, take = take, stop
		side = order.SideTypeSell
	}
	if stop <= 0 {
		return "", 0, false
	}
	return side, take / stop, true
}

// GetAllStrengthComponents - все составляющие "силы" по всем отслеживаемым
// парам и периодам, для новой таблицы-скринера. OFI от периода не зависит -
// забирается один раз на пару (та же оптимизация, что у GetAllQuality).
// VPIN тоже не зависит от периода (один трекер на пару, см. prices/vpin.go).
// PriceLevels берётся из GetAllPriceLevels (её собственный кэш на
// priceLevelsCacheTTL, см. levels.go) - НЕ вызовом GetPriceLevels (pair,
// period) внутри цикла по парам: у GetPriceLevels свой поход в БД на КАЖДЫЙ
// вызов, и дёргать его в цикле по парам значило бы N раз тянуть из БД один и
// тот же набор свечей на период, выбрасывая из него все пары кроме одной -
// ровно та ошибка, от которой предостерегает комментарий у
// GetAllPriceLevels (когда-то она уже закралась сюда - см. историю правок).
// Regime дешёвый (читает уже накопленную в памяти историю) - его можно звать
// сколько угодно раз, лишнего похода в БД он не делает.
func (s *AssetsSetup) GetAllStrengthComponents() map[string]map[string]StrengthComponents {
	result := make(map[string]map[string]StrengthComponents)
	if s.prices == nil {
		return result
	}

	for _, pair := range s.prices.Pairs {
		hasOFI := false
		ofiZ := 0.0
		ofiConfirmed := false
		if s.depth != nil {
			if _, z, ok := s.depth.GetOFIZScore(pair); ok {
				hasOFI, ofiZ = true, z
			}
			// Подтверждённая сторона должна совпадать с ТЕКУЩИМ знаком OFI:
			// коммит окна троттлится (см. ofiWindowDuration), и если сторона
			// только что развернулась, GetOFIConfirmedSide ещё может
			// отдавать стрик от ПРЕДЫДУЩЕЙ, уже неактуальной стороны - без
			// сверки композит принял бы устаревшее подтверждение за
			// актуальное.
			if confirmedSide, ok := s.depth.GetOFIConfirmedSide(pair); ok {
				currentSide := order.SideTypeBuy
				if ofiZ < 0 {
					currentSide = order.SideTypeSell
				}
				ofiConfirmed = confirmedSide == currentSide
			}
		}

		hasVPIN := false
		vpinZ := 0.0
		vpinConfirmed := false
		if _, z, ok := s.prices.GetVPINZScore(pair); ok {
			hasVPIN, vpinZ = true, z
		}
		// Та же сверка с текущим знаком, что и у OFI выше - см. комментарий
		// там же.
		if confirmedSide, ok := s.prices.GetVPINConfirmedSide(pair); ok {
			currentSide := "BUY"
			if vpinZ < 0 {
				currentSide = "SELL"
			}
			vpinConfirmed = confirmedSide == currentSide
		}

		byPeriod := make(map[string]StrengthComponents, len(s.prices.Periods))
		for period := range s.prices.Periods {
			c := StrengthComponents{
				Pair:          pair,
				Period:        period,
				HasOFI:        hasOFI,
				OFIZScore:     ofiZ,
				OFIConfirmed:  ofiConfirmed,
				HasVPIN:       hasVPIN,
				VPINZScore:    vpinZ,
				VPINConfirmed: vpinConfirmed,
			}

			if regime, ok := s.GetVolatilityRegime(pair, period); ok {
				c.HasRegime, c.Regime = true, regime
			}

			// В отличие от OFI/VPIN (посчитаны один раз выше, не зависят от
			// периода), относительная сила ЗАВИСИТ от периода по смыслу -
			// сравнение с рынком за 15m и за 4h разное, поэтому считается
			// внутри цикла по периодам.
			if _, z, ok := s.prices.GetRelativeStrengthZScore(pair, period); ok {
				c.HasRelativeStrength, c.RelativeStrengthZScore = true, z
				// Та же сверка с текущим знаком, что и у OFI/VPIN выше - см.
				// комментарий там же.
				if confirmedSide, ok := s.prices.GetRelativeStrengthConfirmedSide(pair, period); ok {
					currentSide := "BUY"
					if z < 0 {
						currentSide = "SELL"
					}
					c.RelativeStrengthConfirmed = confirmedSide == currentSide
				}
			}

			byPeriod[period] = c
		}
		result[pair] = byPeriod
	}

	s.fillStrengthLevels(result)

	return result
}

// fillStrengthLevels - PriceLevels для GetAllStrengthComponents, из уже
// закэшированного GetAllPriceLevels (см. levels.go) - без своего похода в БД.
func (s *AssetsSetup) fillStrengthLevels(result map[string]map[string]StrengthComponents) {
	for pair, byPeriod := range s.GetAllPriceLevels() {
		if _, tracked := result[pair]; !tracked {
			continue
		}
		for period, levels := range byPeriod {
			side, score, ok := LevelsSideScore(levels)
			if !ok {
				continue
			}

			c := result[pair][period]
			c.HasLevels, c.LevelsSide, c.LevelsScore = true, side, score
			result[pair][period] = c
		}
	}
}
