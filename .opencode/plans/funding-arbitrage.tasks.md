# Funding Arbitrage - Tasks

## Backend: Domain Layer

- [ ] Edit `internal/domain/funding_rate.go` -- add MarkPrice, IndexPrice, NextFundingRate, PreviousFundingRate, InterestRate fields (decimal.Decimal)
- [ ] Create `internal/domain/arbitrage.go` -- FundingArbitrage, ArbitrageLeg, ArbitrageFees, NextPaymentInfo, FullLoopInfo, LoopPayment structs

## Backend: Exchange Mappers

- [ ] Edit `internal/exchange/exchanges/bybit/mapper.go` -- extract 5 new fields from CCXT FundingRate struct in mapFundingRate()
- [ ] Edit `internal/exchange/exchanges/okx/mapper.go` -- same changes as bybit mapper

## Backend: Storage Layer

- [ ] Edit `pkg/rediskey/keys.go` -- add FundingRatePubSub(), FundingArbitragePair(), FundingArbitragePairs() functions
- [ ] Edit `internal/store/store.go` -- add SetFundingArbitrage, ListFundingArbitrage, RemoveFundingArbitrage to Store interface
- [ ] Edit `internal/store/redis/redis.go` -- implement arbitrage storage (SET + individual JSON keys pattern), add Pub/Sub publish to SetFundingRate (mirrors existing ticker pattern)

## Backend: Arbitrage Calculator

- [ ] Create `internal/arbitrage/calculator.go` -- Calculator struct with Recompute(ctx, symbol) method. Reads FR+ticker for all exchanges from store. Computes optimal direction, net rate, APR, spread, mock fees, full loop (LCM-based), next payment. Writes results back to store. Removes stale pairs.

## Backend: Worker Binary

- [ ] Create `cmd/worker/main.go` -- loads config, connects to Redis, creates store + calculator. Pattern-subscribes to gembo:*:ticker:updates and gembo:*:funding:updates. Parses each message for symbol. Calls calculator.Recompute(). Graceful shutdown via signal context.

## Backend: API

- [ ] Create `internal/api/arbitrage.go` -- handleFundingArbitrage handler. Reads from store via ListFundingArbitrage(). Sorts by APR descending. Returns via writeOK().
- [ ] Edit `internal/api/routes.go` -- register GET /v1/arbitrage/funding route

## Backend: Build & Deploy

- [ ] Edit `Makefile` -- add `worker` target (go build -o bin/worker ./cmd/worker), update `build` to include all 3 binaries
- [ ] Edit `deployments/docker-compose.yml` -- add worker service (same image, different entrypoint)

## Frontend: API Client

- [ ] Edit `lib/api.ts` -- add FundingArbitrage, ArbitrageLeg, ArbitrageFees, NextPaymentInfo, FullLoopInfo, LoopPayment TypeScript interfaces. Add fetchFundingArbitrage() function.

## Frontend: UI Components

- [ ] Create `app/arbitrage/arbitrage-table.tsx` -- TanStack React Table client component. Columns: Symbol, Long, Short, Net Rate 8h, APR, Net APR, Open Spread, Next Payment, Volume. Expandable sub-rows with full loop timeline, leg details, fee breakdown. Default sort APR desc. Search by symbol. Countdown timer for next payment.
- [ ] Edit `app/arbitrage/page.tsx` -- replace placeholder with ArbitrageTable component import and render

## Verification

- [ ] Run `make build` -- all 3 binaries compile
- [ ] Run `make test` -- all Go tests pass
- [ ] Run `make lint && make vet` -- clean
- [ ] Run `npm run build` (frontend) -- TypeScript compiles
- [ ] Manual test: run collector + worker + API, verify GET /v1/arbitrage/funding returns computed opportunities
