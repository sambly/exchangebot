package prices

import (
	"context"
	"fmt"
	"main/database"
	"main/exchange"
	"testing"
)

func TestUpdateDelta(t *testing.T) {

	ctx := context.Background()

	binance, err := exchange.NewBinance(ctx)
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

	asetsPrices := &AsetsPrices{
		Pairs:       pairs,
		ChangeDelta: make(map[string]map[string][]*ChangeDelta),
		database:    db,
	}

	for _, pair := range pairs {

		if _, ok := asetsPrices.ChangePrices[pair]; !ok {
			asetsPrices.ChangeDelta[pair] = map[string][]*ChangeDelta{}
		}

	}

	asetsPrices.UpdateDelta()

	fmt.Println(asetsPrices.ChangeDelta["BTCUSDT"]["1d"])

}
