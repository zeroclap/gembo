# Funding Arbitrage Feature

## Overview

Real-time cross-exchange funding rate arbitrage opportunity detection for perpetual contracts. The system identifies pairs where opening a **long** on one exchange and a **short** on another yields a positive net funding rate, and displays all relevant metrics for decision-making.

## Strategy: Long + Short (Cross-Exchange)

For each symbol available on 2+ exchanges:

1. **Short leg**: The exchange with the highest (most positive) funding rate -- shorts receive funding payments
2. **Long leg**: The exchange with the lowest (most negative) funding rate -- longs receive funding payments
3. **Net profit**: The differential between the two rates, minus trading fees and spread costs

### Key Concepts

- **Funding Rate**: Periodic payments between long/short holders on perpetual contracts. Positive rate = longs pay shorts. Negative rate = shorts pay longs.
- **Net Funding Rate (8h)**: The combined earning from both legs, normalized to an 8-hour period: `short_rate_8h - long_rate_8h`
- **APR**: Annualized net rate: `net_rate_8h * 3 * 365 * 100%`
- **Open Spread**: Cost of entering both positions simultaneously: `(short_bid - long_ask) / midpoint * 100%`. Negative = you pay to enter.
- **Full Loop**: The LCM of both funding intervals. Within one loop, each leg pays at its own interval. The cumulative P&L at each payment point shows where you're negative before the cycle completes.
- **Next Payment**: The very next funding event across both legs -- useful for "grab and go" partial-loop opportunities.

## Architecture

Three independent processes communicating via Redis:

```
Collector                Redis                  Worker                 API
(data ingestion)         (store + message bus)  (event processing)     (read-only HTTP)

Ticker WS ──> SetTicker ──> Pub/Sub ──────────> Subscribe
                  |          channel                |
                  +-- SET key + data                +-- Read FR+Ticker
                                                    |   for symbol S
Funding REST -> SetFundingRate -> Pub/Sub -------> Subscribe
                  |               channel           |
                  +-- SET key + data                +-- Compute arbitrage
                                                    |
                                                    +-- SetFundingArbitrage --> GET /v1/arbitrage/funding
```

### Process Responsibilities

| Binary | Responsibility | Scaling |
|---|---|---|
| `cmd/collector` | Ingests tickers (WS) and funding rates (REST) from exchanges. Stores in Redis. Publishes Pub/Sub events. Does NOT compute arbitrage. | One per exchange set |
| `cmd/worker` | Subscribes to Redis Pub/Sub events. On each ticker/funding update, recomputes arbitrage opportunities for the affected symbol across all exchange pairs. Writes results to Redis. | Horizontally (future: Redis Streams + consumer groups) |
| `cmd/api` | Serves pre-computed arbitrage opportunities from Redis via REST. No computation. | Horizontally behind LB |

### Event Transport: Redis Pub/Sub

**Channels**:
- `gembo:<exchange>:ticker:updates` (already exists)
- `gembo:<exchange>:funding:updates` (new, mirrors ticker pattern)

**Worker subscription**: Pattern subscribe `gembo:*:ticker:updates` and `gembo:*:funding:updates`.

**Trade-off**: Pub/Sub is fire-and-forget. If the worker is down, events are lost. This is acceptable because data is continuously refreshed -- the next update cycle triggers recomputation. For durability, migrate to Redis Streams with consumer groups later.

## Data Model

### Enriched FundingRate (domain/funding_rate.go)

New fields from CCXT `FundingRate` struct:

| Field | Type | Description |
|---|---|---|
| `MarkPrice` | `decimal.Decimal` | Mark price used for funding calculation |
| `IndexPrice` | `decimal.Decimal` | Spot index price |
| `NextFundingRate` | `decimal.Decimal` | Predicted next funding rate (exchange-provided) |
| `PreviousFundingRate` | `decimal.Decimal` | Previous period's rate (trend indicator) |
| `InterestRate` | `decimal.Decimal` | Base interest rate component |

### FundingArbitrage (domain/arbitrage.go)

