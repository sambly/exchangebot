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

// Account хранит балансы аккаунта. Обработчики telebot выполняются конкурентно
// (каждый апдейт в своей горутине), поэтому доступ к assets/assetsKey обязан
// быть под мьютексом: раньше "Обновить данные" параллельно с "BALANCE" давали
// не просто гонку, а fatal error - конкурентные итерацию и запись map.
type Account struct {
	mu           sync.RWMutex
	exchange     exchange.Exchange
	Notification *notification.Notification
	AssetPrices  *prices.AssetsPrices

	assetsKey []string                  // пары к USDT которые есть на Spot, Flexible, Staking
	assets    map[string]*exModel.Asset // Структура пары к USDT

	BaseLimitAsset float64
}

var accLogger = logger.AddFields(map[string]interface{}{
	"package": "account",
})

func NewAccount(exchange exchange.Exchange, assetPrices *prices.AssetsPrices) (*Account, error) {
	acc := Account{
		exchange:       exchange,
		assetsKey:      make([]string, 0),
		assets:         make(map[string]*exModel.Asset),
		AssetPrices:    assetPrices,
		BaseLimitAsset: 1.0,
	}
	return &acc, nil
}

// GetAssets отдаёт копию балансов: наружу не должны утекать ссылки на map,
// которую UpdateAssets перестраивает.
func (acc *Account) GetAssets() map[string]exModel.Asset {
	acc.mu.RLock()
	defer acc.mu.RUnlock()

	assets := make(map[string]exModel.Asset, len(acc.assets))
	for name, asset := range acc.assets {
		if asset != nil {
			assets[name] = *asset
		}
	}
	return assets
}

// GetAssetsKeys отдаёт копию списка пар
func (acc *Account) GetAssetsKeys() []string {
	acc.mu.RLock()
	defer acc.mu.RUnlock()

	keys := make([]string, len(acc.assetsKey))
	copy(keys, acc.assetsKey)
	return keys
}

func (acc *Account) UpdateAssets() error {
	// Запросы к бирже делаем ДО захвата мьютекса: держать его на время трёх
	// HTTP-вызовов значило бы блокировать на секунды все чтения балансов.
	assetsSpotRaw, err := acc.exchange.GetAssetsSpot(context.Background())
	if err != nil {
		return err
	}
	assetsFlexible, err := acc.exchange.GetAssetsFlexibleV2(context.Background())
	if err != nil {
		return err
	}
	assetsStaking, err := acc.exchange.GetAssetsStaking(context.Background())
	if err != nil {
		return err
	}

	acc.mu.Lock()
	defer acc.mu.Unlock()

	acc.assetsKey = make([]string, 0)

	// Сброс старых данных
	for _, item := range acc.assets {
		item.On = false
		item.CommonData = nil
		item.SpotData = nil
		item.FlexibleData = nil
		item.StakingData = nil
	}

	acc.feederAssets(assetsSpotRaw, "AssetSpot")
	acc.feederAssets(assetsFlexible, "AssetFlexible")
	acc.feederAssets(assetsStaking, "AssetStaking")

	acc.assetsKey = nil

	for key := range acc.assets {
		asset := acc.assets[key]
		if !asset.On || asset.CommonData == nil || asset.CommonData.FullPrice < acc.BaseLimitAsset {
			delete(acc.assets, key)
		} else {
			acc.assetsKey = append(acc.assetsKey, key)
		}
	}

	return nil
}

// feederAssets вызывается только из UpdateAssets, уже под acc.mu.Lock()
func (acc *Account) feederAssets(data []exModel.AssetData, typeData string) {
	for _, value := range data {
		valueAsset := value.AssetBase + "USDT"

		if _, ok := acc.assets[valueAsset]; !ok {
			acc.assets[valueAsset] = &exModel.Asset{Name: valueAsset}
		}
		asset := acc.assets[valueAsset]
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
