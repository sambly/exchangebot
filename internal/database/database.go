package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sambly/exchangeService/pkg/model"
	exModel "github.com/sambly/exchangeService/pkg/model"

	_ "github.com/go-sql-driver/mysql" // init MySQL
)

type periods struct {
	Name     string
	Duration time.Duration
}

func DbInit(dbname, hostname, port, username, password string) (*sql.DB, error) {

	db, err := DbConnection(dbname, hostname, port, username, password)
	if err != nil {
		return db, err
	}

	err = CreateOrdersTable(db)
	if err != nil {
		return db, err
	}

	err = CreateOrdersInfoTable(db)
	if err != nil {
		return db, err
	}

	periods := []periods{
		{Name: "ch1m", Duration: time.Second * 60},
		{Name: "ch3m", Duration: time.Minute * 3},
		{Name: "ch15m", Duration: time.Minute * 15},
		{Name: "ch1h", Duration: time.Hour},
		{Name: "ch4h", Duration: time.Hour * 4},
		{Name: "ch12h", Duration: time.Hour * 12},
	}

	for _, period := range periods {
		err = CreateTableName(db, period.Name)
		if err != nil {
			return db, err
		}
	}

	return db, nil
}

func dsn(dbname, hostname, port, username, password string) string {
	loc := `&loc=Local`
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&%s", username, password, hostname, port, dbname, loc)
}

