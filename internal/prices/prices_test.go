package prices

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/database"
)

var cfg = config.Database{
	Type:     "mysql",
	Name:     "datafeeder",
	Host:     "127.0.0.1",
	Port:     "3306",
	User:     "root",
	Password: "q1w2e3",
}

func TestUpdateDeltaFullData(t *testing.T) {

	ctx := context.Background()

	_ = flag.Set("test.timeout", "5m")

	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		t.Error(err)
	}
	pairs, err := binance.GetPairsToUSDT(ctx)
	if err != nil {
		t.Error(err)
	}

	db, err := database.DbInit(cfg)
	if err != nil {
		t.Error(err)
	}

	periods := map[string]time.Duration{
		"ch1m":  time.Second * 60,
		"ch3m":  time.Minute * 3,
		"ch15m": time.Minute * 15,
		"ch1h":  time.Hour,
		"ch4h":  time.Hour * 4,
		"ch12h": time.Hour * 12,
	}

	asetsPrices := NewAssetsPrices(pairs, periods, periods, db)

	_ = asetsPrices.UpdateChangeDelta()

}
