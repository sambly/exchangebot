package service

import (
	"bufio"
	"fmt"
	"os"

	"github.com/sambly/exchangeService/pkg/exchange"
)

func GetPairs(fromFile bool, exch exchange.Exchange) ([]string, error) {
	if fromFile {
		return GetPairsFile("configs/pairs.txt")
	}
	return exch.GetPairsToUSDT()
}

func GetPairsFile(fileName string) ([]string, error) {
	var pairs []string
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pairs = append(pairs, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return pairs, nil
}
