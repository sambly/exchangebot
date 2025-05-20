package service

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/sambly/exchangeService/pkg/exchange"
)

func GetPairs(ctx context.Context, fromFile bool, exch exchange.Exchange) ([]string, error) {
	if fromFile {
		return GetPairsFile("configs/pairs.txt")
	}
	return exch.GetPairsToUSDT(ctx)
}

func GetPairsFile(fileName string) ([]string, error) {

	file, err := os.OpenFile(fileName, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening/creating file: %v", err)
	}
	defer file.Close()

	if stat, _ := file.Stat(); stat.Size() == 0 {
		return []string{}, fmt.Errorf("file - %s is empty", fileName)
	}

	var pairs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pairs = append(pairs, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return pairs, nil
}
