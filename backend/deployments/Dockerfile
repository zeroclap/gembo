# syntax=docker/dockerfile:1

# ---- deps: download modules and pre-compile the heavy ccxt dependency ----
# This layer is only invalidated when go.mod/go.sum change, not on source edits.
FROM golang:1.25-alpine AS deps

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# Pre-compile ccxt so it lands in the build cache before any source is copied.
# A throw-away main that imports the packages actually used by the exchange
# adapters is enough to trigger full compilation of the library.
RUN mkdir -p /tmp/warmup && cat > /tmp/warmup/main.go <<'EOF'
package main

import (
	_ "github.com/ccxt/ccxt/go/v4"
	_ "github.com/ccxt/ccxt/go/v4/pro"
)

func main() {}
EOF
RUN go build -v /tmp/warmup/main.go

# ---- build each binary in its own stage so they compile in parallel ----
FROM deps AS build-collector
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/collector ./cmd/collector

FROM deps AS build-api
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM deps AS build-worker
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM deps AS build-trader
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/trader ./cmd/trader

# ---- runtime images ----
FROM gcr.io/distroless/static-debian12 AS collector
COPY --from=build-collector /out/collector /collector
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/collector"]

FROM gcr.io/distroless/static-debian12 AS api
COPY --from=build-api /out/api /api
COPY configs/config.yaml /configs/config.yaml
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM gcr.io/distroless/static-debian12 AS worker
COPY --from=build-worker /out/worker /worker
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/worker"]

FROM gcr.io/distroless/static-debian12 AS trader
COPY --from=build-trader /out/trader /trader
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/trader"]
