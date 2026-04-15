.PHONY: build collector api worker trader \
        run-collector run-api run-worker run-trader \
        tidy lint vet test clean \
        docker-build \
        up down

BIN_DIR  := bin
COMPOSE  := deployments/docker-compose.yml

# ---- local build -------------------------------------------------------

build: collector api worker trader

collector:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/collector ./cmd/collector

api:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/api ./cmd/api

worker:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/worker ./cmd/worker

trader:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/trader ./cmd/trader

run-collector: collector
	./$(BIN_DIR)/collector

run-api: api
	./$(BIN_DIR)/api

run-worker: worker
	./$(BIN_DIR)/worker

run-trader: trader
	./$(BIN_DIR)/trader

# ---- quality -----------------------------------------------------------

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BIN_DIR)

# ---- docker (production/CI) --------------------------------------------

docker-build:
	docker build -f deployments/Dockerfile --target collector -t gembo-collector:latest .
	docker build -f deployments/Dockerfile --target api       -t gembo-api:latest .
	docker build -f deployments/Dockerfile --target worker    -t gembo-worker:latest .
	docker build -f deployments/Dockerfile --target trader    -t gembo-trader:latest .

# ---- dev: full stack (infra + services) --------------------------------

docker-up:
	docker compose -f $(COMPOSE) up -d --wait

docker-down:
	docker compose -f $(COMPOSE) down
