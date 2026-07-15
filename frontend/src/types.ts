// Ответ от /trade/api/getChPrice
export interface MarketsStatEntry {
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
    '1d': ChangePriceEntry
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