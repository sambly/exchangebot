package entrysetup

// GetVolatilityRegime - см. AssetsPrices.GetVolatilityRegime. Проксируется
// без изменений: расчёт и история живут в prices (там же, где GetVolatility),
// entrysetup - просто единая точка доступа ко всем показателям удобства
// входа, чтобы потребителю не нужно было знать, откуда какой берётся.
func (s *AssetsSetup) GetVolatilityRegime(pair, period string) (float64, bool) {
	if s.prices == nil {
		return 0, false
	}
	return s.prices.GetVolatilityRegime(pair, period)
}

// GetAllVolatilityRegime - GetVolatilityRegime сразу по всем отслеживаемым
// парам и периодам, для таблицы "по рынку целиком". Читает только уже
// накопленную в памяти историю (см. prices/volatility.go) - в отличие от
// GetAllPriceLevels, никаких обращений к БД здесь нет.
func (s *AssetsSetup) GetAllVolatilityRegime() map[string]map[string]float64 {
	if s.prices == nil {
		return make(map[string]map[string]float64)
	}

	result := make(map[string]map[string]float64, len(s.prices.Pairs))
	for _, pair := range s.prices.Pairs {
		byPeriod := make(map[string]float64, len(s.prices.Periods))
		for period := range s.prices.Periods {
			if regime, ok := s.GetVolatilityRegime(pair, period); ok {
				byPeriod[period] = regime
			}
		}
		result[pair] = byPeriod
	}
	return result
}
