// Поиск локальных максимумов минимумов в разные периоды

package strategy

import (
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

type LocalExtremes struct {
	Pairs    []string
	Extremes map[string]map[string]*Data
}

type Data struct {
	Max float64
	Min float64
}

func NewLocalExtremes(pairs []string, periodsChange map[string]time.Duration) *LocalExtremes {

	localExtremes := &LocalExtremes{
		Pairs:    pairs,
		Extremes: make(map[string]map[string]*Data),
	}

	for _, pair := range pairs {
		localExtremes.Extremes[pair] = map[string]*Data{}

		for period := range periodsChange {
			localExtremes.Extremes[pair][period] = &Data{}
		}
	}

	return localExtremes
}

func (ext *LocalExtremes) OnMarket(ms exModel.MarketsStat) {

}

func (ext *LocalExtremes) Start() {

}

// func (le *LocalExtremes) AddData(pair Pair, period Period, max float64, min float64) {
// 	if _, exists := le.Pair[pair]; !exists {
// 		le.Pair[pair] = make(map[Period]*Data)

// 		le.Pairs = append(le.Pairs, pair)
// 	}

// 	if _, exists := le.Pair[pair][period]; !exists {
// 		le.Pair[pair][period] = &Data{
// 			Max: max,
// 			Min: min,
// 		}
// 	}
// }
