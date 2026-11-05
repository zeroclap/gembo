.PHONY: build market-data-collector account-data-collector api arbitrage-worker position-manager \
        run-market-data-collector run-account-data-collector run-api run-arbitrage-worker run-position-manager \
        tidy lint vet test test-integration clean \
        docker-build docker-build-ci-egress-probe \
        up down

BIN_DIR  := bin
COMPOSE  := deployments/docker-compose.yml

# ---- local build -------------------------------------------------------

build: market-data-collector account-data-collector api arbitrage-worker position-manager

market-data-collector:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/market-data-collector ./cmd/market-data-collector

account-data-collector:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/account-data-collector ./cmd/account-data-collector

api:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/api ./cmd/api

arbitrage-worker:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/arbitrage-worker ./cmd/arbitrage-worker

position-manager:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/position-manager ./cmd/position-manager

run-market-data-collector: market-data-collector
	./$(BIN_DIR)/market-data-collector

run-account-data-collector: account-data-collector
	./$(BIN_DIR)/account-data-collector

run-api: api
	./$(BIN_DIR)/api

run-arbitrage-worker: arbitrage-worker
	./$(BIN_DIR)/arbitrage-worker

run-position-manager: position-manager
	./$(BIN_DIR)/position-manager

# ---- quality -----------------------------------------------------------

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

# Calls real exchange public endpoints; requires non-US network egress
# (Bybit/Binance geo-block US IPs, issue #4). Never run by `make test` or
# plain `go test ./...`.
test-integration:
	go test -tags integration -v ./internal/exchanges/bybit/...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BIN_DIR)

# ---- docker (production/CI) --------------------------------------------

docker-build:
	docker build -f deployments/Dockerfile --target market-data-collector  -t gembo-market-data-collector:latest .
	docker build -f deployments/Dockerfile --target account-data-collector -t gembo-account-data-collector:latest .
	docker build -f deployments/Dockerfile --target api                    -t gembo-api:latest .
	docker build -f deployments/Dockerfile --target arbitrage-worker       -t gembo-arbitrage-worker:latest .
	docker build -f deployments/Dockerfile --target position-manager       -t gembo-position-manager:latest .

# Used only by the non-US egress CI job (infra/ci-egress/, issue #4), which
# runs exchange-probe as a Cloud Run Job. Not part of docker-build/docker-up.
docker-build-ci-egress-probe:
	docker build --platform=linux/amd64 -f deployments/Dockerfile --target exchange-probe -t gembo-exchange-probe:latest .

# ---- dev: full stack (infra + services) --------------------------------

docker-up:
	docker compose -f $(COMPOSE) up -d --wait

docker-down:
	docker compose -f $(COMPOSE) down
