"use client";

import { useEffect, useRef, useState } from "react";
import {
  createChart,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  type UTCTimestamp,
  ColorType,
} from "lightweight-charts";
import { useQueryClient } from "@tanstack/react-query";
import type { ArbitragePair } from "@/hooks/use-arbitrage-data";
import {
  useMarketSnapshots,
  computeLiveMetrics,
  type Resolution,
} from "@/hooks/use-market-snapshots";
import type { TickerSnapshot, TickerSnapshotBar } from "@/lib/api";
import { cn } from "@/lib/utils";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function barToLineData(bar: TickerSnapshotBar): LineData {
  return {
    time: bar.ts as UTCTimestamp,
    value: parseFloat(bar.close),
  };
}

// ---------------------------------------------------------------------------
// Single chart pane
// ---------------------------------------------------------------------------

interface ChartPaneProps {
  label: string;
  unit: string;
  bars: TickerSnapshotBar[];
  liveValue: number | null;
  liveTs: number | null;
  formatValue?: (v: number) => string;
}

function ChartPane({ label, unit, bars, liveValue, liveTs, formatValue }: ChartPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Line"> | null>(null);

  const fmt = formatValue ?? ((v: number) => v.toFixed(6));

  // Initialize chart once on mount.
  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: "transparent" },
        textColor: "hsl(var(--muted-foreground))",
        fontSize: 11,
      },
      grid: {
        vertLines: { color: "hsl(var(--border) / 0.3)" },
        horzLines: { color: "hsl(var(--border) / 0.3)" },
      },
      crosshair: { mode: 1 },
      rightPriceScale: { borderVisible: false },
      timeScale: { borderVisible: false, timeVisible: true, secondsVisible: false },
      handleScroll: true,
      handleScale: true,
    });

    const series = chart.addLineSeries({
      color: "hsl(var(--primary))",
      lineWidth: 2,
      priceLineVisible: false,
      lastValueVisible: true,
      crosshairMarkerVisible: true,
    });

    chartRef.current = chart;
    seriesRef.current = series;

    const observer = new ResizeObserver(() => {
      if (containerRef.current) {
        chart.applyOptions({ width: containerRef.current.clientWidth });
      }
    });
    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, []);

  // Sync historical bars whenever they change.
  useEffect(() => {
    if (!seriesRef.current || bars.length === 0) return;
    const data = bars.map(barToLineData);
    seriesRef.current.setData(data);
    chartRef.current?.timeScale().fitContent();
  }, [bars]);

  // Update the current (last) bar on each live value tick.
  useEffect(() => {
    if (!seriesRef.current || liveValue === null || liveTs === null) return;
    seriesRef.current.update({
      time: liveTs as UTCTimestamp,
      value: liveValue,
    });
  }, [liveValue, liveTs]);

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between px-1">
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
        {liveValue !== null && (
          <span className="text-xs font-mono text-foreground tabular-nums">
            {fmt(liveValue)} {unit}
          </span>
        )}
      </div>
      <div ref={containerRef} className="h-[160px] w-full" />
    </div>
  );
}

// ---------------------------------------------------------------------------
// ArbitrageCharts
// ---------------------------------------------------------------------------

interface ArbitrageChartsProps {
  pair: ArbitragePair | null;
}

const RESOLUTIONS: Resolution[] = ["1m", "5m"];

export function ArbitrageCharts({ pair }: ArbitrageChartsProps) {
  const [resolution, setResolution] = useState<Resolution>("1m");
  const queryClient = useQueryClient();

  // Read bars from React Query cache (populated by the WS hook).
  const { bars } = useMarketSnapshots(pair, resolution);

  // Re-render when live snapshots change so the panes receive updated values.
  const [, forceUpdate] = useState(0);

  useEffect(() => {
    if (!pair) return;

    const { symbol, longExchange, shortExchange } = pair;

    // Subscribe to React Query cache mutations for the relevant keys.
    const unsub = queryClient.getQueryCache().subscribe((event) => {
      const key = event.query.queryKey;
      if (!Array.isArray(key)) return;

      const isLiveSnap =
        key[0] === "ticker-snapshot-live" &&
        key[2] === symbol &&
        (key[1] === longExchange || key[1] === shortExchange);

      const isHistory =
        key[0] === "ticker-snapshot-history" && key[1] === symbol;

      if (isLiveSnap || isHistory) {
        forceUpdate((n) => n + 1);
      }
    });

    return () => unsub();
  }, [pair, queryClient]);

  // Compute live metrics from the two latest exchange snapshots.
  const snapLong = pair
    ? queryClient.getQueryData<TickerSnapshot>([
        "ticker-snapshot-live",
        pair.longExchange,
        pair.symbol,
      ]) ?? null
    : null;
  const snapShort = pair
    ? queryClient.getQueryData<TickerSnapshot>([
        "ticker-snapshot-live",
        pair.shortExchange,
        pair.symbol,
      ]) ?? null
    : null;

  const liveMetrics = computeLiveMetrics(snapLong, snapShort);

  const isEmpty = bars.netRate8h.length === 0 && bars.spreadPct.length === 0;

  return (
    <div className="rounded-lg border bg-card/30 p-3 flex flex-col gap-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
          Market History
        </span>
        <div className="flex items-center gap-0.5 rounded-md border bg-muted/30 p-0.5">
          {RESOLUTIONS.map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setResolution(r)}
              className={cn(
                "px-2 py-0.5 text-xs rounded transition-colors",
                resolution === r
                  ? "bg-background text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              {r}
            </button>
          ))}
        </div>
      </div>

      {isEmpty ? (
        <div className="flex items-center justify-center h-[80px] text-xs text-muted-foreground">
          Waiting for data…
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <ChartPane
            label="Net Funding Rate"
            unit="/ 8h"
            bars={bars.netRate8h}
            liveValue={liveMetrics.netRate8h}
            liveTs={liveMetrics.ts}
            formatValue={(v) => (v * 100).toFixed(4) + "%"}
          />
          <ChartPane
            label="Price Spread"
            unit="%"
            bars={bars.spreadPct}
            liveValue={liveMetrics.spreadPct}
            liveTs={liveMetrics.ts}
            formatValue={(v) => v.toFixed(4) + "%"}
          />
        </div>
      )}
    </div>
  );
}
