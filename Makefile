.PHONY: build collector api run-collector run-api tidy lint test clean

BIN_DIR := bin

build: collector api

collector:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/collector ./cmd/collector

api:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/api ./cmd/api

run-collector: collector
	./$(BIN_DIR)/collector

run-api: api
	./$(BIN_DIR)/api

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

docker-up:
	docker compose -f deployments/docker-compose.yml up --build

docker-down:
	docker compose -f deployments/docker-compose.yml down
