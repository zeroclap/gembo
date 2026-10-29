# Current API to CLI inventory

C01 slice 1, 29 October 2026. Source: `3dc539cd125764709e288ce5d17f10725e505ea3`.

This source inventory covers every registration in [routes.go](../../backend/internal/service/api/routes.go). All 35 operations have implementation handlers and lack a supported product CLI. “Implemented-unverified” describes source presence; no operation was exercised in this slice. This is not the complete application capability registry or C01 acceptance.

The [machine inventory](api-inventory.json) preserves handler, source, authentication, proposed command, and follow-up package. IDs are preliminary until the complete registry is reviewed. Commands are proposals, not executable interfaces. C02 supplies the shared client foundation before feature command implementation.

| Current registration | Proposed command | Follow-up |
|---|---|---|
| `GET /v1/health` | `gembo status` | C03 |
| `POST /v1/auth/login` | `gembo auth login` | C03 |
| `GET /v1/exchanges` | `gembo exchanges list` | C03 |
| `GET /v1/balances` | `gembo balances list` | C03 |
| `GET /v1/tickers` | `gembo tickers list` | C03 |
| `GET /v1/exchanges/{exchange}/markets` | `gembo markets list` | C03 |
| `GET /v1/exchanges/{exchange}/markets/{symbol}` | `gembo markets get` | C03 |
| `GET /v1/exchanges/{exchange}/balances` | `gembo balances list --exchange` | C03 |
| `GET /v1/exchanges/{exchange}/tickers` | `gembo tickers list --exchange` | C03 |
| `GET /v1/exchanges/{exchange}/tickers/{symbol}` | `gembo tickers get` | C03 |
| `GET /v1/exchanges/{exchange}/funding-rates` | `gembo funding-rates list` | C03 |
| `GET /v1/exchanges/{exchange}/funding-rates/{symbol}` | `gembo funding-rates get` | C03 |
| `GET /v1/arbitrage/funding` | `gembo arbitrage funding list` | C03 |
| `POST /v1/arbitrage/funding/simulate` | `gembo arbitrage funding simulate` | C03 |
| `GET /v1/arbitrage/spread` | `gembo arbitrage spread list` | C03 |
| `POST /v1/arbitrage/spread/simulate` | `gembo arbitrage spread simulate` | C03 |
| `GET /v1/account` | `gembo account get` | C03 |
| `GET /v1/account/credentials` | `gembo credentials list` | C03 |
| `POST /v1/account/credentials` | `gembo credentials create` | C03 |
| `PUT /v1/account/credentials/{id}` | `gembo credentials update` | C03 |
| `DELETE /v1/account/credentials/{id}` | `gembo credentials delete` | C03 |
| `GET /v1/account/workspaces` | `gembo workspaces list` | C03 |
| `PUT /v1/account/workspaces` | `gembo workspaces replace` | C03 |
| `POST /v1/positions/preflight-open` | `gembo positions preflight` | C03 |
| `POST /v1/positions` | `gembo positions create` | C04 |
| `GET /v1/positions` | `gembo positions list` | C03 |
| `GET /v1/positions/{id}` | `gembo positions get` | C03 |
| `POST /v1/positions/{id}/close` | `gembo positions close` | C04 |
| `POST /v1/positions/{id}/size` | `gembo positions size` | C04 |
| `POST /v1/positions/{id}/margin` | `gembo positions margin` | C04 |
| `DELETE /v1/positions/{id}` | `gembo positions delete` | C04 |
| `GET /v1/positions/{id}/orders` | `gembo positions orders list` | C03 |
| `GET /v1/positions/{id}/funding` | `gembo positions funding list` | C03 |
| `/v1/arbitrage/ws` | `gembo arbitrage watch` | D07 |
| `DELETE /v1/flush` | `gembo admin flush` | D08 |

## Contract boundaries to preserve

- Current REST responses use `{data,error,ts}`; errors have `code` and `message`. The CLI protocol proposed in the [delivery contract](../delivery/06-cli-contract.md) is a separate versioned envelope and must not be presented as current HTTP behavior.
- Login and health are public. Other registrations pass through `requireAuth`, including WebSocket upgrade. The WebSocket route has no HTTP method constraint in ServeMux. A route comment claiming query authentication does not override middleware behavior.
- WebSocket subscribe data uses `symbol`, `exchange_a`, and `exchange_b`; outgoing messages use `type: arbitrage` and `data`. Snapshot and updates share that type. Durable replay, gap notices and CLI NDJSON are missing contracts, to implement and test in D07.
- Positions list accepts status and symbol filters. Create/preflight, simulation, credential, workspace, size and margin inputs require field-by-field schemas and semantic tests in the next C01 slice. The command rows do not establish option coverage.
- Workspace PUT replaces the saved collection; it does not supply individual CRUD, import/export, or conflict detection. Proposed `replace` preserves that distinction.
- Health currently establishes process response only. Administrative flush is authenticated but needs an explicit administrative authority design in D08. A command proposal grants no authority.
- Position mutations may trigger exchange execution through background processing. Contract review and inventory approval do not authorize running them.

## Remaining C01 deliverables

Complete these before claiming C01 or Phase 0 acceptance:

1. Inventory UI actions, client calculations and filters, saved state, domain outcomes, all specialist utilities, and background/admin workflows with stable IDs, owners, permissions, environment support and test links.
2. Classify mocked dashboard/P&L and transfer data separately from real API capabilities; inspect each relevant UI source. Track planned identity revocation, operation lifecycle, execution authority and capability discovery independently.
3. Design the complete command tree and meaningful options; map request/response fields and domain errors, including denied and uncertain outcomes.
4. Produce versioned OpenAPI, event and CLI machine schemas with dialect/version decisions and deterministic validation/drift checks. No schemas are delivered by this slice.
5. Produce a parity report over the full inventory, recording missing commands and scenarios. Current route coverage is 35/35 registrations; supported CLI coverage is 0/35 of these operations. This denominator excludes unreviewed non-route capabilities.

No application source, runtime configuration, dependencies or operational permissions change.
