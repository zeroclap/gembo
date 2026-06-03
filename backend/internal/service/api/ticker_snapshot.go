package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// snapshotResolution maps resolution string to bucket size in seconds.
var snapshotResolution = map[string]int64{
	"1m": 60,
	"5m": 300,
}

// snapshotBars holds OHLC bar series for the two cross-exchange derived metrics.
type snapshotBars struct {
	NetRate8h []domain.TickerSnapshotBar `json:"net_rate_8h"`
	SpreadPct []domain.TickerSnapshotBar `json:"spread_pct"`
}

// handleGetArbitrageChart returns OHLC bars for a cross-exchange pair over the
// last 1h. The resolution parameter controls the bucket size.
//
// GET /v1/arbitrage/chart?symbol=BTC%2FUSDT%3AUSDT&exchange_a=bybit&exchange_b=okx&resolution=1m
func (h *Handler) handleGetArbitrageChart(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	exA := r.URL.Query().Get("exchange_a")
	exB := r.URL.Query().Get("exchange_b")
	resStr := r.URL.Query().Get("resolution")

	if symbol == "" || exA == "" || exB == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "symbol, exchange_a and exchange_b are required")
		return
	}

	bucketSecs, ok := snapshotResolution[resStr]
	if !ok {
		bucketSecs = 60
		resStr = "1m"
	}

	now := time.Now().Unix()
	from := now - 3600

	ctx := r.Context()
	snapsA, err := h.store.GetTickerSnapshots(ctx, exA, symbol, from, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	snapsB, err := h.store.GetTickerSnapshots(ctx, exB, symbol, from, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	bars := buildSnapshotBars(snapsA, snapsB, bucketSecs)
	writeOK(w, struct {
		Resolution string       `json:"resolution"`
		Bars       snapshotBars `json:"bars"`
	}{Resolution: resStr, Bars: bars})
}

// buildSnapshotBars aligns two per-exchange snapshot series and computes OHLC
// bars for net_rate_8h and spread_pct over bucketSecs-wide time buckets.
// snapsA and snapsB must be ordered by TS ascending (as returned by Redis).
func buildSnapshotBars(snapsA, snapsB []domain.TickerSnapshot, bucketSecs int64) snapshotBars {
	// Index snapsB into a bucket map for fast lookup when iterating snapsA.
	type bEntry struct{ snaps []domain.TickerSnapshot }
	bBuckets := make(map[int64]*bEntry, len(snapsB)/6+1)
	for _, s := range snapsB {
		bk := bucketStart(s.TS, bucketSecs)
		if bBuckets[bk] == nil {
			bBuckets[bk] = &bEntry{}
		}
		bBuckets[bk].snaps = append(bBuckets[bk].snaps, s)
	}

	// Group snapsA by bucket, then for each snapshot in the bucket find the
	// temporally closest snapshot from B, compute derived metrics, build OHLC.
	type bucketAccum struct {
		rateVals   []decimal.Decimal
		spreadVals []decimal.Decimal
	}
	buckets := make(map[int64]*bucketAccum, len(snapsA)/6+1)
	bucketOrder := make([]int64, 0)

	for _, a := range snapsA {
		bk := bucketStart(a.TS, bucketSecs)
		be, ok := bBuckets[bk]
		if !ok || len(be.snaps) == 0 {
			continue
		}
		b := closestSnapshot(be.snaps, a.TS)

		rate := a.Rate8h.Sub(b.Rate8h).Abs()

		mid := a.Price.Add(b.Price)
		var spread decimal.Decimal
		if !mid.IsZero() {
			// spread = (ask_a - bid_b) / mid * 100  (directional: a is long leg)
			spread = a.Ask.Sub(b.Bid).Div(mid.Div(decimal.NewFromInt(2))).Mul(decimal.NewFromInt(100))
		}

		acc := buckets[bk]
		if acc == nil {
			acc = &bucketAccum{}
			buckets[bk] = acc
			bucketOrder = append(bucketOrder, bk)
		}
		acc.rateVals = append(acc.rateVals, rate)
		acc.spreadVals = append(acc.spreadVals, spread)
	}

	sort.Slice(bucketOrder, func(i, j int) bool { return bucketOrder[i] < bucketOrder[j] })

	rateBars := make([]domain.TickerSnapshotBar, 0, len(bucketOrder))
	spreadBars := make([]domain.TickerSnapshotBar, 0, len(bucketOrder))

	for _, bk := range bucketOrder {
		acc := buckets[bk]
		rateBars = append(rateBars, toOHLC(bk, acc.rateVals))
		spreadBars = append(spreadBars, toOHLC(bk, acc.spreadVals))
	}

	return snapshotBars{NetRate8h: rateBars, SpreadPct: spreadBars}
}

// buildSnapshotHistory fetches snapshots for both legs and returns bars for all
// supported resolutions. Used by the WebSocket handler on subscribe.
func buildSnapshotHistory(ctx context.Context, h *Handler, exA, exB, symbol string) map[string]snapshotBars {
	now := time.Now().Unix()
	from := now - 3600

	snapsA, err := h.store.GetTickerSnapshots(ctx, exA, symbol, from, now)
	if err != nil || len(snapsA) == 0 {
		return nil
	}
	snapsB, err := h.store.GetTickerSnapshots(ctx, exB, symbol, from, now)
	if err != nil || len(snapsB) == 0 {
		return nil
	}

	result := make(map[string]snapshotBars, len(snapshotResolution))
	for res, secs := range snapshotResolution {
		result[res] = buildSnapshotBars(snapsA, snapsB, secs)
	}
	return result
}

// bucketStart floors ts to the nearest multiple of bucketSecs.
func bucketStart(ts, bucketSecs int64) int64 {
	return (ts / bucketSecs) * bucketSecs
}

// closestSnapshot returns the snapshot from snaps whose TS is closest to ts.
func closestSnapshot(snaps []domain.TickerSnapshot, ts int64) domain.TickerSnapshot {
	best := snaps[0]
	bestDiff := absDiff(snaps[0].TS, ts)
	for _, s := range snaps[1:] {
		if d := absDiff(s.TS, ts); d < bestDiff {
			best = s
			bestDiff = d
		}
	}
	return best
}

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

// toOHLC builds a TickerSnapshotBar from an ordered slice of values.
func toOHLC(ts int64, vals []decimal.Decimal) domain.TickerSnapshotBar {
	bar := domain.TickerSnapshotBar{
		TS:    ts,
		Open:  vals[0],
		Close: vals[len(vals)-1],
		High:  vals[0],
		Low:   vals[0],
	}
	for _, v := range vals[1:] {
		if v.GreaterThan(bar.High) {
			bar.High = v
		}
		if v.LessThan(bar.Low) {
			bar.Low = v
		}
	}
	return bar
}
