package anomaly

import (
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

// Этот файл - вся поверхность, которой anomaly повёрнута к бэктестеру.
//
// Ядро детектора (detect) одно на бой и на бэктест - см. metricSource. Здесь
// только конструктор без внешнего мира и прогон одной свечи. Если этот файл
// растёт - значит, в него утекает логика детекции, и её надо возвращать в detect.

// NewBacktestDetector - детектор для прогона по истории.
//
// Отличия от боевого NewStrategy - только в подключении к миру:
//   - нет AssetsPrices и уведомлений: свечи приносит бэктестер;
//   - нет сидирования: историю набирает сам прогон, свеча за свечой, - в этом
//     и смысл walk-forward, никакого знания будущего;
//   - confirmChecks принудительно = 1: в бою это счётчик МИНУТНЫХ проверок
//     скользящего окна, а бэктест видит одну точку на период. Требовать
//     "2 свечи подряд" вместо "2 минуты подряд" - это другое, куда более
//     жёсткое правило, и оно бы тихо исказило результат.
//
// Конфиг читается тот же (config.yaml): бэктест проверяет ту стратегию,
// которая реально торгует, а не её вариант с ручными параметрами.
func NewBacktestDetector(periods map[string]time.Duration, pairs []string) (*AnomalyStrategy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	cfg.Pairs = pairs

	for _, pc := range cfg.Periods {
		pc.ConfirmChecks = 1
	}

	str := &AnomalyStrategy{
		Config:       cfg,
		Periods:      periods,
		history:      make(map[string]map[string]map[string]*MetricRecord),
		states:       make(map[string]map[string]*PeriodState),
		marketStates: make(map[string]*PeriodState),
	}

	for period := range periods {
		str.marketStates[period] = &PeriodState{}
	}
	for _, pair := range pairs {
		str.history[pair] = make(map[string]map[string]*MetricRecord)
		str.states[pair] = make(map[string]*PeriodState)
		for period := range periods {
			str.history[pair][period] = make(map[string]*MetricRecord)
			str.states[pair][period] = &PeriodState{}
			str.initMetricsForPair(pair, period, str.windowSizeForPeriod(period))
		}
	}

	return str, nil
}

// DetectCandle прогоняет очередную ЗАКРЫТУЮ свечу пары через ядро детектора.
//
// prev и current - соседние свечи периода: их отношение и есть одна выборка
// метрик, на том же тождестве построено сидирование боевой истории из БД.
// Свеча всегда коммитится в историю (в бэктесте одна свеча = один период, то
// есть троттлинг nextSampleAt здесь выражается сам собой).
func (s *AnomalyStrategy) DetectCandle(pair, period string, prev, current exModel.Candle) (signal.Signal, bool) {
	periodCfg, ok := s.Config.Periods[period]
	if !ok || !periodCfg.Enabled {
		return signal.Signal{}, false
	}
	if _, tracked := s.history[pair]; !tracked {
		return signal.Signal{}, false
	}

	result := s.detect(pair, period, &periodCfg.Thresholds, true, candleSource{prev: prev, current: current})
	if result == nil || !result.IsAnomalous || !result.HasPrice {
		return signal.Signal{}, false
	}

	sig := s.signalFrom(result)
	// Время сигнала - момент свечи, а не время запуска бэктеста
	sig.Time = current.Time
	return sig, true
}
