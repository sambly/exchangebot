package entrysetup

import "github.com/sambly/exchangebot/internal/order"

// StrengthComponents - сырые составляющие "скрытой силы" пары: набор
// независимых наблюдений (книга заявок, стены, уровни, режим волатильности,
// buy-активность), НЕ сведённых в одно число. Свод в композит и выбор, какие
// компоненты учитывать, - решение потребителя (веб-таблица с чекбоксами), не
// этого пакета: тот же принцип "сырое наблюдение / решение", что у Walls и
// Quality.
type StrengthComponents struct {
	Pair   string
	Period string

	// HasImbalance/ImbalanceZScore - см. depth.GetImbalanceZScore. Знак задаёт
	// сторону: положительный - перевес покупателей.
	HasImbalance    bool
	ImbalanceZScore float64
	// ImbalanceConfirmed - см. depth.GetImbalanceConfirmedSide: сторона
	// имбаланса держится уже некоторое время, а не мелькнула на одном сэмпле.
	// Проверяется, что подтверждённая сторона совпадает с ТЕКУЩИМ знаком
	// ImbalanceZScore - если сторона только что развернулась, подтверждение
	// от предыдущей стороны не засчитывается (см. GetAllStrengthComponents).
	ImbalanceConfirmed bool

	// HasWalls/WallsSide/WallsScore - см. Quality (стены стакана).
	HasWalls   bool
	WallsSide  order.SideType
	WallsScore float64
	// WallsConfirmed - см. GetWallsConfirmedSide, та же логика сверки со
	// стороной, что и у ImbalanceConfirmed.
	WallsConfirmed bool

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

	// HasActivity/ActivityZScore - см. AssetsPrices.GetBuyActivityZScore.
	HasActivity    bool
	ActivityZScore float64
}

// levelsSideScore - тот же принцип "ближе - в стоп, дальше - в тейк", что и у
// qualityFromWalls (score.go), только по PriceLevels вместо стакана: PriceLevels
// сам не даёт готовую сторону/score, только Support/Resistance с дистанциями.
func levelsSideScore(levels PriceLevels) (order.SideType, float64, bool) {
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
// парам и периодам, для новой таблицы-скринера. Walls/Imbalance от периода не
// зависят - забираются один раз на пару (та же оптимизация, что у
// GetAllQuality). PriceLevels берётся из GetAllPriceLevels (её собственный
// кэш на priceLevelsCacheTTL, см. levels.go) - НЕ вызовом GetPriceLevels
// (pair, period) внутри цикла по парам: у GetPriceLevels свой поход в БД на
// КАЖДЫЙ вызов, и дёргать его в цикле по парам значило бы N раз тянуть из БД
// один и тот же набор свечей на период, выбрасывая из него все пары кроме
// одной - ровно та ошибка, от которой предостерегает комментарий у
// GetAllPriceLevels (когда-то она уже закралась сюда - см. историю правок).
// Regime и Activity дешёвые (читают уже накопленную в памяти историю) - их
// можно звать сколько угодно раз, лишнего похода в БД они не делают.
func (s *AssetsSetup) GetAllStrengthComponents() map[string]map[string]StrengthComponents {
	result := make(map[string]map[string]StrengthComponents)
	if s.prices == nil {
		return result
	}

	for _, pair := range s.prices.Pairs {
		hasImbalance := false
		imbalanceZ := 0.0
		imbalanceConfirmed := false
		if s.depth != nil {
			if _, z, ok := s.depth.GetImbalanceZScore(pair); ok {
				hasImbalance, imbalanceZ = true, z
			}
			// Подтверждённая сторона должна совпадать с ТЕКУЩИМ знаком
			// имбаланса: сэмплирование в depth троттлится (см.
			// imbalanceSampleInterval), и если сторона только что
			// развернулась, GetImbalanceConfirmedSide ещё может отдавать
			// стрик от ПРЕДЫДУЩЕЙ, уже неактуальной стороны - без сверки
			// композит принял бы устаревшее подтверждение за актуальное.
			if confirmedSide, ok := s.depth.GetImbalanceConfirmedSide(pair); ok {
				currentSide := order.SideTypeBuy
				if imbalanceZ < 0 {
					currentSide = order.SideTypeSell
				}
				imbalanceConfirmed = confirmedSide == currentSide
			}
		}

		walls, hasWalls := s.GetWalls(pair)
		s.sampleWallsConfirm(pair, walls)

		byPeriod := make(map[string]StrengthComponents, len(s.prices.Periods))
		for period := range s.prices.Periods {
			c := StrengthComponents{
				Pair:               pair,
				Period:             period,
				HasImbalance:       hasImbalance,
				ImbalanceZScore:    imbalanceZ,
				ImbalanceConfirmed: imbalanceConfirmed,
			}

			if hasWalls {
				if q, ok := s.qualityFromWalls(pair, period, walls); ok {
					c.HasWalls, c.WallsSide, c.WallsScore = true, q.Side, q.Score
					if confirmedSide, ok := s.GetWallsConfirmedSide(pair); ok {
						c.WallsConfirmed = confirmedSide == q.Side
					}
				}
			}

			if regime, ok := s.GetVolatilityRegime(pair, period); ok {
				c.HasRegime, c.Regime = true, regime
			}

			if z, ok := s.prices.GetBuyActivityZScore(pair, period); ok {
				c.HasActivity, c.ActivityZScore = true, z
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
			side, score, ok := levelsSideScore(levels)
			if !ok {
				continue
			}

			c := result[pair][period]
			c.HasLevels, c.LevelsSide, c.LevelsScore = true, side, score
			result[pair][period] = c
		}
	}
}
