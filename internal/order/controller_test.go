package order

// func TestCommon(t *testing.T) {

// 	ctx := context.Background()

// 	db, err := database.DbConnection()
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	defer db.Close()

// 	err = database.CreateOrdersTable(db)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	mStat := make(map[string]*model.MarketsStat)
// 	mStat["BTCUSDT"] = &model.MarketsStat{Time: time.Now(), Price: 10}

// 	paperWallet := exchange.NewPaperWallet(ctx)
// 	paperWallet.MarketsStat = mStat

// 	orderController, err := NewController(ctx, paperWallet, db)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	order, err := orderController.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1.0)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	mStat["BTCUSDT"] = &model.MarketsStat{Time: time.Now().Add(time.Minute), Price: 40}

// 	err = orderController.ClosePosition(order.ID)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	_ = order
// }
