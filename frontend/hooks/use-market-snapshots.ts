"use client";

import { useQueryClient } from "@tanstack/react-query";
import type { ArbitragePair } from "@/hooks/use-arbitrage-data";
import type { TickerSnapshot, TickerSnapshotBar, SnapshotBars, TickerSnapshotHistory } from "@/lib/api";

export type Resolution = "1m" | "5m";

export interface MarketSnapshotBars {
  netRate8h: TickerSnapshotBar[];
  spreadPct: TickerSnapshotBar[];
}

export interface LiveSnapshotMetrics {
  /** Absolute difference of normalized funding rates between the two legs. */
  netRate8h: number | null;
  /** Spread % between the long leg ask and short leg bid. */
  spreadPct: number | null;
  /** Unix timestamp of the latest snapshot used for these metrics. */
  ts: number | null;
}

/**
 * useMarketSnapshots reads pre-built OHLC history bars (seeded by the WS hook
 * on subscribe) and the latest per-exchange live snapshots from React Query
 * cache. The chart component drives its visual updates from these values.
 *
 * Cross-exchange derived metrics are computed here, not stored — the cache
 * holds raw per-exchange TickerSnapshot values.
 */
export function useMarketSnapshots(
  pair: ArbitragePair | null,
  resolution: Resolution
): { bars: MarketSnapshotBars; live: LiveSnapshotMetrics } {
  const queryClient = useQueryClient();

  const empty: MarketSnapshotBars = { netRate8h: [], spreadPct: [] };

  if (!pair) {
    return { bars: empty, live: { netRate8h: null, spreadPct: null, ts: null } };
  }

  // History bars — set by the WS hook when "ticker_snapshot_history" arrives.
  const history = queryClient.getQueryData<TickerSnapshotHistory>([
    "ticker-snapshot-history",
    pair.symbol,
    pair.longExchange,
    pair.shortExchange,
  ]);

  const resBars: SnapshotBars | undefined = history?.[resolution];
  const bars: MarketSnapshotBars = resBars
    ? { netRate8h: resBars.net_rate_8h, spreadPct: resBars.spread_pct }
    : empty;

  // Live snapshots — updated by the WS hook on each "ticker_snapshot" message.
  const snapA = queryClient.getQueryData<TickerSnapshot>([
    "ticker-snapshot-live",
    pair.longExchange,
    pair.symbol,
  ]);
  const snapB = queryClient.getQueryData<TickerSnapshot>([
    "ticker-snapshot-live",
    pair.shortExchange,
    pair.symbol,
  ]);

  const live = computeLiveMetrics(snapA ?? null, snapB ?? null);

  return { bars, live };
}

/**
 * computeLiveMetrics derives cross-exchange metrics from the two latest
 * per-exchange snapshots. Called by the chart component on each render to
 * update the current (last) bar imperatively via series.update().
 */
export function computeLiveMetrics(
  snapLong: TickerSnapshot | null,
  snapShort: TickerSnapshot | null
): LiveSnapshotMetrics {
  if (!snapLong || !snapShort) {
    return { netRate8h: null, spreadPct: null, ts: null };
  }

  const rateA = parseFloat(snapLong.rate_8h);
  const rateB = parseFloat(snapShort.rate_8h);
  const netRate8h = Math.abs(rateA - rateB);

  const askLong = parseFloat(snapLong.ask);
  const bidShort = parseFloat(snapShort.bid);
  const midA = parseFloat(snapLong.price);
  const midB = parseFloat(snapShort.price);
  const mid = (midA + midB) / 2;
  const spreadPct = mid !== 0 ? ((askLong - bidShort) / mid) * 100 : 0;

  const ts = Math.max(snapLong.ts, snapShort.ts);

  return { netRate8h, spreadPct, ts };
}
