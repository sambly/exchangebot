package strategy

import exModel "github.com/sambly/exchangeService/pkg/model"

type Strategy interface {
	OnMarket(ms exModel.MarketsStat)
	Start()
}

type Option func(*ControllerStrategy)

type ControllerStrategy struct {
	Strategy      []Strategy
	LocalExtremes *LocalExtremes
}

func NewControllerStrategy(options ...Option) (*ControllerStrategy, error) {

	ctrlStr := &ControllerStrategy{}

	for _, option := range options {
		option(ctrlStr)
	}

	return ctrlStr, nil
}

func WithLocalExtremes(strategy *LocalExtremes) Option {
	return func(ctrlStr *ControllerStrategy) {
		ctrlStr.LocalExtremes = strategy
		ctrlStr.Strategy = append(ctrlStr.Strategy, strategy)
	}
}

func (ctrStr *ControllerStrategy) OnMarket(ms exModel.MarketsStat) {

}
