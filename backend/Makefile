.PHONY: build collector api worker run-collector run-api run-worker tidy lint test clean

BIN_DIR := bin

build: collector api worker

collector:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/collector ./cmd/collector

api:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/api ./cmd/api

worker:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/worker ./cmd/worker

run-collector: collector
	./$(BIN_DIR)/collector

run-api: api
	./$(BIN_DIR)/api

run-worker: worker
	./$(BIN_DIR)/worker

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

docker-build:
	docker build -f deployments/Dockerfile --target collector -t gembo-collector:latest .
	docker build -f deployments/Dockerfile --target api      -t gembo-api:latest .
	docker build -f deployments/Dockerfile --target worker   -t gembo-worker:latest .

docker-up:
	docker compose -f deployments/docker-compose.yml up --build

docker-down:
	docker compose -f deployments/docker-compose.yml down
