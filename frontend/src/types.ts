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