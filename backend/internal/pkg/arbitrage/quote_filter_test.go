package arbitrage

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

func filterTestTicker(sym string, src time.Time) domain.Ticker {
	return domain.Ticker{
		Symbol:     sym,
		Bid:        decimal.NewFromInt(100),
		Ask:        decimal.NewFromInt(101),
		SourceTime: src,
	}
}

func TestUsableTickers_WarnsOnMissingSourceTime(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	now := time.Now().UTC()
	p := domain.DefaultQualityPolicy()

	got := usableTickers(logger, "binance", []domain.Ticker{
		filterTestTicker("A", now),
		filterTestTicker("B", time.Time{}),
		filterTestTicker("C", now.Add(-time.Hour)),
	}, now, p)

	if len(got) != 1 || got[0].Symbol != "A" {
		t.Fatalf("usable = %+v, want only A", got)
	}
	out := buf.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "missing_source_time=1") ||
		!strings.Contains(out, "exchange=binance") {
		t.Errorf("expected a WARN summary naming missing source time, got:\n%s", out)
	}
	if !strings.Contains(out, "dropped stale tickers") {
		t.Errorf("expected stale drops to be logged, got:\n%s", out)
	}
}

func TestUsableTickers_StaleOnlyDoesNotWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	now := time.Now().UTC()

	got := usableTickers(logger, "bybit", []domain.Ticker{
		filterTestTicker("A", now.Add(-time.Hour)),
	}, now, domain.DefaultQualityPolicy())

	if len(got) != 0 {
		t.Fatalf("usable = %+v, want none", got)
	}
	if buf.Len() != 0 {
		t.Errorf("plain staleness must not warn, got:\n%s", buf.String())
	}
}
