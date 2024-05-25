package prices

import (
	"context"
	"flag"
	"main/internal/config"
	"main/internal/database"
	"main/internal/exchange"
	"testing"
	"time"
)

func TestUpdateDeltaFullData(t *testing.T) {

	flag.Set("test.timeout", "5m")

	binance, err := exchange.NewBinance(context.Background())
	if err != nil {
		t.Error(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		t.Error(err)
	}

	config, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}

	db, err := database.DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	periods := map[string]time.Duration{
		"ch1m":  time.Second * 60,
		"ch3m":  time.Minute * 3,
		"ch15m": time.Minute * 15,
		"ch1h":  time.Hour,
		"ch4h":  time.Hour * 4,
		"ch12h": time.Hour * 12,
	}

	asetsPrices := NewAssetsPrices(pairs, periods, periods, nil, db, nil)

	asetsPrices.UpdateDelta()

}
