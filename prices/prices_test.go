package prices

import (
	"context"
	"flag"
	"main/config"
	"main/database"
	"main/exchange"
	"testing"
	"time"
)

func TestUpdateDelta(t *testing.T) {

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

	db, err := database.DbConnection(config.NameDb, config.HostNameDb, config.UserNameDb, config.PasswordDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	asetsPrices := NewAssetsPrices(pairs, []string{}, nil, 0, db, nil)

	asetsPrices.UpdateDelta()

}

func TestUpdateChanges(t *testing.T) {

	flag.Set("test.timeout", "5m")
	config, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}

	db, err := database.DbConnection(config.NameDb, config.HostNameDb, config.UserNameDb, config.PasswordDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	binance, err := exchange.NewBinance(context.Background())
	if err != nil {
		t.Error(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		t.Error(err)
	}

	pairs = []string{"BTCUSDT"}

	//changePeriods := []string{"ch1m", "ch3m", "ch15m", "ch1h", "ch4h"}
	changePeriods := []string{"ch1m"}

	asetsPrices := NewAssetsPrices(pairs, changePeriods, nil, 0, db, nil)

	// periods := map[string]time.Duration{
	// 	"ch1m":  time.Second * 60,
	// 	"ch3m":  time.Minute * 3,
	// 	"ch15m": time.Minute * 15,
	// 	"ch1h":  time.Hour,
	// 	"ch4h":  time.Hour * 4,
	// }

	periods := map[string]time.Duration{
		"ch1m": time.Second * 60,
	}

	timeNow := time.Now()
	timeRounding := timeNow.Truncate(60 * time.Second)

	for _, pair := range pairs {
		for period, timePeriod := range periods {

			changeData := asetsPrices.ChangePrices[pair][period]

			timeRoundingFormating := timeRounding.Add(-1 * timePeriod)
			candle, err := database.SelectMarketStateTime(db, pair, timeRoundingFormating)
			if err != nil {
				t.Error(err)
			}

			// fmt.Println(pair, candle.Close, candle.Volume, period)
			changeData.LastPrice = candle.Close
			changeData.LastPrice = candle.Volume

		}

	}

}
