package simplebuy

import (
	"time"

	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
)

type StrategySimpleBuy struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *StrategySimpleBuyMenu

	Periods      map[string]time.Duration
	AssetsPrices *prices.AsetsPrices
}

func NewStrategy(notify *notification.Notification) (*StrategySimpleBuy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	str := &StrategySimpleBuy{
		Config:       cfg,
		Notification: notify,
	}
	return str, nil
}

func (s *StrategySimpleBuy) WithTelegramMenu() *StrategySimpleBuy {
	tlgMenu := NewStrategyMenu("Base стратегия", "strategiesBase", s)
	s.TelegramMenu = tlgMenu
	return s
}