```
FundingArbitrage
|-- Symbol                          string
|-- Long                            ArbitrageLeg
|   |-- ExchangeID                  string
|   |-- FundingRate                 decimal     // current rate
|   |-- IntervalHours               int         // 4, 8, etc.
|   |-- Rate8h                      decimal     // normalized to 8h
|   |-- NextFundingAt               time.Time
|   |-- NextFundingRate             decimal     // predicted next rate
|   |-- Price, Bid, Ask             decimal
|   |-- MarkPrice, IndexPrice       decimal
|   |-- Volume24h                   decimal
|   |-- MakerFeePct, TakerFeePct    decimal     // mock values
|
|-- Short                           ArbitrageLeg (same structure)
|
|-- NetRate8h                       decimal     // short_rate_8h - long_rate_8h
|-- APR                             decimal     // annualized percentage
|-- OpenSpread                      decimal     // entry cost percentage
|
|-- Fees                            ArbitrageFees
|   |-- OpenFeesPct                 decimal     // taker on both legs to enter
|   |-- CloseFeesPct                decimal     // taker on both legs to exit
|   |-- RoundTripFeesPct            decimal     // open + close
|   |-- NetAPRAfterFees             decimal     // APR adjusted for round-trip
|
|-- NextPayment                     NextPaymentInfo
|   |-- ExchangeID                  string      // which exchange pays next
|   |-- Side                        string      // "long" or "short"
|   |-- Rate                        decimal     // the rate for this payment
|   |-- PaymentPct                  decimal     // your P&L for this single payment
|   |-- At                          time.Time   // when it occurs
|   |-- TimeUntilSecs               int64       // seconds from computation time
|
|-- FullLoop                        FullLoopInfo
|   |-- DurationHours               int         // LCM of both intervals
|   |-- Payments                    []LoopPayment (ordered by AtHour)
|   |   |-- ExchangeID              string
|   |   |-- Side                    string      // "long" or "short"
|   |   |-- Rate                    decimal
|   |   |-- PaymentPct              decimal     // your +/- for this step
|   |   |-- AtHour                  int         // hour offset within loop
|   |   |-- Cumulative              decimal     // running P&L up to here
|   |-- NetPctPerLoop               decimal     // total P&L for one full cycle
|
|-- UpdatedAt                       time.Time
```

### Full Loop Calculation (Theoretical)

For long_interval=4h, short_interval=8h:

```
LCM(4, 8) = 8h

Payments within one loop:
  Hour 4: Long leg  -> P&L = -rate_long  (you're long, funding acts on you)
  Hour 8: Long leg  -> P&L = -rate_long
  Hour 8: Short leg -> P&L = +rate_short (you're short, you receive)

Net per loop = 2*(-rate_long) + 1*(rate_short)

If rate_long = -0.02% (negative, so you RECEIVE as long):
  -(-0.02%) = +0.02% per payment = +0.04% for 2 payments

If rate_short = +0.05% (positive, so you RECEIVE as short):
  +0.05% for 1 payment

Net per 8h loop = +0.04% + 0.05% = +0.09%
```

Each payment in the `Payments` array has a `Cumulative` field showing the running P&L, so the user can see intermediate states and assess partial-loop viability.

### Mock Fee Values

| Exchange | Maker | Taker |
|---|---|---|
| bybit | 0.02% | 0.055% |
| okx | 0.02% | 0.05% |

Entry and exit use taker fees (market orders assumed):
- `OpenFeesPct = taker_long + taker_short`
- `CloseFeesPct = taker_long + taker_short`
- `RoundTripFeesPct = OpenFeesPct + CloseFeesPct`

### Redis Key Structure

| Key | Type | Description |
|---|---|---|
| `gembo:arbitrage:funding:pairs` | SET | All pair identifiers |
| `gembo:arbitrage:funding:<symbol>\|<exA>\|<exB>` | STRING (JSON) | Single opportunity (exA < exB alphabetically) |
| `gembo:<exchange>:funding:updates` | Pub/Sub channel | Funding rate event notifications |

## API Endpoint

### `GET /v1/arbitrage/funding`

Returns all pre-computed funding arbitrage opportunities, sorted by APR descending.

