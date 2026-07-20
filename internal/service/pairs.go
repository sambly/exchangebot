package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
)

// PairsProvider — источник списка пар. Его удовлетворяют и Binance, и gRPC-клиент
// exchangeService, поэтому пары можно брать из того же источника, что и поток данных.
type PairsProvider interface {
	GetPairsToUSDT(ctx context.Context) ([]string, error)
}

func GetPairs(ctx context.Context, fromFile bool, provider PairsProvider) ([]string, error) {
	if fromFile {
		return GetPairsFile("configs/pairs.txt")
	}
	return provider.GetPairsToUSDT(ctx)
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
