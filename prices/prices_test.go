package prices

import (
	"context"
	"flag"
	"main/database"
	"main/exchange"
	"testing"
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

	db, err := database.DbConnection()
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	asetsPrices := NewAssetsPrices(pairs, []string{}, nil, 0, db, nil)

	asetsPrices.UpdateDelta()

	//fmt.Println(asetsPrices.ChangeDelta["BTCUSDT"]["1d"])

}