**Response**:
```json
{
  "data": [
    {
      "symbol": "BTC/USDT:USDT",
      "long": {
        "exchange_id": "okx",
        "funding_rate": "-0.0002",
        "interval_hours": 8,
        "rate_8h": "-0.0002",
        "next_funding_at": "2026-03-30T16:00:00Z",
        "next_funding_rate": "-0.00015",
        "price": "87250.5",
        "bid": "87250.0",
        "ask": "87251.0",
        "mark_price": "87250.3",
        "index_price": "87248.0",
        "volume_24h": "12500000",
        "maker_fee_pct": "0.0002",
        "taker_fee_pct": "0.0005"
      },
      "short": {
        "exchange_id": "bybit",
        "funding_rate": "0.0003",
        "interval_hours": 8,
        "rate_8h": "0.0003",
        "next_funding_at": "2026-03-30T16:00:00Z",
        "next_funding_rate": "0.00025",
        "price": "87255.0",
        "bid": "87254.5",
        "ask": "87255.5",
        "mark_price": "87254.8",
        "index_price": "87248.0",
        "volume_24h": "15000000",
        "maker_fee_pct": "0.0002",
        "taker_fee_pct": "0.00055"
      },
      "net_rate_8h": "0.0005",
      "apr": "54.75",
      "open_spread": "0.004",
      "fees": {
        "open_fees_pct": "0.00105",
        "close_fees_pct": "0.00105",
        "round_trip_fees_pct": "0.0021",
        "net_apr_after_fees": "51.88"
      },
      "next_payment": {
        "exchange_id": "bybit",
        "side": "short",
        "rate": "0.0003",
        "payment_pct": "0.0003",
        "at": "2026-03-30T16:00:00Z",
        "time_until_secs": 7200
      },
      "full_loop": {
        "duration_hours": 8,
        "payments": [
          {
            "exchange_id": "okx",
            "side": "long",
            "rate": "-0.0002",
            "payment_pct": "0.0002",
            "at_hour": 8,
            "cumulative": "0.0002"
          },
          {
            "exchange_id": "bybit",
            "side": "short",
            "rate": "0.0003",
            "payment_pct": "0.0003",
            "at_hour": 8,
            "cumulative": "0.0005"
          }
        ],
        "net_pct_per_loop": "0.0005"
      },
      "updated_at": "2026-03-30T14:00:05Z"
    }
  ],
  "error": null,
  "ts": "2026-03-30T14:00:10Z"
}
```

## Frontend

### Arbitrage Table (app/arbitrage/arbitrage-table.tsx)

Client component using TanStack React Table.

**Main columns** (all sortable):

| Column | Content |
|---|---|
| Symbol | Trading pair |
| Long | Exchange badge + rate% + /Nh |
| Short | Exchange badge + rate% + /Nh |
| Net Rate (8h) | Normalized differential (green) |
| APR | Annualized %, bold |
| Net APR | After round-trip fees |
| Open Spread | Entry cost %, green if positive, red if negative |
| Next Payment | Live countdown + exchange + amount |
| Volume | Min of both legs |

**Default sort**: APR descending.

**Expandable sub-row** per opportunity:
- Full loop payment timeline (table with cumulative P&L column)
- Both legs: price, bid, ask, mark price, index price, predicted next rate
- Fee breakdown: open / close / round-trip

**Refresh**: Same 15s polling interval as tickers page. Countdown timer for "Next Payment" computed client-side from `time_until_secs` + response timestamp.

## File Changes Summary

| # | File | Action |
|---|---|---|
| 1 | `backend/internal/domain/funding_rate.go` | Edit: add 5 new CCXT fields |
| 2 | `backend/internal/domain/arbitrage.go` | New: full arbitrage model hierarchy |
| 3 | `backend/internal/exchange/exchanges/bybit/mapper.go` | Edit: map new CCXT fields |
| 4 | `backend/internal/exchange/exchanges/okx/mapper.go` | Edit: map new CCXT fields |
| 5 | `backend/pkg/rediskey/keys.go` | Edit: add arbitrage + funding Pub/Sub keys |
| 6 | `backend/internal/store/store.go` | Edit: add arbitrage methods to interface |
| 7 | `backend/internal/store/redis/redis.go` | Edit: implement arbitrage storage + funding Pub/Sub |
| 8 | `backend/internal/arbitrage/calculator.go` | New: computation engine |
| 9 | `backend/cmd/worker/main.go` | New: worker binary |
| 10 | `backend/internal/api/arbitrage.go` | New: read-only handler |
| 11 | `backend/internal/api/routes.go` | Edit: register new route |
| 12 | `backend/Makefile` | Edit: add worker target |
| 13 | `backend/deployments/docker-compose.yml` | Edit: add worker service |
| 14 | `frontend/lib/api.ts` | Edit: add types + fetch function |
| 15 | `frontend/app/arbitrage/arbitrage-table.tsx` | New: client component |
| 16 | `frontend/app/arbitrage/page.tsx` | Edit: render ArbitrageTable |

## Future Considerations

- **More exchanges**: Adding exchanges automatically increases arbitrage pair coverage. The calculator handles N*(N-1)/2 pairs per symbol.
- **Real fees**: Replace mock fee map with fees fetched from exchange APIs (CCXT provides `fetchTradingFees`).
- **Redis Streams**: Migrate from Pub/Sub to Streams for durable event delivery and consumer groups.
- **Spot + Short**: Second strategy type. Requires collecting spot market data alongside perpetual data.
- **Historical funding**: Track cumulative funding rates over 24h/7d for trend analysis.
- **Alerts**: Worker can emit alert events when APR exceeds a threshold.
