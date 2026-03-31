# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/collector ./cmd/collector && \
    go build -trimpath -ldflags="-s -w" \
    -o /out/api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" \
    -o /out/worker ./cmd/worker

# ---- collector image ----
FROM gcr.io/distroless/static-debian12 AS collector

COPY --from=builder /out/collector /collector
COPY configs/config.example.yaml /configs/config.yaml

ENTRYPOINT ["/collector"]

# ---- api image ----
FROM gcr.io/distroless/static-debian12 AS api

COPY --from=builder /out/api /api
COPY configs/config.example.yaml /configs/config.yaml

EXPOSE 8080
ENTRYPOINT ["/api"]

# ---- worker image ----
FROM gcr.io/distroless/static-debian12 AS worker

COPY --from=builder /out/worker /worker
COPY configs/config.example.yaml /configs/config.yaml

ENTRYPOINT ["/worker"]
