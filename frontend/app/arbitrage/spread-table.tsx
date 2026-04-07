"use client";

import { useQuery } from "@tanstack/react-query";
import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  fetchSpreadArbitrage,
  simulateSpreadArbitrage,
  type SpreadArbitrage,
  type SpreadSimulationResult,
} from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import {
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  X,
  Loader2,
  TrendingUp,
  TrendingDown,
  SlidersHorizontal,
  ChevronDown,
  ChevronUp,
  RotateCcw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

function formatPct(value: string, decimals = 4): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(decimals)}%`;
}

function formatPrice(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "-";
  if (num >= 1)
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  if (num >= 0.01)
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 4,
      maximumFractionDigits: 4,
    });
  return num.toLocaleString(undefined, {
    minimumFractionDigits: 6,
    maximumFractionDigits: 8,
  });
}

function formatVolume(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  if (num >= 1_000_000_000) return `${(num / 1_000_000_000).toFixed(2)}B`;
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(2)}M`;
  if (num >= 1_000) return `${(num / 1_000).toFixed(2)}K`;
  return num.toFixed(2);
}

function formatUsd(value: string): string {
  const num = parseFloat(value);
  if (!isFinite(num)) return "-";
  const abs = Math.abs(num);
  const sign = num >= 0 ? "+" : "-";
  if (abs >= 1000) {
    return `${sign}$${abs.toLocaleString(undefined, { minimumFractionDigits: 4, maximumFractionDigits: 4 })}`;
  }
  return `${sign}$${abs.toFixed(4)}`;
}

function formatUsdPlain(value: string): string {
  const num = parseFloat(value);
  if (!isFinite(num)) return "-";
  const abs = Math.abs(num);
  if (abs >= 1000) {
    return `$${abs.toLocaleString(undefined, { minimumFractionDigits: 4, maximumFractionDigits: 4 })}`;
  }
  return `$${abs.toFixed(4)}`;
}

// ---------------------------------------------------------------------------
// Small components
// ---------------------------------------------------------------------------

function ExchangeBadge({ exchangeId }: { exchangeId: string }) {
  const colorMap: Record<string, string> = {
    bybit: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
    okx: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
    gate: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
    binance: "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400",
  };

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium uppercase tracking-wider",
        colorMap[exchangeId] ?? "bg-muted text-muted-foreground"
      )}
    >
      {exchangeId}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Slug encoding for spread arb
// ---------------------------------------------------------------------------

export function encodeSpreadSlug(
  symbol: string,
  buyEx: string,
  sellEx: string
): string {
  return `${encodeURIComponent(symbol)}--${encodeURIComponent(buyEx)}--${encodeURIComponent(sellEx)}`;
}

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

type SortKey = "symbol" | "spread_pct" | "net_spread_pct" | "volume";

interface SortState {
  key: SortKey;
  desc: boolean;
}

function getSortValue(
  arb: SpreadArbitrage,
  key: SortKey
): number | string {
  switch (key) {
    case "symbol":
      return arb.symbol;
    case "spread_pct":
      return parseFloat(arb.spread_pct) || 0;
    case "net_spread_pct":
      return parseFloat(arb.net_spread_pct) || 0;
    case "volume":
      return Math.min(
        parseFloat(arb.buy_volume_24h) || 0,
        parseFloat(arb.sell_volume_24h) || 0
      );
  }
}

function sortData(
  data: SpreadArbitrage[],
  sort: SortState
): SpreadArbitrage[] {
  const sorted = [...data].sort((a, b) => {
    const va = getSortValue(a, sort.key);
    const vb = getSortValue(b, sort.key);
    if (typeof va === "string" && typeof vb === "string") {
      return va.localeCompare(vb);
    }
    return (va as number) - (vb as number);
  });
  if (sort.desc) sorted.reverse();
  return sorted;
}

function SortButton({
  label,
  sortKey,
  current,
  onToggle,
}: {
  label: string;
  sortKey: SortKey;
  current: SortState;
  onToggle: (key: SortKey) => void;
}) {
  const active = current.key === sortKey;
  return (
    <button
      className="flex items-center font-medium"
      onClick={() => onToggle(sortKey)}
    >
      {label}
      {active && current.desc ? (
        <ArrowDown className="ml-1 h-3 w-3" />
      ) : active && !current.desc ? (
        <ArrowUp className="ml-1 h-3 w-3" />
      ) : (
        <ArrowUpDown className="ml-1 h-3 w-3 opacity-30" />
      )}
    </button>
  );
}

// ---------------------------------------------------------------------------
// Filtering
// ---------------------------------------------------------------------------

interface SpreadFilters {
  symbols: string[];
  exchanges: string[];
  netSpreadMin: string;
  netSpreadMax: string;
}

