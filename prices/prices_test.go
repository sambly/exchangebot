package prices

import (
	"context"
	"flag"
	"fmt"
	"main/config"
	"main/database"
	"main/exchange"
	"main/model"
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

	// binance, err := exchange.NewBinance(context.Background())
	// if err != nil {
	// 	t.Error(err)
	// }
	// pairs, err := binance.GetPairsToUSDT()
	// if err != nil {
	// 	t.Error(err)
	// }

	//pairs = []string{"BTCUSDT"}

	//changePeriods := []string{"ch1m", "ch3m", "ch15m", "ch1h", "ch4h"}
	//changePeriods := []string{"ch1m"}

	//asetsPrices := NewAssetsPrices(pairs, changePeriods, nil, 0, db, nil)

	periods := map[string]time.Duration{
		"ch1m":  time.Second * 60,
		"ch3m":  time.Minute * 3,
		"ch15m": time.Minute * 15,
		"ch1h":  time.Hour,
		"ch4h":  time.Hour * 4,
	}

	timeNow := time.Now()
	timeRounding := timeNow.Truncate(60 * time.Second)

	// Определить масимальное время из периода для запроса в бд
	var max time.Duration
	for _, dur := range periods {
		if dur > max {
			max = dur
		}
	}

	timeRoundingMax := timeRounding.Add(-max)

	candlesList, err := database.SelectMarketStateTimev2(db, timeRoundingMax)
	if err != nil {
		t.Error(err)
	}

	var candles = map[string]map[string]*model.ChangeDataFull{}

	for _, candle := range candlesList {
		for period, periodValue := range periods {

			if _, ok := candles[candle.Pair]; !ok {
				candles[candle.Pair] = map[string]*model.ChangeDataFull{}
			}
			if _, ok := candles[candle.Pair][period]; !ok {
				candles[candle.Pair][period] = &model.ChangeDataFull{}
			}

			item := candles[candle.Pair][period]

			if isTimeMultipleOfInterval1(candle.Time, periodValue) {
				item.DatasetCandle = append(item.DatasetCandle, model.DatasetCandle{Price: candle.Close, Volume: candle.Volume})
				item.LastPrice = candle.Close
				item.LastVolume = candle.Volume
				item.Fill = true
			} else {
				item.DatasetCandle = append(item.DatasetCandle, model.DatasetCandle{Price: candle.Close, Volume: candle.Volume})
			}

		}
	}
	fmt.Println("LOL")

}

// Проверка на кратность времени
func isTimeMultipleOfInterval1(t time.Time, interval time.Duration) bool {
	startTime := time.Unix(0, 0) // Начальное время (начало Unix эпохи)
	return t.Sub(startTime)%interval == 0
}
