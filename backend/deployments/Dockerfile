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
FROM deps AS build-market-data-collector
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/market-data-collector ./cmd/market-data-collector

FROM deps AS build-account-data-collector
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/account-data-collector ./cmd/account-data-collector

FROM deps AS build-api
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM deps AS build-arbitrage-worker
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/arbitrage-worker ./cmd/arbitrage-worker

FROM deps AS build-position-manager
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/position-manager ./cmd/position-manager

# exchange-probe is an operator CLI, not a long-running service, but it still
# needs a container image to run as the non-US egress CI job (issue #4).
FROM deps AS build-exchange-probe
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/exchange-probe ./cmd/exchange-probe

# ---- runtime images ----
FROM gcr.io/distroless/static-debian12 AS market-data-collector
COPY --from=build-market-data-collector /out/market-data-collector /market-data-collector
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/market-data-collector"]

FROM gcr.io/distroless/static-debian12 AS account-data-collector
COPY --from=build-account-data-collector /out/account-data-collector /account-data-collector
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/account-data-collector"]

FROM gcr.io/distroless/static-debian12 AS api
COPY --from=build-api /out/api /api
COPY configs/config.yaml /configs/config.yaml
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM gcr.io/distroless/static-debian12 AS arbitrage-worker
COPY --from=build-arbitrage-worker /out/arbitrage-worker /arbitrage-worker
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/arbitrage-worker"]

FROM gcr.io/distroless/static-debian12 AS position-manager
COPY --from=build-position-manager /out/position-manager /position-manager
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/position-manager"]

# Runs as the non-US egress CI job (issue #4). No exchange or account
# credentials, no production access — public endpoints only.
FROM gcr.io/distroless/static-debian12 AS exchange-probe
COPY --from=build-exchange-probe /out/exchange-probe /exchange-probe
COPY configs/config.yaml /configs/config.yaml
ENTRYPOINT ["/exchange-probe"]
