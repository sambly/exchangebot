package account

import (
	"context"
	"sync"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
)

type Account struct {
	// TODO не реализован полноценно Mutex
	sync.Mutex
	exchange     exchange.Exchange
	Notification *notification.Notification
	AssetPrices  *prices.AssetsPrices
	AssetsKey    []string                  // пары к USDT которые есть на Spot, Flexible, Staking
	Assets       map[string]*exModel.Asset // Структура пары к USDT

	BaseLimitAsset float64
}

var accLogger = logger.AddFields(map[string]interface{}{
	"package": "account",
})

func NewAccount(exchange exchange.Exchange, assetPrices *prices.AssetsPrices) (*Account, error) {
	acc := Account{
		exchange:       exchange,
		AssetsKey:      make([]string, 0),
		Assets:         make(map[string]*exModel.Asset),
		AssetPrices:    assetPrices,
		BaseLimitAsset: 1.0,
	}
	return &acc, nil
}

func (acc *Account) UpdateAssets() error {
	acc.AssetsKey = make([]string, 0)

	// Сброс старых данных
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

	assetsFlexible, err := acc.exchange.GetAssetsFlexibleV2(context.Background())
	if err != nil {
		return err
	}
	acc.feederAssets(assetsFlexible, "AssetFlexible")

	assetsStaking, err := acc.exchange.GetAssetsStaking(context.Background())
	if err != nil {
		return err
	}
	acc.feederAssets(assetsStaking, "AssetStaking")

	acc.AssetsKey = nil

	for key := range acc.Assets {
		asset := acc.Assets[key]
		if !asset.On || asset.CommonData == nil || asset.CommonData.FullPrice < acc.BaseLimitAsset {
			delete(acc.Assets, key)
		} else {
			acc.AssetsKey = append(acc.AssetsKey, key)
		}
	}

	return nil
}

func (acc *Account) feederAssets(data []exModel.AssetData, typeData string) {
	for _, value := range data {
		valueAsset := value.AssetBase + "USDT"

		if _, ok := acc.Assets[valueAsset]; !ok {
			acc.Assets[valueAsset] = &exModel.Asset{Name: valueAsset}
		}
		asset := acc.Assets[valueAsset]
		asset.On = true

		marketStat, err := acc.AssetPrices.GetMarketsStatForPair(valueAsset)
		if err != nil {
			accLogger.Warnf("Не удалось получить цену для %s: %v", valueAsset, err)
			continue
		}

		asset.Price = marketStat.Price

		assetData := &exModel.AssetData{
			AssetBase: valueAsset,
			Amount:    value.Amount,
			FullPrice: asset.Price * value.Amount,
		}

		if asset.CommonData == nil {
			asset.CommonData = &exModel.AssetData{
				AssetBase: valueAsset,
				Amount:    0,
				FullPrice: 0,
			}
		}

		switch typeData {
		case "AssetSpot":
			asset.SpotData = assetData
			asset.CommonData.Amount += assetData.Amount
		case "AssetFlexible":
			asset.FlexibleData = assetData
			asset.CommonData.Amount += assetData.Amount
		case "AssetStaking":
			asset.StakingData = assetData
			asset.CommonData.Amount += assetData.Amount
		}

		asset.CommonData.FullPrice = asset.CommonData.Amount * asset.Price
	}
}
