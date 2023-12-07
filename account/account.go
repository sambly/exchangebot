package account

import (
	"context"
	"main/model"
	"main/service"
	"sync"

	"golang.org/x/exp/slices"
)

type Account struct {
	sync.Mutex
	exchange  service.Exchange
	assetsKey []string                // пары к USDT которые есть на на Spot, Flexible, Staking
	assets    map[string]*model.Asset // Сруктура пары к USDT
}

func NewAccount(exchange service.Exchange) (*Account, error) {
	acc := Account{
		exchange:  exchange,
		assetsKey: make([]string, 0),
		assets:    make(map[string]*model.Asset),
	}
	if err := acc.UpdateAssets(); err != nil {
		return nil, err
	}
	return &acc, nil
}

func (acc *Account) UpdateAssets() (err error) {

	acc.Lock()
	defer acc.Unlock()
	// Обнуляем позиции для последующего обновления
	acc.assetsKey = make([]string, 0)
	// Сбрасываем состояние On (наличие элемента в структуре)
	for _, item := range acc.assets {
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
	for key := range acc.assets {
		if !acc.assets[key].On {
			delete(acc.assets, key)
		}
	}

	return nil
}

func (acc *Account) feederAssets(data []model.AssetData, typeData string) {

	for _, value := range data {

		valueAsset := value.AssetBase + "USDT"
		if idx := slices.Index(acc.assetsKey, valueAsset); idx == -1 {
			acc.assetsKey = append(acc.assetsKey, valueAsset)
		}
		if _, ok := acc.assets[valueAsset]; !ok {
			acc.assets[valueAsset] = &model.Asset{Name: valueAsset, On: true}
		}

		assetData := &model.AssetData{
			AssetBase: valueAsset,
			Amount:    value.Amount,
			FullPrice: acc.assets[valueAsset].Price * value.Amount,
		}
		if acc.assets[valueAsset].CommonData == nil {
			acc.assets[valueAsset].CommonData = &model.AssetData{AssetBase: valueAsset, Amount: 0, FullPrice: 0}
		}

		if typeData == "AssetSpot" {
			acc.assets[valueAsset].SpotData = assetData
			acc.assets[valueAsset].CommonData.Amount = acc.assets[valueAsset].CommonData.Amount + acc.assets[valueAsset].SpotData.Amount
		}
		if typeData == "AssetFlexible" {
			acc.assets[valueAsset].FlexibleData = assetData
			acc.assets[valueAsset].CommonData.Amount = acc.assets[valueAsset].CommonData.Amount + acc.assets[valueAsset].FlexibleData.Amount
		}
		if typeData == "AssetStaking" {
			acc.assets[valueAsset].StakingData = assetData
			acc.assets[valueAsset].CommonData.Amount = acc.assets[valueAsset].CommonData.Amount + acc.assets[valueAsset].StakingData.Amount
		}
		acc.assets[valueAsset].CommonData.FullPrice = acc.assets[valueAsset].CommonData.Amount * acc.assets[valueAsset].Price
	}

}
