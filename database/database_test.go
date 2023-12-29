package database

import (
	"testing"
)

func TestSelectCandlesTable(t *testing.T) {

	db, err := DbConnection()
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	_, err = SelectCandlesTable(db)
	if err != nil {
		t.Error(err)
	}

}
