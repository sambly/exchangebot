package account

import (
	"context"
	"sync"

	"github.com/sambly/exchangeBot/internal/notification"
	"github.com/sambly/exchangeBot/internal/prices"
	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"golang.org/x/exp/slices"
)

type Account struct {
	sync.Mutex
	exchange     exchange.Exchange
	Notification *notification.Notification
	AssetPrices  *prices.AsetsPrices
	AssetsKey    []string                  // пары к USDT которые есть на на Spot, Flexible, Staking
	Assets       map[string]*exModel.Asset // Сруктура пары к USDT
}

func NewAccount(exchange exchange.Exchange, assetPrices *prices.AsetsPrices, notification *notification.Notification) (*Account, error) {
	acc := Account{
		exchange:     exchange,
		AssetsKey:    make([]string, 0),
		Assets:       make(map[string]*exModel.Asset),
		AssetPrices:  assetPrices,
		Notification: notification,
	}
	return &acc, nil
}

func (acc *Account) UpdateAssets() (err error) {

	// Обнуляем позиции для последующего обновления
	acc.AssetsKey = make([]string, 0)
	// Сбрасываем состояние On (наличие элемента в структуре)
	for _, item := range acc.Assets {
		item.On = false
		item.CommonData = nil
		item.SpotData = nil
		item.FlexibleData = nil
		item.StakingData = nil
	}

	assetsSpotRaw, err := acc.exchange.GetAssetsSpot(context.Background())
	if err != nil {
		return err
	}
	acc.feederAssets(assetsSpotRaw, "AssetSpot")

	assetsFlexible, err := acc.exchange.GetAssetsFlexible(context.Background())
	if err != nil {
		return err
	}
	acc.feederAssets(assetsFlexible, "AssetFlexible")

	assetsStaking, err := acc.exchange.GetAssetsStaking(context.Background())
	if err != nil {
		return err
	}
	acc.feederAssets(assetsStaking, "AssetStaking")

	// Если позиция удаленна , удаляем ее из App
	for key := range acc.Assets {
		if !acc.Assets[key].On {
			delete(acc.Assets, key)
		}
	}

	return nil
}

func (acc *Account) feederAssets(data []exModel.AssetData, typeData string) {

	for _, value := range data {

		valueAsset := value.AssetBase + "USDT"
		if idx := slices.Index(acc.AssetsKey, valueAsset); idx == -1 {
			acc.AssetsKey = append(acc.AssetsKey, valueAsset)
		}
		if _, ok := acc.Assets[valueAsset]; !ok {
			acc.Assets[valueAsset] = &exModel.Asset{Name: valueAsset}
		}
		acc.Assets[valueAsset].On = true

		if _, ok := acc.AssetPrices.MarketsStat[valueAsset]; ok {
			acc.Assets[valueAsset].Price = acc.AssetPrices.MarketsStat[valueAsset].Price
		}
		assetData := &exModel.AssetData{
			AssetBase: valueAsset,
			Amount:    value.Amount,
			FullPrice: acc.Assets[valueAsset].Price * value.Amount,
		}
		if acc.Assets[valueAsset].CommonData == nil {
			acc.Assets[valueAsset].CommonData = &exModel.AssetData{AssetBase: valueAsset, Amount: 0, FullPrice: 0}
		}

		if typeData == "AssetSpot" {
			acc.Assets[valueAsset].SpotData = assetData
			acc.Assets[valueAsset].CommonData.Amount = acc.Assets[valueAsset].CommonData.Amount + acc.Assets[valueAsset].SpotData.Amount
		}
		if typeData == "AssetFlexible" {
			acc.Assets[valueAsset].FlexibleData = assetData
			acc.Assets[valueAsset].CommonData.Amount = acc.Assets[valueAsset].CommonData.Amount + acc.Assets[valueAsset].FlexibleData.Amount
		}
		if typeData == "AssetStaking" {
			acc.Assets[valueAsset].StakingData = assetData
			acc.Assets[valueAsset].CommonData.Amount = acc.Assets[valueAsset].CommonData.Amount + acc.Assets[valueAsset].StakingData.Amount
		}
		acc.Assets[valueAsset].CommonData.FullPrice = acc.Assets[valueAsset].CommonData.Amount * acc.Assets[valueAsset].Price
	}

}
