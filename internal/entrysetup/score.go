package entrysetup

import "github.com/sambly/exchangebot/internal/order"

// Quality - оценка удобства входа по паре, сведённая к одному числу: во
// сколько раз дальняя стена дальше ближней.
//
// БЛИЖНЯЯ стена (по ходу от текущей цены) - естественный кандидат в стоп: она
// же и держит цену, и даёт ориентир, где выставлять защиту. ДАЛЬНЯЯ - кандидат
// в тейк: до неё есть простор для движения. Сторона сделки следует из того,
// какая стена ближе: если ближе стена снизу (support), тесный стоп там - у
// лонга; если ближе стена сверху (resistance) - у шорта.
//
// Score = TakeDistancePercent / StopDistancePercent. Это НЕ рекомендация
// "входить" и не готовый план сделки (стоп/тейк для реальной позиции всё ещё
// считает simplesale.Plan по волатильности) - чисто наблюдение "по структуре
// стакана прямо сейчас соотношение такое-то". Абсолютные дистанции отданы
// рядом с Score намеренно: соотношение 5:1 при стопе в 8% - формально
// "отличный" сетап, который на деле бесполезен, и решать, что считать разумной
// дистанцией, - дело потребителя (веб, стратегия), а не этого пакета.
//
// ВАЖНО про Score и период: Score - это отношение двух дистанций, и period на
// него НЕ влияет - деление обеих на одну и ту же волатильность сократилось бы.
// От периода зависит другое: подходят ли АБСОЛЮТНЫЕ дистанции под горизонт
// сделки. Для этого есть *Sigma-поля: та же дистанция, но в единицах типичного
// движения пары ЗА period (AssetsPrices.GetVolatility). 0.3 сигмы - стена почти
// на цене, шум пробьёт её за минуты вне зависимости от Score; 5 сигм - для
// этого периода недостижимо далеко.
type Quality struct {
	Pair   string
	Period string

	// Side - сторона, которую подсказывает асимметрия стен: BUY, если ближе
	// стена снизу (support), SELL - если ближе стена сверху (resistance).
	Side order.SideType

	// StopDistancePercent - расстояние до БЛИЖНЕЙ стены (кандидат в стоп)
	StopDistancePercent float64
	// TakeDistancePercent - расстояние до ДАЛЬНЕЙ стены (кандидат в тейк)
	TakeDistancePercent float64
	// Score - TakeDistancePercent / StopDistancePercent. Больше - выгоднее
	// потенциальное соотношение прибыль/риск по структуре стакана. От period
	// не зависит, см. комментарий у типа.
	Score float64

	// StopDistanceSigma/TakeDistanceSigma - те же дистанции в единицах
	// волатильности пары за period. HasVolatility=false, пока по паре+периоду
	// не накопилось истории (см. AssetsPrices.GetVolatility) - тогда оба поля
	// нулевые и использовать их нельзя.
	StopDistanceSigma float64
	TakeDistanceSigma float64
	HasVolatility     bool
}

// GetQuality сводит Walls к одному числу через сторону, где стена ближе, и
// добавляет привязку к горизонту сделки через волатильность пары за period.
// false, если хотя бы одной из стен сейчас нет (см. Walls.HasSupport/
// HasResistance) - соотношение без обеих сторон не посчитать честно.
func (s *AssetsSetup) GetQuality(pair, period string) (Quality, bool) {
	walls, ok := s.GetWalls(pair)
	if !ok {
		return Quality{}, false
	}
	return s.qualityFromWalls(pair, period, walls)
}

// qualityFromWalls - GetQuality без повторного похода в стакан: GetWalls не
// зависит от period (это снапшот стакана прямо сейчас), а GetAllQuality
// считает Quality для НЕСКОЛЬКИХ периодов одной пары - без этого разделения
// каждый период заново пересортировывал бы один и тот же стакан.
func (s *AssetsSetup) qualityFromWalls(pair, period string, walls Walls) (Quality, bool) {
	if !walls.HasSupport || !walls.HasResistance {
		return Quality{}, false
	}

	stop, take := walls.Support.DistancePercent, walls.Resistance.DistancePercent
	side := order.SideTypeBuy
	if stop > take {
		stop, take = take, stop
		side = order.SideTypeSell
	}

	if stop <= 0 {
		return Quality{}, false
	}

	q := Quality{
		Pair:                pair,
		Period:              period,
		Side:                side,
		StopDistancePercent: stop,
		TakeDistancePercent: take,
		Score:               take / stop,
	}

	if s.prices != nil {
		if vol, ok := s.prices.GetVolatility(pair, period); ok && vol > 0 {
			q.StopDistanceSigma = stop / vol
			q.TakeDistanceSigma = take / vol
			q.HasVolatility = true
		}
	}

	return q, true
}

// GetAllQuality - GetQuality сразу по всем отслеживаемым парам и периодам,
// для таблицы "по рынку целиком" (см. depth.GetAllImbalanceZScore). Пары
// берутся из depth (там же живут стены), периоды - из prices.Periods.
// Пара, для которой сейчас нет обеих стен, попадает в результат с пустой
// картой периодов, а не выпадает совсем - потребитель видит все пары
// одинаково, просто часть значений отсутствует.
func (s *AssetsSetup) GetAllQuality() map[string]map[string]Quality {
	result := make(map[string]map[string]Quality)
	if s.depth == nil || s.prices == nil {
		return result
	}

	for _, pair := range s.depth.Pairs {
		byPeriod := make(map[string]Quality, len(s.prices.Periods))

		if walls, ok := s.GetWalls(pair); ok {
			for period := range s.prices.Periods {
				if q, ok := s.qualityFromWalls(pair, period, walls); ok {
					byPeriod[period] = q
				}
			}
		}

		result[pair] = byPeriod
	}

	return result
}
