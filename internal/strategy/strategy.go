package strategy

import (
	"context"
	"fmt"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

type Strategy interface {
	OnMarket(ms exModel.MarketsStat)
	Start(ctx context.Context) error
}

type Option func(*ControllerStrategy)

type ControllerStrategy struct {
	Strategies []Strategy
}

func NewControllerStrategy(options ...Option) (*ControllerStrategy, error) {
	ctrlStr := &ControllerStrategy{}

	for _, option := range options {
		option(ctrlStr)
	}

	return ctrlStr, nil
}

func (cs *ControllerStrategy) WithStrategy(strategy Strategy) *ControllerStrategy {
	cs.Strategies = append(cs.Strategies, strategy)
	return cs
}

func (cs *ControllerStrategy) StartAll(ctx context.Context) error {
	for _, strategy := range cs.Strategies {
		if err := strategy.Start(ctx); err != nil {
			return fmt.Errorf("failed to start strategy: %w", err)
		}
	}
	return nil
}