const EMPTY_FILTERS: SpreadFilters = {
  symbols: [],
  exchanges: [],
  netSpreadMin: "",
  netSpreadMax: "",
};

function filterData(
  data: SpreadArbitrage[],
  filters: SpreadFilters
): SpreadArbitrage[] {
  return data.filter((arb) => {
    if (filters.symbols.length > 0) {
      const sym = arb.symbol.toLowerCase();
      if (!filters.symbols.some((tag) => sym.includes(tag.toLowerCase()))) {
        return false;
      }
    }
    if (filters.exchanges.length > 0) {
      const exLower = filters.exchanges.map((e) => e.toLowerCase());
      const buyEx = arb.buy_exchange_id.toLowerCase();
      const sellEx = arb.sell_exchange_id.toLowerCase();
      if (!exLower.some((e) => buyEx === e || sellEx === e)) {
        return false;
      }
    }
    if (filters.netSpreadMin) {
      const min = parseFloat(filters.netSpreadMin);
      if (!isNaN(min) && parseFloat(arb.net_spread_pct) < min) return false;
    }
    if (filters.netSpreadMax) {
      const max = parseFloat(filters.netSpreadMax);
      if (!isNaN(max) && parseFloat(arb.net_spread_pct) > max) return false;
    }
    return true;
  });
}

function countActiveFilters(f: SpreadFilters): number {
  let count = 0;
  if (f.symbols.length > 0) count++;
  if (f.exchanges.length > 0) count++;
  if (f.netSpreadMin) count++;
  if (f.netSpreadMax) count++;
  return count;
}

// ---------------------------------------------------------------------------
// Data row
// ---------------------------------------------------------------------------

