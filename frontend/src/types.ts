// Ответ от /trade/api/getChPrice
export interface MarketsStatEntry {
  Price: number
  Volume: number
  Ch24: number
}

export interface MarketsStat {
  [pair: string]: MarketsStatEntry
}

export interface ChangePriceEntry {
  ChangePercent: number
  Price: number
}

export interface ChangePrices {
  [pair: string]: {
    '1m': ChangePriceEntry
    '3m': ChangePriceEntry
    '15m': ChangePriceEntry
    '1h': ChangePriceEntry
    '4h': ChangePriceEntry
    '12h': ChangePriceEntry
  }
}

// Статус подписки exchange_service на пару: "Active" | "Inactive".
// Пара может отсутствовать в карте, если exchange_service ещё не ответил -
// это неопределённый статус, а не "Inactive".
export interface FeedStatus {
  [pair: string]: string
}

export interface GetChPriceResponse {
  MarketsStat: MarketsStat
  ChangePrices: ChangePrices
  FeedStatus: FeedStatus
}

// Ответ от /trade/api/getChDelta
export interface DeltaEntry {
  Volume: number
  VolumeBuy: number
  VolumeAsk: number
  Trades: number
  TradesBuy: number
  TradesAsk: number
}

export interface DeltaData {
  [frame: string]: DeltaEntry
}

export interface DeltaFast {
  [pair: string]: DeltaData
}

export interface GetChDeltaResponse {
  DeltaFast: DeltaFast
}

// Ответ от /trade/api/getDepthImbalance
export interface ImbalanceEntry {
  Imbalance: number
  ZScore: number
  Ready: boolean
}

export interface ImbalanceData {
  [pair: string]: ImbalanceEntry
}

export interface GetDepthImbalanceResponse {
  Imbalance: ImbalanceData
}

// Ответ от /trade/api/getEntryQuality
//
// Score - во сколько раз дальняя стена стакана дальше ближней; НЕ зависит от
// периода (см. Go-комментарий у entrysetup.Quality) - берётся из любого
// доступного периода записи. StopDistanceSigma/TakeDistanceSigma - те же
// дистанции в единицах волатильности пары ЗА КОНКРЕТНЫЙ период, ими периоды
// уже различаются.
export interface QualityEntry {
  Side: 'BUY' | 'SELL'
  StopDistancePercent: number
  TakeDistancePercent: number
  Score: number
  StopDistanceSigma: number
  TakeDistanceSigma: number
  HasVolatility: boolean
}

export interface QualityData {
  [pair: string]: {
    [period: string]: QualityEntry
  }
}

// PriceLevel/PriceLevels - уровни поддержки/сопротивления ПО ИСТОРИИ ЦЕНЫ
// (swing high/low за последние ~100 свечей периода), в отличие от Quality -
// это не стакан, устойчивее к спуфингу, но запаздывает относительно текущего
// момента.
export interface PriceLevel {
  Price: number
  Time: string
  DistancePercent: number
}

export interface PriceLevelsEntry {
  Support: PriceLevel
  HasSupport: boolean
  Resistance: PriceLevel
  HasResistance: boolean
}

export interface PriceLevelsData {
  [pair: string]: {
    [period: string]: PriceLevelsEntry
  }
}

// VolatilityRegimeData - отношение недавней волатильности к типичной за тот
// же период. <1 - сжатие (часто предшествует выносу), >1 - расширение
// (уже разогналось). См. Go-комментарий у AssetsPrices.GetVolatilityRegime.
export interface VolatilityRegimeData {
  [pair: string]: {
    [period: string]: number
  }
}

export interface GetEntryQualityResponse {
  Quality: QualityData
  PriceLevels: PriceLevelsData
  VolatilityRegime: VolatilityRegimeData
}

// Ответ от /trade/api/getStrength - сырые составляющие "скрытой силы" пары,
// каждая со своим Has-флагом (сигнал сейчас недоступен - не ноль, а "нет
// данных"). Композит из них и то, какие компоненты учитывать, считает фронт
// (чекбоксы в DataStrength.vue) - см. Go-комментарий у entrysetup.StrengthComponents.
export interface StrengthEntry {
  HasImbalance: boolean
  ImbalanceZScore: number
  // ImbalanceConfirmed - сторона имбаланса держится несколько сэмплов подряд
  // (см. Go-комментарий у depth.GetImbalanceConfirmedSide), а не мелькнула на
  // одном снимке стакана. Раздельно от Has* - значение показываем всегда,
  // подтверждение решает, учитывать ли его в композите "Потенциала".
  ImbalanceConfirmed: boolean

  HasWalls: boolean
  WallsSide: 'BUY' | 'SELL'
  WallsScore: number
  // WallsConfirmed - см. ImbalanceConfirmed, но для стен.
  WallsConfirmed: boolean

  HasLevels: boolean
  LevelsSide: 'BUY' | 'SELL'
  LevelsScore: number

  HasRegime: boolean
  Regime: number

  HasActivity: boolean
  ActivityZScore: number
}

export interface StrengthData {
  [pair: string]: {
    [period: string]: StrengthEntry
  }
}

export interface GetStrengthResponse {
  Strength: StrengthData
}

// Ответ от /trade/api/getStrategiesStatus - стратегии с runtime-переключателями
// (см. Go-комментарий у strategy.WebToggle/strategy.WebNotificationToggle).
// Has* независимы: у base есть только уведомления, у executor - только
// Enabled, у anomaly - оба. Стратегия без обоих в списке не появится вовсе.
export interface StrategyStatus {
  IDName: string
  Name: string
  HasEnable: boolean
  Enabled: boolean
  HasNotifications: boolean
  NotificationsEnabled: boolean
}