func DbConnection(dbname, hostname, port, username, password string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn(dbname, hostname, port, username, password))
	if err != nil {
		return nil, fmt.Errorf("error %s when opening DB", err)
	}
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+dbname)
	if err != nil {
		return nil, fmt.Errorf("error %s when creating DB", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error %s when fetching rows", err)
	}
	db.Close()

	db, err = sql.Open("mysql", dsn(dbname, hostname, port, username, password))
	if err != nil {
		return nil, fmt.Errorf("error %s when opening DB", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(time.Minute * 5)

	ctx, cancelfunc = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("errors %s pinging DB", err)
	}

	return db, nil
}

func InsertCandlesTables(db *sql.DB, candle exModel.Candle) error {

	query := "INSERT INTO candles (Time,Pair,Open,Close,High,Low,Volume) VALUES (?,?,?,?,?,?,?)"
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	stmtLicense, err := db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmtLicense.Close()

	res, err := stmtLicense.ExecContext(
		ctx,
		candle.Time,
		candle.Pair,
		candle.Open,
		candle.Close,
		candle.High,
		candle.Low,
		candle.Volume,
	)
	if err != nil {
		return fmt.Errorf("error %s when inserting row into candles table", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when finding rows affected", err)
	}

	return nil
}

func SelectCandlesTable(db *sql.DB) ([]exModel.Candle, error) {

	query := "select Time,Pair,Open,Close,Low,High,Volume,QuoteVolume,AmountTrade,AmountTradeBuy,ActiveBuyVolume from candlesch1m;"

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candles := []exModel.Candle{}
	for rows.Next() {
		candle := exModel.Candle{}
		if err := rows.Scan(
			&candle.Time,
			&candle.Pair,
			&candle.Open,
			&candle.Close,
			&candle.Low,
			&candle.High,
			&candle.Volume,
			&candle.QuoteVolume,
			&candle.AmountTrade,
			&candle.AmountTradeBuy,
			&candle.ActiveBuyVolume,
		); err != nil {
			return nil, err
		}

		candle.AmountTradeAsk = candle.AmountTrade - candle.AmountTradeBuy
		candle.ActiveAskVolume = candle.Volume - candle.ActiveBuyVolume
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candles, nil
}

func CreateOrdersTable(db *sql.DB) error {

	query := `CREATE TABLE IF NOT EXISTS orders(
		ID int primary key auto_increment,
		TimeCreated datetime,
		Time datetime,
		Pair text,
		Side text,
		Type text,
		Status text,
		PriceCreated float,
		Price float,
		Quantity float,
		Profit float
		)`

	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when creating orders", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when getting rows affected", err)
	}
	return nil
}

func CreateTableName(db *sql.DB, tableName string) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
        Id int primary key auto_increment,
        Time datetime,
        Pair VARCHAR(20),
        Open DOUBLE,
        Close DOUBLE,
        Low DOUBLE,
        High DOUBLE,
        Volume DOUBLE,
        QuoteVolume DOUBLE,
        AmountTrade INT,
        AmountTradeBuy INT,
        ActiveBuyVolume DOUBLE,
        ActiveBuyQuoteVolume DOUBLE
    )`, "candles"+tableName)

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when creating %s table", err, "candles"+tableName)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when getting rows affected", err)
	}

	if rowsAffected > 0 {
		indexQuery := fmt.Sprintf(`CREATE INDEX idx_pair ON %s (Pair)`, "candles"+tableName)
		_, err = db.ExecContext(ctx, indexQuery)
		if err != nil {
			return fmt.Errorf("error %s when creating index on %s table", err, "candles"+tableName)
		}
	}

	return nil
}

func Orders(db *sql.DB) ([]*exModel.Order, error) {

	orders := []*exModel.Order{}

	query := "select * from orders;"
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return orders, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return orders, err
	}
	defer rows.Close()

	for rows.Next() {
		var order exModel.Order
		if err := rows.Scan(
			&order.ID,
			&order.TimeCreated,
			&order.Time,
			&order.Pair,
			&order.Side,
			&order.Type,
			&order.Status,
			&order.PriceCreated,
			&order.Price,
			&order.Quantity,
			&order.Profit); err != nil {
			return orders, err
		}
		orders = append(orders, &order)
	}
	if err := rows.Err(); err != nil {
		return orders, err
	}

	return orders, nil
}

func CreateOrder(db *sql.DB, order *exModel.Order) (int64, error) {

	query := "INSERT INTO orders (TimeCreated,Time,Pair,Side,Type,Status,PriceCreated,Price,Quantity,Profit) VALUES (?,?,?,?,?,?,?,?,?,?)"
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	stmtLicense, err := db.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmtLicense.Close()

	res, err := stmtLicense.ExecContext(
		ctx,
		order.TimeCreated.Format("2006-01-02 15:04:05"),
		order.Time.Format("2006-01-02 15:04:05"),
		order.Pair,
		order.Side,
		order.Type,
		order.Status,
		order.PriceCreated,
		order.Price,
		order.Quantity,
		order.Profit,
	)
	if err != nil {
		return 0, fmt.Errorf("error %s when inserting row into orders table", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error %s get last id", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error %s when finding rows affected", err)
	}

	return id, nil
}

func ClosePosition(db *sql.DB, order *exModel.Order, id int64) error {

	query := "UPDATE orders SET Time=?,Status=?,Price=?,Profit=? WHERE ID=?"
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	updateOrder, err := db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer updateOrder.Close()

	res, err := updateOrder.ExecContext(
		ctx,
		order.Time,
		order.Status,
		order.Price,
		order.Profit,
		id,
	)
	if err != nil {
		return fmt.Errorf("error %s when updating row into orders table", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return err
	}

	return nil
}

func CreateOrdersInfoTable(db *sql.DB) error {

	query := `CREATE TABLE IF NOT EXISTS orders_info(
		ID int primary key auto_increment,
		idOrder int,
		frame text,
		strategy text,
		comment text,
		marketsStat json,
		changePrices json,
		deltaFast json
		)`

	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when creating orders_info", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when getting rows affected", err)
	}
	return nil
}

func InsertOrdersInfoTable(db *sql.DB, idOrder int64, frame, strategy, comment string, mkStat, chData, dFast []byte) error {

	query := "INSERT INTO orders_info (idOrder,frame,strategy,comment,marketsStat,changePrices,deltaFast) VALUES (?,?,?,?,?,?,?)"
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	stmtLicense, err := db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmtLicense.Close()

	res, err := stmtLicense.ExecContext(
		ctx,
		idOrder,
		frame,
		strategy,
		comment,
		mkStat,
		chData,
		dFast,
	)
	if err != nil {
		return fmt.Errorf("error %s when inserting row into orders_info table", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when finding rows affected", err)
	}

	return nil
}

func SelectMarketStateTime(db *sql.DB, pair string, timeRounding time.Time) (exModel.Candle, error) {

	candle := exModel.Candle{}

	query := "select Close,Volume from candles WHERE Pair = ? and Time = ?;"

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	stmt, err := db.PrepareContext(ctx, query)

	if err != nil {
		return candle, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmt.Close()

	err = stmt.QueryRowContext(ctx, pair, timeRounding.Format("2006-01-02 15:04:05")).Scan(
		&candle.Close,
		&candle.Volume,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return candle, nil
		}
		return candle, err
	}

	return candle, nil
}

func SelectMarketStateTimev2(db *sql.DB, timeRounding time.Time) ([]exModel.Candle, error) {
	candles := []exModel.Candle{}

	query := "SELECT Time, Pair, Close, Volume,ActiveBuyVolume,AmountTrade,AmountTradeBuy FROM candlesch1m WHERE Time >= ? ORDER BY Time DESC;"

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return candles, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, timeRounding.Format("2006-01-02 15:04:05"))
	if err != nil {
		return candles, err
	}
	defer rows.Close()

	for rows.Next() {
		var candle exModel.Candle
		if err := rows.Scan(&candle.Time,
			&candle.Pair,
			&candle.Close,
			&candle.Volume,
			&candle.ActiveBuyVolume,
			&candle.AmountTrade,
			&candle.AmountTradeBuy,
		); err != nil {
			return candles, err
		}
		candle.AmountTradeAsk = candle.AmountTrade - candle.AmountTradeBuy
		candle.ActiveAskVolume = candle.Volume - candle.ActiveBuyVolume
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return candles, err
	}

	return candles, nil
}

func SelectDeltaPeriod(db *sql.DB, pair string, period string) ([]exModel.ChangeDeltaForCandle, error) {
	candles := []exModel.ChangeDeltaForCandle{}

	maping := map[string]string{
		"1m":  "ch1m",
		"3m":  "ch3m",
		"15m": "ch15m",
		"1h":  "ch1h",
		"4h":  "ch4h",
		"1d":  "ch12h",
	}

	query := fmt.Sprintf("select Time,Volume,ActiveBuyVolume,AmountTrade,AmountTradeBuy,Open,Close,High,Low  from %s WHERE Pair = ?;", "candles"+maping[period])

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return candles, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, pair)
	if err != nil {
		return candles, err
	}
	defer rows.Close()

	for rows.Next() {
		var changeDelta model.ChangeDeltaForCandle
		if err := rows.Scan(
			&changeDelta.Time,
			&changeDelta.Volume,
			&changeDelta.VolumeBuy,
			&changeDelta.Trades,
			&changeDelta.TradesBuy,
			&changeDelta.Open,
			&changeDelta.Close,
			&changeDelta.High,
			&changeDelta.Low,
		); err != nil {
			return candles, err
		}
		changeDelta.TradesAsk = changeDelta.Trades - changeDelta.TradesBuy
		changeDelta.VolumeAsk = changeDelta.Volume - changeDelta.VolumeBuy
		candles = append(candles, changeDelta)
	}
	if err := rows.Err(); err != nil {
		return candles, err
	}

	return candles, nil
}