function DataRowPair({
  arb,
  isSelected,
  onSelect,
}: {
  arb: SpreadArbitrage;
  isSelected: boolean;
  onSelect: () => void;
}) {
  const spreadPct = parseFloat(arb.spread_pct);
  const netPct = parseFloat(arb.net_spread_pct);

  return (
    <div
      className={cn(
        "group/row cursor-pointer border-b transition-colors hover:bg-muted/50 text-[11px] leading-tight",
        isSelected && "bg-violet-500/5 dark:bg-violet-500/10 border-violet-500/20"
      )}
      onClick={onSelect}
    >
      <div className="flex">
        {/* Col 1: Symbol */}
        <div className="flex items-center px-2 py-1 min-w-[90px] flex-[1]">
          <div className="flex items-center gap-1 whitespace-nowrap font-medium text-xs">
            {isSelected && (
              <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />
            )}
            {arb.symbol}
          </div>
        </div>

        {/* Col 2: Buy/Sell Pair */}
        <div className="flex-[3] min-w-0 flex flex-col justify-center gap-0.5 py-0.5">
          {/* Buy leg */}
          <div className="flex">
            <div className="flex-[1.5] min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
                <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                  <ArrowUp className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
                </span>
                <span className="text-[0.5625rem] text-muted-foreground uppercase">buy</span>
                <ExchangeBadge exchangeId={arb.buy_exchange_id} />
                <span className="flex-1" />
                <span className="font-mono tabular-nums text-muted-foreground">
                  {formatPrice(arb.buy_ask)}
                </span>
              </div>
            </div>
            <div className="flex-[1] min-w-0 px-2 whitespace-nowrap text-right">
              <span className="font-mono tabular-nums text-muted-foreground text-[10px]">
                vol {formatVolume(arb.buy_volume_24h)}
              </span>
            </div>
          </div>
          {/* Sell leg */}
          <div className="flex">
            <div className="flex-[1.5] min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
                <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                  <ArrowDown className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
                </span>
                <span className="text-[0.5625rem] text-muted-foreground uppercase">sell</span>
                <ExchangeBadge exchangeId={arb.sell_exchange_id} />
                <span className="flex-1" />
                <span className="font-mono tabular-nums text-muted-foreground">
                  {formatPrice(arb.sell_bid)}
                </span>
              </div>
            </div>
            <div className="flex-[1] min-w-0 px-2 whitespace-nowrap text-right">
              <span className="font-mono tabular-nums text-muted-foreground text-[10px]">
                vol {formatVolume(arb.sell_volume_24h)}
              </span>
            </div>
          </div>
        </div>

        {/* Col 3: Spread */}
        <div className="flex items-center px-2 py-0.5 min-w-[130px] flex-[1.5] whitespace-nowrap">
          <div className="flex flex-col leading-[1.15rem]">
            <div className="flex items-center gap-1">
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider w-10">
                gross
              </span>
              <span
                className={cn(
                  "font-mono tabular-nums",
                  spreadPct >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {formatPct(arb.spread_pct)}
              </span>
            </div>
            <div className="flex items-center gap-1">
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider w-10">
                net
              </span>
              <span
                className={cn(
                  "font-mono tabular-nums font-medium",
                  netPct >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {formatPct(arb.net_spread_pct)}
              </span>
            </div>
          </div>
        </div>

        {/* Col 4: Spread Abs */}
        <div className="flex items-center px-2 py-0.5 min-w-[80px] flex-[0.8] whitespace-nowrap">
          <span className="font-mono tabular-nums text-muted-foreground">
            ${formatPrice(arb.spread_abs)}
          </span>
        </div>

        {/* Col 5: Fees */}
        <div className="flex items-center px-2 py-0.5 min-w-[80px] flex-[0.8] whitespace-nowrap">
          <span className="font-mono tabular-nums text-muted-foreground">
            {formatPct(arb.fees.round_trip_fees_pct)}
          </span>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Simulation presets
// ---------------------------------------------------------------------------

const SIZE_PRESETS = [
  { label: "$100", value: "100" },
  { label: "$1K", value: "1000" },
  { label: "$5K", value: "5000" },
  { label: "$10K", value: "10000" },
  { label: "$50K", value: "50000" },
];

const TARGET_PRESETS = [
  { label: "0%", value: "0" },
  { label: "0.05%", value: "0.05" },
  { label: "0.1%", value: "0.1" },
  { label: "0.2%", value: "0.2" },
  { label: "0.5%", value: "0.5" },
];

// ---------------------------------------------------------------------------
// Detail panel
// ---------------------------------------------------------------------------

function DetailPanel({
  arb,
  onClose,
  onNavigate,
}: {
  arb: SpreadArbitrage;
  onClose: () => void;
  onNavigate: () => void;
}) {
  const [positionSize, setPositionSize] = useState("1000");
  const [targetSpreadPct, setTargetSpreadPct] = useState("0");
  const [slippagePct, setSlippagePct] = useState("0.05");
  const [simResult, setSimResult] = useState<SpreadSimulationResult | null>(null);
  const [simLoading, setSimLoading] = useState(false);
  const [simError, setSimError] = useState<string | null>(null);

  const runSimulation = useCallback(async () => {
    const size = parseFloat(positionSize);
    if (!size || size <= 0) return;

    setSimLoading(true);
    setSimError(null);
    try {
      const result = await simulateSpreadArbitrage({
        symbol: arb.symbol,
        buy_exchange_id: arb.buy_exchange_id,
        sell_exchange_id: arb.sell_exchange_id,
        position_size: positionSize,
        target_spread_pct: targetSpreadPct || "0",
        slippage_pct: slippagePct || "0",
      });
      setSimResult(result);
    } catch (err) {
      setSimError(err instanceof Error ? err.message : "Simulation failed");
    } finally {
      setSimLoading(false);
    }
  }, [arb, positionSize, targetSpreadPct, slippagePct]);

  // Auto-trigger simulation when the panel opens.
  const didAutoRun = useRef(false);
  useEffect(() => {
    if (!didAutoRun.current) {
      didAutoRun.current = true;
      runSimulation();
    }
  }, [runSimulation]);

  const detailRows: {
    label: string;
    buy: React.ReactNode;
    sell: React.ReactNode;
  }[] = [
    {
      label: "Exchange",
      buy: <ExchangeBadge exchangeId={arb.buy_exchange_id} />,
      sell: <ExchangeBadge exchangeId={arb.sell_exchange_id} />,
    },
    {
      label: "Price",
      buy: formatPrice(arb.buy_ask),
      sell: formatPrice(arb.sell_bid),
    },
    {
      label: "Volume 24h",
      buy: formatVolume(arb.buy_volume_24h),
      sell: formatVolume(arb.sell_volume_24h),
    },
    {
      label: "Taker fee",
      buy: `${arb.fees.buy_taker_fee_pct}%`,
      sell: `${arb.fees.sell_taker_fee_pct}%`,
    },
  ];

  return (
    <div className="shrink-0 border-l flex flex-col overflow-y-auto min-h-0 h-full w-full">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-3">
        <div>
          <div className="font-medium">{arb.symbol}</div>
          <div className="text-xs text-muted-foreground flex items-center gap-1.5">
            <span className="inline-flex items-center gap-1">
              <TrendingUp className="h-3 w-3 text-emerald-500" />
              <span className="text-[0.5625rem] text-muted-foreground">buy</span>
              <ExchangeBadge exchangeId={arb.buy_exchange_id} />
            </span>
            <span className="text-muted-foreground/50">/</span>
            <span className="inline-flex items-center gap-1">
              <TrendingDown className="h-3 w-3 text-red-500" />
              <span className="text-[0.5625rem] text-muted-foreground">sell</span>
              <ExchangeBadge exchangeId={arb.sell_exchange_id} />
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={(e) => {
              e.stopPropagation();
              onNavigate();
            }}
          >
            Open Position
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <Tabs defaultValue="details" className="flex-1 min-h-0 gap-0">
        <TabsList className="mx-4 mt-3">
          <TabsTrigger value="details">Details</TabsTrigger>
          <TabsTrigger value="simulation">Simulation</TabsTrigger>
        </TabsList>

        {/* Details tab */}
        <TabsContent value="details" className="flex-1 min-h-0 overflow-y-auto">
          <div className="p-4 space-y-5">
            {/* Comparison table */}
            <div>
              <table className="text-xs w-full">
                <thead>
                  <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    <th className="pr-4 pb-1 text-left font-medium" />
                    <th className="pr-4 pb-1 text-right font-medium">
                      <span className="inline-flex items-center gap-1">
                        <TrendingUp className="h-3 w-3 text-emerald-500" />
                        Buy
                      </span>
                    </th>
                    <th className="pb-1 text-right font-medium">
                      <span className="inline-flex items-center gap-1">
                        <TrendingDown className="h-3 w-3 text-red-500" />
                        Sell
                      </span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {detailRows.map((row) => (
                    <tr key={row.label} className="border-t border-border/30">
                      <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                        {row.label}
                      </td>
                      <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                        {row.buy}
                      </td>
                      <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                        {row.sell}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Spread info */}
            <div>
              <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
                Spread
              </div>
              <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <span className="text-muted-foreground">Mid price</span>
                <span className="font-mono tabular-nums text-right">
                  ${formatPrice(arb.mid_price)}
                </span>
                <span className="text-muted-foreground">Spread (abs)</span>
                <span className="font-mono tabular-nums text-right">
                  ${formatPrice(arb.spread_abs)}
                </span>
                <span className="text-muted-foreground">Spread (%)</span>
                <span
                  className={cn(
                    "font-mono tabular-nums text-right",
                    parseFloat(arb.spread_pct) >= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatPct(arb.spread_pct)}
                </span>
              </div>
            </div>

            {/* Fee breakdown */}
            <div>
              <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
                Fees
              </div>
              <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <span className="text-muted-foreground">Open</span>
                <span className="font-mono tabular-nums text-right">
                  {formatPct(arb.fees.open_fees_pct)}
                </span>
                <span className="text-muted-foreground">Close</span>
                <span className="font-mono tabular-nums text-right">
                  {formatPct(arb.fees.close_fees_pct)}
                </span>
                <span className="text-muted-foreground">Round-trip</span>
                <span className="font-mono tabular-nums text-right">
                  {formatPct(arb.fees.round_trip_fees_pct)}
                </span>

                <span className="col-span-2 border-t border-border/30 my-1" />

                <span className="text-muted-foreground font-medium">Net spread</span>
                <span
                  className={cn(
                    "font-mono tabular-nums text-right font-medium",
                    parseFloat(arb.net_spread_pct) >= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatPct(arb.net_spread_pct)}
                </span>
              </div>
            </div>
          </div>
        </TabsContent>

        {/* Simulation tab */}
        <TabsContent value="simulation" className="flex-1 min-h-0 flex flex-col overflow-hidden">
          <div className="px-4 pt-4 pb-2 space-y-3 shrink-0">
            {/* Inputs */}
            <div>
              <div className="flex items-end gap-2">
                <div className="flex-1 min-w-0 space-y-1">
                  <label className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    Position, $
                  </label>
                  <input
                    type="text"
                    inputMode="decimal"
                    value={positionSize}
                    onChange={(e) => setPositionSize(e.target.value)}
                    placeholder="1000"
                    className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
                  />
                </div>
                <div className="w-20 shrink-0 space-y-1">
                  <label className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    Target, %
                  </label>
                  <input
                    type="text"
                    inputMode="decimal"
                    value={targetSpreadPct}
                    onChange={(e) => setTargetSpreadPct(e.target.value)}
                    placeholder="0"
                    className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
                  />
                </div>
                <div className="w-16 shrink-0 space-y-1">
                  <label className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    Slip, %
                  </label>
                  <input
                    type="text"
                    inputMode="decimal"
                    value={slippagePct}
                    onChange={(e) => setSlippagePct(e.target.value)}
                    placeholder="0.05"
                    className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
                  />
                </div>
                <Button
                  variant="default"
                  size="sm"
                  className="h-7 px-3 text-xs bg-violet-600 hover:bg-violet-700 dark:bg-violet-500 dark:hover:bg-violet-600 text-white shrink-0"
                  onClick={runSimulation}
                  disabled={simLoading}
                >
                  {simLoading ? (
                    <Loader2 className="h-3 w-3 animate-spin" />
                  ) : (
                    "Calculate"
                  )}
                </Button>
              </div>
              {/* Size presets */}
              <div className="flex items-center gap-1 mt-1">
                {SIZE_PRESETS.map((preset) => (
                  <button
                    key={preset.value}
                    type="button"
                    className={cn(
                      "rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium transition-colors",
                      positionSize === preset.value
                        ? "bg-violet-500/15 text-violet-600 dark:text-violet-400"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground"
                    )}
                    onClick={() => setPositionSize(preset.value)}
                  >
                    {preset.label}
                  </button>
                ))}
                <span className="mx-1 text-border">|</span>
                {TARGET_PRESETS.map((preset) => (
                  <button
                    key={preset.value}
                    type="button"
                    className={cn(
                      "rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium transition-colors",
                      targetSpreadPct === preset.value
                        ? "bg-violet-500/15 text-violet-600 dark:text-violet-400"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground"
                    )}
                    onClick={() => setTargetSpreadPct(preset.value)}
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
            </div>

            {simError && (
              <div className="text-xs text-red-500 dark:text-red-400">
                {simError}
              </div>
            )}
          </div>

          {/* Simulation results */}
          <div className="flex-1 min-h-0 overflow-y-auto">
            <div className="px-4 py-3">
              {simResult ? (
                <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                  <span className="text-muted-foreground">Position size</span>
                  <span className="font-mono tabular-nums text-right">
                    {formatUsdPlain(simResult.position_size)}
                  </span>

                  <span className="col-span-2 border-t border-border/30 my-1" />

                  <span className="text-muted-foreground">Entry spread</span>
                  <span className="font-mono tabular-nums text-right">
                    {formatPct(simResult.entry_spread_pct)}
                  </span>
                  <span className="text-muted-foreground">Target spread</span>
                  <span className="font-mono tabular-nums text-right">
                    {formatPct(simResult.target_spread_pct)}
                  </span>
                  <span className="text-muted-foreground">Spread capture</span>
                  <span
                    className={cn(
                      "font-mono tabular-nums text-right",
                      parseFloat(simResult.spread_capture_pct) >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {formatPct(simResult.spread_capture_pct)}
                    <span className="ml-1 text-muted-foreground">
                      ({formatUsd(simResult.spread_capture_usd)})
                    </span>
                  </span>

                  <span className="col-span-2 border-t border-border/30 my-1" />

                  <span className="text-muted-foreground">Entry cost</span>
                  <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
                    {formatUsdPlain(simResult.entry_cost)}
                  </span>
                  <span className="text-muted-foreground">Exit cost</span>
                  <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
                    {formatUsdPlain(simResult.exit_cost)}
                  </span>
                  <span className="text-muted-foreground">Total cost</span>
                  <span className="font-mono tabular-nums text-right font-medium text-red-600 dark:text-red-400">
                    {formatUsdPlain(simResult.total_cost)}
                  </span>

                  <span className="col-span-2 border-t border-border/30 my-1" />

                  <span className="text-muted-foreground">Gross P&L</span>
                  <span
                    className={cn(
                      "font-mono tabular-nums text-right",
                      parseFloat(simResult.gross_pnl) >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {formatUsd(simResult.gross_pnl)}
                  </span>
                  <span className="text-muted-foreground font-medium">Net P&L</span>
                  <span
                    className={cn(
                      "font-mono tabular-nums text-right font-medium",
                      parseFloat(simResult.net_pnl) >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {formatUsd(simResult.net_pnl)}
                    <span className="ml-1">
                      ({formatPct(simResult.net_pnl_pct)})
                    </span>
                  </span>

                  <span className="col-span-2 border-t border-border/30 my-1" />

                  <span className="text-muted-foreground">Breakeven spread</span>
                  <span className="font-mono tabular-nums text-right">
                    {formatPct(simResult.breakeven_spread_pct)}
                  </span>
                </div>
              ) : (
                <div className="text-xs text-muted-foreground">
                  Run a simulation to see P&L details.
                </div>
              )}
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Simple filter panel
// ---------------------------------------------------------------------------

function SpreadFilterPanel({
  filters,
  setSymbols,
  setExchanges,
  setNetSpreadMin,
  setNetSpreadMax,
  availableSymbols,
  availableExchanges,
}: {
  filters: SpreadFilters;
  setSymbols: (s: string[]) => void;
  setExchanges: (s: string[]) => void;
  setNetSpreadMin: (s: string) => void;
  setNetSpreadMax: (s: string) => void;
  clearAll?: () => void;
  availableSymbols: string[];
  availableExchanges: string[];
}) {
  return (
    <div className="grid gap-3 rounded-lg border bg-card/50 p-3 sm:grid-cols-3">
      <div className="space-y-1">
        <label className={cn(
          "text-[0.625rem] font-medium uppercase tracking-wider",
          filters.symbols.length > 0 ? "text-foreground" : "text-muted-foreground"
        )}>
          Symbol
        </label>
        <div className="flex min-h-7 flex-wrap items-center gap-1 rounded-md border bg-background px-2 py-1 ring-ring focus-within:ring-2">
          {filters.symbols.map((tag, i) => (
            <Badge key={tag} variant="secondary" className="h-5 gap-0.5 pl-1.5 pr-1 text-[0.625rem]">
              {tag}
              <button
                type="button"
                className="ml-0.5 rounded-full p-0 hover:bg-muted"
                onClick={() => setSymbols(filters.symbols.filter((_, j) => j !== i))}
              >
                <X className="h-2.5 w-2.5" />
              </button>
            </Badge>
          ))}
          <SymbolInput
            suggestions={availableSymbols.filter((s) => !filters.symbols.includes(s))}
            onAdd={(v) => setSymbols([...filters.symbols, v])}
            placeholder={filters.symbols.length === 0 ? "Type symbol..." : ""}
          />
        </div>
      </div>
      <div className="space-y-1">
        <label className={cn(
          "text-[0.625rem] font-medium uppercase tracking-wider",
          filters.exchanges.length > 0 ? "text-foreground" : "text-muted-foreground"
        )}>
          Exchange
        </label>
        <div className="flex min-h-7 flex-wrap items-center gap-1 rounded-md border bg-background px-2 py-1 ring-ring focus-within:ring-2">
          {filters.exchanges.map((tag, i) => (
            <Badge key={tag} variant="secondary" className="h-5 gap-0.5 pl-1.5 pr-1 text-[0.625rem]">
              {tag}
              <button
                type="button"
                className="ml-0.5 rounded-full p-0 hover:bg-muted"
                onClick={() => setExchanges(filters.exchanges.filter((_, j) => j !== i))}
              >
                <X className="h-2.5 w-2.5" />
              </button>
            </Badge>
          ))}
          <SymbolInput
            suggestions={availableExchanges.filter((s) => !filters.exchanges.includes(s))}
            onAdd={(v) => setExchanges([...filters.exchanges, v])}
            placeholder={filters.exchanges.length === 0 ? "Type exchange..." : ""}
          />
        </div>
      </div>
      <div className="space-y-1">
        <label className={cn(
          "text-[0.625rem] font-medium uppercase tracking-wider",
          filters.netSpreadMin || filters.netSpreadMax ? "text-foreground" : "text-muted-foreground"
        )}>
          Net Spread %
        </label>
        <div className="flex items-center gap-2">
          <input
            type="text"
            inputMode="decimal"
            value={filters.netSpreadMin}
            onChange={(e) => setNetSpreadMin(e.target.value)}
            placeholder="Min"
            className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
          />
          <span className="text-muted-foreground text-xs">-</span>
          <input
            type="text"
            inputMode="decimal"
            value={filters.netSpreadMax}
            onChange={(e) => setNetSpreadMax(e.target.value)}
            placeholder="Max"
            className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
          />
        </div>
      </div>
    </div>
  );
}

/** Tiny inline input with dropdown suggestions. */
function SymbolInput({
  suggestions,
  onAdd,
  placeholder,
}: {
  suggestions: string[];
  onAdd: (value: string) => void;
  placeholder: string;
}) {
  const [input, setInput] = useState("");
  const [showDrop, setShowDrop] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const filtered = suggestions.filter((s) =>
    s.toLowerCase().includes(input.toLowerCase())
  );

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setShowDrop(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  return (
    <div ref={ref} className="relative flex-1 min-w-[60px]">
      <input
        type="text"
        value={input}
        onChange={(e) => {
          setInput(e.target.value);
          setShowDrop(true);
        }}
        onFocus={() => setShowDrop(true)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && input.trim()) {
            e.preventDefault();
            onAdd(input.trim());
            setInput("");
            setShowDrop(false);
          }
        }}
        placeholder={placeholder}
        className="h-5 w-full bg-transparent text-xs outline-none placeholder:text-muted-foreground"
      />
      {showDrop && filtered.length > 0 && (
        <div className="absolute left-0 z-50 mt-1 max-h-48 w-full overflow-y-auto rounded-md border bg-popover p-1 shadow-md">
          {filtered.slice(0, 20).map((item) => (
            <button
              key={item}
              type="button"
              className="flex w-full items-center rounded-sm px-2 py-1 text-xs outline-none text-popover-foreground hover:bg-accent hover:text-accent-foreground"
              onMouseDown={(e) => {
                e.preventDefault();
                onAdd(item);
                setInput("");
                setShowDrop(false);
              }}
            >
              {item}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main spread table component
// ---------------------------------------------------------------------------

export function SpreadTable() {
  const router = useRouter();
  const [sort, setSort] = useState<SortState>({
    key: "net_spread_pct",
    desc: true,
  });
  const [selectedArbKey, setSelectedArbKey] = useState<string | null>(null);
  const [openPanel, setOpenPanel] = useState<"none" | "filters">("none");
  const [filters, setFilters] = useState<SpreadFilters>(EMPTY_FILTERS);

  // Resizable detail panel
  const PANEL_MIN = 340;
  const PANEL_MAX = 700;
  const PANEL_DEFAULT = 420;
  const [panelWidth, setPanelWidth] = useState(PANEL_DEFAULT);
  const isDragging = useRef(false);
  const dragStartX = useRef(0);
  const dragStartWidth = useRef(0);

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (!isDragging.current) return;
      e.preventDefault();
      const delta = dragStartX.current - e.clientX;
      const newWidth = Math.min(PANEL_MAX, Math.max(PANEL_MIN, dragStartWidth.current + delta));
      setPanelWidth(newWidth);
    };
    const onMouseUp = () => {
      if (!isDragging.current) return;
      isDragging.current = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
    return () => {
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };
  }, []);

  const startResize = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    isDragging.current = true;
    dragStartX.current = e.clientX;
    dragStartWidth.current = panelWidth;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  }, [panelWidth]);

  const {
    data: arbitrages,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["spread-arbitrage"],
    queryFn: fetchSpreadArbitrage,
  });

  const activeFilterCount = countActiveFilters(filters);

  const filteredData = useMemo(
    () => (arbitrages ? filterData(arbitrages, filters) : []),
    [arbitrages, filters]
  );

  const sortedData = useMemo(
    () => sortData(filteredData, sort),
    [filteredData, sort]
  );

  const availableSymbols = useMemo(() => {
    if (!arbitrages) return [];
    return [...new Set(arbitrages.map((a) => a.symbol))].sort();
  }, [arbitrages]);

  const availableExchanges = useMemo(() => {
    if (!arbitrages) return [];
    const set = new Set<string>();
    for (const a of arbitrages) {
      set.add(a.buy_exchange_id);
      set.add(a.sell_exchange_id);
    }
    return [...set].sort();
  }, [arbitrages]);

  const toggleSort = useCallback((key: SortKey) => {
    setSort((prev) => ({
      key,
      desc: prev.key === key ? !prev.desc : true,
    }));
  }, []);

  const getRowKey = useCallback(
    (arb: SpreadArbitrage) =>
      `${arb.symbol}::${arb.buy_exchange_id}::${arb.sell_exchange_id}`,
    []
  );

  const selectRow = useCallback(
    (arb: SpreadArbitrage) => {
      const key = getRowKey(arb);
      setSelectedArbKey((prev) => (prev === key ? null : key));
    },
    [getRowKey]
  );

  const selectedArb = useMemo(() => {
    if (!selectedArbKey || !sortedData) return null;
    return sortedData.find((a) => getRowKey(a) === selectedArbKey) ?? null;
  }, [selectedArbKey, sortedData, getRowKey]);

  const togglePanel = useCallback((panel: "filters") => {
    setOpenPanel((prev) => (prev === panel ? "none" : panel));
  }, []);

  const clearAll = useCallback(() => {
    setFilters(EMPTY_FILTERS);
  }, []);

  // Virtualization
  const ROW_HEIGHT = 40;
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: sortedData.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  });

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">
          Failed to load spread arbitrage data: {error.message}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0">
      {/* Left: table area */}
      <div className="flex-1 min-w-0 flex flex-col">
        {/* Toolbar & filters */}
        <div className="px-4 pt-3 pb-2 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-screen-xl space-y-2">
            <div className="flex items-center justify-between gap-4">
              <div className="text-xs text-muted-foreground">
                Cross-exchange price spread opportunities (Buy low, Sell high)
              </div>
              {arbitrages && (
                <div className="flex items-center gap-2">
                  {activeFilterCount > 0 &&
                    filteredData.length !== arbitrages.length && (
                      <Badge
                        variant="outline"
                        className="font-mono text-muted-foreground"
                      >
                        {filteredData.length} / {arbitrages.length}
                      </Badge>
                    )}
                  <Badge variant="outline" className="font-mono">
                    {filteredData.length} pairs
                  </Badge>
                </div>
              )}
            </div>

            {/* Toolbar */}
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className={cn(
                    "h-7 gap-1.5 text-xs",
                    activeFilterCount > 0 && "border-primary/40 bg-primary/5 text-primary"
                  )}
                  onClick={() => togglePanel("filters")}
                >
                  <SlidersHorizontal className="h-3 w-3" />
                  Filters
                  {activeFilterCount > 0 && (
                    <Badge
                      variant="default"
                      className="ml-0.5 h-4 min-w-4 px-1 text-[0.5rem]"
                    >
                      {activeFilterCount}
                    </Badge>
                  )}
                  {openPanel === "filters" ? (
                    <ChevronUp className="h-3 w-3" />
                  ) : (
                    <ChevronDown className="h-3 w-3" />
                  )}
                </Button>

                {activeFilterCount > 0 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 gap-1 text-xs text-muted-foreground"
                    onClick={clearAll}
                  >
                    <RotateCcw className="h-3 w-3" />
                    Clear all
                  </Button>
                )}
              </div>

              {openPanel === "filters" && (
                <SpreadFilterPanel
                  filters={filters}
                  setSymbols={(s) => setFilters((f) => ({ ...f, symbols: s }))}
                  setExchanges={(s) => setFilters((f) => ({ ...f, exchanges: s }))}
                  setNetSpreadMin={(s) => setFilters((f) => ({ ...f, netSpreadMin: s }))}
                  setNetSpreadMax={(s) => setFilters((f) => ({ ...f, netSpreadMax: s }))}
                  clearAll={clearAll}
                  availableSymbols={availableSymbols}
                  availableExchanges={availableExchanges}
                />
              )}
            </div>
          </div>
        </div>

        {/* Table with sticky header + virtualized rows */}
        <div className="flex-1 min-h-0 px-4 pb-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-screen-xl h-full">
            <div className="rounded-lg border flex flex-col overflow-hidden h-full">
              {/* Header */}
              <div className="flex items-center border-b bg-background text-xs text-foreground font-medium">
                <div className="px-2 py-1.5 min-w-[90px] flex-[1]">
                  <SortButton
                    label="Symbol"
                    sortKey="symbol"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
                <div className="flex-[3] min-w-0 flex">
                  <div className="flex-[1.5] min-w-0 px-2 py-1.5">Pair / Price</div>
                  <div className="flex-[1] min-w-0 px-2 py-1.5">Volume</div>
                </div>
                <div className="px-2 py-1.5 min-w-[130px] flex-[1.5]">
                  <SortButton
                    label="Spread"
                    sortKey="net_spread_pct"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
                <div className="px-2 py-1.5 min-w-[80px] flex-[0.8]">
                  <SortButton
                    label="Abs"
                    sortKey="spread_pct"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
                <div className="px-2 py-1.5 min-w-[80px] flex-[0.8]">
                  Fees
                </div>
              </div>

              {/* Scrollable body */}
              {isLoading ? (
                <div className="p-2 space-y-2">
                  {Array.from({ length: 10 }).map((_, i) => (
                    <div key={i} className="flex gap-3">
                      <Skeleton className="h-6 w-20" />
                      <Skeleton className="h-6 flex-1" />
                      <Skeleton className="h-6 flex-1" />
                      <Skeleton className="h-6 w-28" />
                      <Skeleton className="h-6 w-16" />
                    </div>
                  ))}
                </div>
              ) : sortedData.length === 0 ? (
                <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground p-8">
                  {activeFilterCount > 0
                    ? "No results match the current filters."
                    : "No spread arbitrage opportunities found. Waiting for data from collector and worker..."}
                </div>
              ) : (
                <div
                  ref={scrollContainerRef}
                  className="flex-1 overflow-y-auto min-h-0"
                >
                  <div
                    style={{ height: virtualizer.getTotalSize(), position: "relative" }}
                  >
                    {virtualizer.getVirtualItems().map((virtualRow) => {
                      const arb = sortedData[virtualRow.index];
                      const rowKey = getRowKey(arb);
                      return (
                        <div
                          key={rowKey}
                          data-index={virtualRow.index}
                          ref={virtualizer.measureElement}
                          style={{
                            position: "absolute",
                            top: 0,
                            left: 0,
                            width: "100%",
                            transform: `translateY(${virtualRow.start}px)`,
                          }}
                        >
                          <DataRowPair
                            arb={arb}
                            isSelected={selectedArbKey === rowKey}
                            onSelect={() => selectRow(arb)}
                          />
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Right: detail panel */}
      {selectedArb && (
        <>
          {/* Resize handle */}
          <div
            className="shrink-0 w-1 cursor-col-resize hover:bg-violet-500/30 active:bg-violet-500/50 transition-colors"
            onMouseDown={startResize}
          />
          <div style={{ width: panelWidth }} className="shrink-0">
            <DetailPanel
              key={getRowKey(selectedArb)}
              arb={selectedArb}
              onClose={() => setSelectedArbKey(null)}
              onNavigate={() => {
                const slug = encodeSpreadSlug(
                  selectedArb.symbol,
                  selectedArb.buy_exchange_id,
                  selectedArb.sell_exchange_id
                );
                router.push(`/arbitrage/spread/${slug}`);
              }}
            />
          </div>
        </>
      )}
    </div>
  );
}
