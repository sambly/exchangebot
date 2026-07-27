// Package entrysetup оценивает УДОБСТВО ВХОДА в сделку по паре прямо сейчас:
// есть ли рядом естественный тесный стоп и достаточно места до тейка. Это
// НЕ стратегия - здесь нет сигналов, сделок и подписчиков, только вычисления
// поверх уже собранных данных.
//
// По структуре и назначению - аналог internal/depth и internal/prices: то же
// read-only хранилище/расчёт, к которому подключаются потребители (веб,
// телеграм, стратегии), а не наоборот. Причина отдельного пакета, а не
// методов внутри depth или prices: показатель удобства входа комбинирует ОБА
// источника (стакан + цена/волатильность), и класть его в один из них значит
// заставить этот пакет знать о другом, которому он сейчас ничего не должен.
package entrysetup

import (
	"github.com/sambly/exchangebot/internal/depth"
	"github.com/sambly/exchangebot/internal/prices"
)

// AssetsSetup считает показатели удобства входа по парам, опираясь на уже
// собранные AssetsPrices (цена/объём/волатильность) и AssetsDepth (L2-стакан).
// Оба поля - внешние read-only источники, AssetsSetup не владеет их данными и
// не подписывается на обновления сам: расчёт идёт по запросу, на снапшоте
// текущего состояния источников.
type AssetsSetup struct {
	prices *prices.AssetsPrices
	depth  *depth.AssetsDepth
}

// NewAssetsSetup - prices и depth должны быть уже инициализированы вызывающим
// кодом (см. application.NewApp): AssetsSetup сам ничего не забирает у биржи.
func NewAssetsSetup(assetsPrices *prices.AssetsPrices, assetsDepth *depth.AssetsDepth) *AssetsSetup {
	return &AssetsSetup{
		prices: assetsPrices,
		depth:  assetsDepth,
	}
}
