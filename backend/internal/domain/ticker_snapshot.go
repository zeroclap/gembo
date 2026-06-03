package domain

import "github.com/shopspring/decimal"

// TickerSnapshot is a point-in-time capture of a single exchange's market state
// for one symbol. Stored as a time-series (Redis sorted set, score = unix ts) to
// back chart queries and any future per-exchange analytics. No cross-exchange
// derived values are stored here — those are computed at read time.
type TickerSnapshot struct {
	ExchangeID  string          `json:"exchange_id"`
	Symbol      string          `json:"symbol"`
	TS          int64           `json:"ts"`           // unix seconds
	Price       decimal.Decimal `json:"price"`        // last traded price
	Bid         decimal.Decimal `json:"bid"`
	Ask         decimal.Decimal `json:"ask"`
	FundingRate decimal.Decimal `json:"funding_rate"` // current raw rate from exchange
	Rate8h      decimal.Decimal `json:"rate_8h"`      // rate normalized to 8h equivalent
	MarkPrice   decimal.Decimal `json:"mark_price"`
	IndexPrice  decimal.Decimal `json:"index_price"`
}

// TickerSnapshotBar is an OHLC bar aggregated from TickerSnapshots over a time
// bucket. The metric it represents (e.g. net funding rate, spread %) depends on
// the caller — this type carries only the shape.
type TickerSnapshotBar struct {
	TS    int64           `json:"ts"`    // bucket start, unix seconds
	Open  decimal.Decimal `json:"open"`
	High  decimal.Decimal `json:"high"`
	Low   decimal.Decimal `json:"low"`
	Close decimal.Decimal `json:"close"`
}
