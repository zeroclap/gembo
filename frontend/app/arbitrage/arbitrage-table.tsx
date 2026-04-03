"use client";

import { useQuery } from "@tanstack/react-query";
import { useState, useMemo, useCallback, useEffect, useRef, Fragment } from "react";
import {
  fetchFundingArbitrage,
  simulateArbitrage,
  type FundingArbitrage,
  type SimulationResult,
  type SimSizeType,
} from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  Clock,
  TrendingUp,
  TrendingDown,
  SlidersHorizontal,
  ChevronDown,
  ChevronUp,
  RotateCcw,
  X,
  Loader2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useArbitrageFilters } from "@/hooks/use-arbitrage-filters";
import { FilterPanel } from "./filter-bar";
import { Countdown } from "./countdown";
import { OpenPositionDialog } from "./open-position-dialog";

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

function formatPct(value: string, decimals = 4): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(decimals)}%`;
}

function formatRate(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  const pct = num * 100;
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
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

function formatAPR(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(2)}%`;
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

function formatDays(value: string): string {
  const num = parseFloat(value);
  if (num < 0 || !isFinite(num)) return "never";
  if (num < 1) return `${(num * 24).toFixed(1)}h`;
  return `${num.toFixed(1)}d`;
}

// ---------------------------------------------------------------------------
// Small components
// ---------------------------------------------------------------------------

/**
 * Compute the net funding spread percentage for the next upcoming cycle.
 */
function computeNextFundingSpread(arb: FundingArbitrage): {
  pct: number;
  label: string;
} {
  const longAt = new Date(arb.long.next_funding_at).getTime();
  const shortAt = new Date(arb.short.next_funding_at).getTime();

  const longValid = !isNaN(longAt);
  const shortValid = !isNaN(shortAt);

  if (!longValid && !shortValid) return { pct: 0, label: "" };

  const longPnl = -(parseFloat(arb.long.funding_rate) || 0);
  const shortPnl = parseFloat(arb.short.funding_rate) || 0;

  if (!shortValid) {
    return { pct: longPnl, label: `${arb.long.interval_hours}h` };
  }
  if (!longValid) {
    return { pct: shortPnl, label: `${arb.short.interval_hours}h` };
  }

  const diff = Math.abs(longAt - shortAt);
  if (diff <= 60_000) {
    const intervals = [arb.long.interval_hours, arb.short.interval_hours].sort(
      (a, b) => a - b
    );
    return {
      pct: longPnl + shortPnl,
      label: intervals[0] === intervals[1]
        ? `${intervals[0]}h`
        : `${intervals[0]}h+${intervals[1]}h`,
    };
  }

  if (longAt < shortAt) {
    return { pct: longPnl, label: `${arb.long.interval_hours}h` };
  }
  return { pct: shortPnl, label: `${arb.short.interval_hours}h` };
}

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
// Sorting
// ---------------------------------------------------------------------------

type SortKey = "symbol" | "funding_spread" | "open_spread";

interface SortState {
  key: SortKey;
  desc: boolean;
}

function getSortValue(
  arb: FundingArbitrage,
  key: SortKey
): number | string {
  switch (key) {
    case "symbol":
      return arb.symbol;
    case "funding_spread":
      return parseFloat(arb.full_loop.net_pct_per_loop) || 0;
    case "open_spread":
      return parseFloat(arb.open_spread) || 0;
  }
}

function sortData(
  data: FundingArbitrage[],
  sort: SortState
): FundingArbitrage[] {
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
// Two-line data row
// ---------------------------------------------------------------------------

function DataRowPair({
  arb,
  isSelected,
  onSelect,
  onOpenPosition,
}: {
  arb: FundingArbitrage;
  isSelected: boolean;
  onSelect: () => void;
  onOpenPosition: () => void;
}) {
  const lcmVal = parseFloat(arb.full_loop.net_pct_per_loop) * 100;
  const nextSpread = computeNextFundingSpread(arb);
  const nextVal = nextSpread.pct * 100;
  const rate24hVal = (parseFloat(arb.net_rate_8h) || 0) * 3 * 100;

  const longRate = parseFloat(arb.long.funding_rate) * 100;
  const shortRate = parseFloat(arb.short.funding_rate) * 100;

  return (
    <tbody
      className={cn(
        "group/row cursor-pointer border-b transition-colors hover:bg-muted/50",
        isSelected && "bg-violet-500/5 dark:bg-violet-500/10 border-violet-500/20"
      )}
      onClick={onSelect}
    >
      {/* Line 1: Long leg */}
      <tr>
        {/* Symbol — spans both lines */}
        <td
          className="p-2 align-middle whitespace-nowrap font-medium"
          rowSpan={2}
        >
          <div className="flex items-center gap-1.5">
            {isSelected && (
              <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />
            )}
            {arb.symbol}
          </div>
        </td>

        {/* Pair: Long */}
        <td className="p-2 align-middle whitespace-nowrap">
          <div className="flex items-center gap-1.5">
            <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
              <ArrowUp className="h-3 w-3 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={arb.long.exchange_id} />
            <span className="flex-1" />
            <span className="font-mono tabular-nums text-muted-foreground">
              {formatPrice(arb.long.price)}
            </span>
          </div>
        </td>

        {/* Funding Rate: Long */}
        <td className="p-2 align-middle whitespace-nowrap">
          <div className="flex items-center gap-1.5">
            <span
              className={cn(
                "font-mono tabular-nums",
                longRate >= 0
                  ? "text-emerald-600 dark:text-emerald-400"
                  : "text-red-600 dark:text-red-400"
              )}
            >
              {formatRate(arb.long.funding_rate)}
            </span>
            <span className="text-muted-foreground text-[0.625rem]">
              {arb.long.interval_hours}h
            </span>
            <span className="text-muted-foreground">
              <Clock className="inline h-3 w-3 mr-0.5" />
              <Countdown targetTime={arb.long.next_funding_at} />
            </span>
          </div>
        </td>

        {/* Funding Spread — spans both lines */}
        <td className="p-2 align-middle whitespace-nowrap" rowSpan={2}>
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-1.5">
              <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider w-12">
                next
              </span>
              <span
                className={cn(
                  "font-mono tabular-nums",
                  nextVal >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {nextVal >= 0 ? "+" : ""}
                {nextVal.toFixed(4)}%
              </span>
              <span className="text-[0.625rem] text-muted-foreground">
                {nextSpread.label}
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider w-12">
                lcm
              </span>
              <span
                className={cn(
                  "font-mono tabular-nums",
                  lcmVal >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {lcmVal >= 0 ? "+" : ""}
                {lcmVal.toFixed(4)}%
              </span>
              <span className="text-[0.625rem] text-muted-foreground">
                {arb.full_loop.duration_hours}h
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider w-12">
                24h
              </span>
              <span
                className={cn(
                  "font-mono tabular-nums",
                  rate24hVal >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {rate24hVal >= 0 ? "+" : ""}
                {rate24hVal.toFixed(4)}%
              </span>
            </div>
          </div>
        </td>

        {/* Open Spread — spans both lines */}
        <td className="p-2 align-middle whitespace-nowrap" rowSpan={2}>
          <div className="flex items-center gap-2">
            <span className="font-mono tabular-nums text-muted-foreground">
              {formatPct(arb.open_spread)}
            </span>
            <Button
              variant="outline"
              size="sm"
              className="h-6 px-2 text-[0.625rem] opacity-0 group-hover/row:opacity-100 transition-opacity"
              onClick={(e) => {
                e.stopPropagation();
                onOpenPosition();
              }}
            >
              Open
            </Button>
          </div>
        </td>
      </tr>

      {/* Line 2: Short leg */}
      <tr>
        {/* Pair: Short */}
        <td className="p-2 align-middle whitespace-nowrap">
          <div className="flex items-center gap-1.5">
            <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
              <ArrowDown className="h-3 w-3 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={arb.short.exchange_id} />
            <span className="flex-1" />
            <span className="font-mono tabular-nums text-muted-foreground">
              {formatPrice(arb.short.price)}
            </span>
          </div>
        </td>

        {/* Funding Rate: Short */}
        <td className="p-2 align-middle whitespace-nowrap">
          <div className="flex items-center gap-1.5">
            <span
              className={cn(
                "font-mono tabular-nums",
                shortRate >= 0
                  ? "text-emerald-600 dark:text-emerald-400"
                  : "text-red-600 dark:text-red-400"
              )}
            >
              {formatRate(arb.short.funding_rate)}
            </span>
            <span className="text-muted-foreground text-[0.625rem]">
              {arb.short.interval_hours}h
            </span>
            <span className="text-muted-foreground">
              <Clock className="inline h-3 w-3 mr-0.5" />
              <Countdown targetTime={arb.short.next_funding_at} />
            </span>
          </div>
        </td>
      </tr>
    </tbody>
  );
}

// ---------------------------------------------------------------------------
// Simulation config presets
// ---------------------------------------------------------------------------

const SIZE_PRESETS = [
  { label: "$1", value: "1" },
  { label: "$10", value: "10" },
  { label: "$100", value: "100" },
  { label: "$1K", value: "1000" },
  { label: "$10K", value: "10000" },
];

const QTY_PRESETS = [
  { label: "1", value: "1" },
  { label: "10", value: "10" },
  { label: "100", value: "100" },
  { label: "1K", value: "1000" },
  { label: "10K", value: "10000" },
];

// ---------------------------------------------------------------------------
// Right-side detail panel
// ---------------------------------------------------------------------------

function DetailPanel({
  arb,
  onClose,
  onOpenPosition,
}: {
  arb: FundingArbitrage;
  onClose: () => void;
  onOpenPosition: () => void;
}) {
  const [sizeType, setSizeType] = useState<SimSizeType>("usd");
  const [positionSize, setPositionSize] = useState("1000");
  const [quantity, setQuantity] = useState("1");
  const [slippagePct, setSlippagePct] = useState("0.05");
  const [simResult, setSimResult] = useState<SimulationResult | null>(null);
  const [simLoading, setSimLoading] = useState(false);
  const [simError, setSimError] = useState<string | null>(null);

  const runSimulation = useCallback(async () => {
    if (sizeType === "usd") {
      const size = parseFloat(positionSize);
      if (!size || size <= 0) return;
    } else {
      const qty = parseFloat(quantity);
      if (!qty || qty <= 0) return;
    }

    setSimLoading(true);
    setSimError(null);
    try {
      const result = await simulateArbitrage({
        symbol: arb.symbol,
        long_exchange_id: arb.long.exchange_id,
        short_exchange_id: arb.short.exchange_id,
        size_type: sizeType,
        ...(sizeType === "usd"
          ? { position_size: positionSize }
          : { quantity: quantity }),
        slippage_pct: slippagePct || "0",
      });
      setSimResult(result);
    } catch (err) {
      setSimError(err instanceof Error ? err.message : "Simulation failed");
    } finally {
      setSimLoading(false);
    }
  }, [arb, sizeType, positionSize, quantity, slippagePct]);

  // Auto-trigger simulation when the panel opens.
  const didAutoRun = useRef(false);
  useEffect(() => {
    if (!didAutoRun.current) {
      didAutoRun.current = true;
      runSimulation();
    }
  }, [runSimulation]);

  const loopPayments = simResult?.loop_payments ?? [];
  const fallbackPayments = arb.full_loop.payments ?? [];
  const hasLoopPayments = loopPayments.length > 0 || fallbackPayments.length > 0;

  const detailRows: {
    label: string;
    long: React.ReactNode;
    short: React.ReactNode;
  }[] = [
    {
      label: "Exchange",
      long: <ExchangeBadge exchangeId={arb.long.exchange_id} />,
      short: <ExchangeBadge exchangeId={arb.short.exchange_id} />,
    },
    {
      label: "Bid / Ask",
      long: `${formatPrice(arb.long.bid)} / ${formatPrice(arb.long.ask)}`,
      short: `${formatPrice(arb.short.bid)} / ${formatPrice(arb.short.ask)}`,
    },
    {
      label: "Mark",
      long: formatPrice(arb.long.mark_price),
      short: formatPrice(arb.short.mark_price),
    },
    {
      label: "Index",
      long: formatPrice(arb.long.index_price),
      short: formatPrice(arb.short.index_price),
    },
    {
      label: "Volume 24h",
      long: formatVolume(arb.long.volume_24h),
      short: formatVolume(arb.short.volume_24h),
    },
    {
      label: "Taker fee",
      long: `${arb.long.taker_fee_pct}%`,
      short: `${arb.short.taker_fee_pct}%`,
    },
    {
      label: "Rate 8h",
      long: formatRate(arb.long.rate_8h),
      short: formatRate(arb.short.rate_8h),
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
              <ExchangeBadge exchangeId={arb.long.exchange_id} />
            </span>
            <span className="text-muted-foreground/50">/</span>
            <span className="inline-flex items-center gap-1">
              <TrendingDown className="h-3 w-3 text-red-500" />
              <ExchangeBadge exchangeId={arb.short.exchange_id} />
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
              onOpenPosition();
            }}
          >
            Open Position
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <div className="p-4 space-y-5 flex-1 overflow-y-auto">
        {/* Comparison table */}
        <div>
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
            Leg comparison
          </div>
          <table className="text-xs w-full">
            <thead>
              <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                <th className="pr-4 pb-1 text-left font-medium" />
                <th className="pr-4 pb-1 text-right font-medium">
                  <span className="inline-flex items-center gap-1">
                    <TrendingUp className="h-3 w-3 text-emerald-500" />
                    Long
                  </span>
                </th>
                <th className="pb-1 text-right font-medium">
                  <span className="inline-flex items-center gap-1">
                    <TrendingDown className="h-3 w-3 text-red-500" />
                    Short
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
                    {row.long}
                  </td>
                  <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                    {row.short}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Fee breakdown */}
        <div>
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
            Fee breakdown
          </div>
          <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
            <span className="text-muted-foreground">Open</span>
            <span className="font-mono tabular-nums text-right whitespace-nowrap">
              {formatPct(arb.fees.open_fees_pct)}
              {simResult && (
                <span className="ml-1 text-muted-foreground">
                  ({formatUsdPlain(simResult.entry_cost)})
                </span>
              )}
            </span>
            <span className="text-muted-foreground">Close</span>
            <span className="font-mono tabular-nums text-right whitespace-nowrap">
              {formatPct(arb.fees.close_fees_pct)}
              {simResult && (
                <span className="ml-1 text-muted-foreground">
                  ({formatUsdPlain(simResult.exit_cost)})
                </span>
              )}
            </span>
            <span className="text-muted-foreground">Round-trip</span>
            <span className="font-mono tabular-nums text-right whitespace-nowrap">
              {formatPct(arb.fees.round_trip_fees_pct)}
              {simResult && (
                <span className="ml-1 text-muted-foreground">
                  ({formatUsdPlain(simResult.total_cost)})
                </span>
              )}
            </span>
            <span className="text-muted-foreground">Net APR</span>
            <span
              className={cn(
                "font-mono tabular-nums font-medium text-right",
                parseFloat(arb.fees.net_apr_after_fees) >= 0
                  ? "text-emerald-600 dark:text-emerald-400"
                  : "text-red-600 dark:text-red-400"
              )}
            >
              {formatAPR(arb.fees.net_apr_after_fees)}
            </span>
          </div>
        </div>

        {/* Simulation */}
        <div>
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-violet-600 dark:text-violet-400 mb-2">
            Simulation
          </div>
          <div className="space-y-3">
            {/* Inputs — single row: size type toggle, value, slippage, calculate */}
            <div>
              <div className="flex items-end gap-2">
                {/* Size type toggle */}
                <div className="shrink-0 space-y-1">
                  <label className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    Mode
                  </label>
                  <div className="flex h-7 rounded-md border bg-muted/50 p-0.5">
                    <button
                      type="button"
                      className={cn(
                        "rounded-sm px-2 text-[0.625rem] font-medium transition-colors",
                        sizeType === "usd"
                          ? "bg-background text-foreground shadow-sm"
                          : "text-muted-foreground hover:text-foreground"
                      )}
                      onClick={() => setSizeType("usd")}
                    >
                      USD
                    </button>
                    <button
                      type="button"
                      className={cn(
                        "rounded-sm px-2 text-[0.625rem] font-medium transition-colors",
                        sizeType === "qty"
                          ? "bg-background text-foreground shadow-sm"
                          : "text-muted-foreground hover:text-foreground"
                      )}
                      onClick={() => setSizeType("qty")}
                    >
                      QTY
                    </button>
                  </div>
                </div>
                {/* Value input */}
                <div className="flex-1 min-w-0 space-y-1">
                  <label className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    {sizeType === "usd" ? "Position, $" : "Contracts"}
                  </label>
                  <input
                    type="text"
                    inputMode="decimal"
                    value={sizeType === "usd" ? positionSize : quantity}
                    onChange={(e) =>
                      sizeType === "usd"
                        ? setPositionSize(e.target.value)
                        : setQuantity(e.target.value)
                    }
                    placeholder={sizeType === "usd" ? "1000" : "1"}
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
              <div className="flex items-center gap-1 mt-1">
                {(sizeType === "usd" ? SIZE_PRESETS : QTY_PRESETS).map(
                  (preset) => {
                    const active =
                      sizeType === "usd"
                        ? positionSize === preset.value
                        : quantity === preset.value;
                    return (
                      <button
                        key={preset.value}
                        type="button"
                        className={cn(
                          "rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium transition-colors",
                          active
                            ? "bg-violet-500/15 text-violet-600 dark:text-violet-400"
                            : "text-muted-foreground hover:bg-muted hover:text-foreground"
                        )}
                        onClick={() =>
                          sizeType === "usd"
                            ? setPositionSize(preset.value)
                            : setQuantity(preset.value)
                        }
                      >
                        {preset.label}
                      </button>
                    );
                  }
                )}
              </div>
            </div>

            {simError && (
              <div className="text-xs text-red-500 dark:text-red-400">
                {simError}
              </div>
            )}

            {/* Results */}
            {simResult && (
              <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <span className="text-muted-foreground">Quantity</span>
                <span className="font-mono tabular-nums text-right">
                  {simResult.quantity} contracts
                </span>
                <span className="text-muted-foreground">Position size</span>
                <span className="font-mono tabular-nums text-right">
                  {formatUsdPlain(simResult.actual_position_size)}
                  {sizeType === "usd" &&
                    simResult.actual_position_size !== positionSize && (
                      <span className="ml-1 text-muted-foreground text-[0.625rem]">
                        (floored)
                      </span>
                    )}
                </span>
                <span className="text-muted-foreground">Capital</span>
                <span className="font-mono tabular-nums text-right">
                  {formatUsdPlain(simResult.capital)}
                </span>
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

                <span className="text-muted-foreground">Funding/8h</span>
                <span
                  className={cn(
                    "font-mono tabular-nums text-right",
                    parseFloat(simResult.funding_8h) >= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatUsd(simResult.funding_8h)}
                </span>
                <span className="text-muted-foreground">Funding/24h</span>
                <span
                  className={cn(
                    "font-mono tabular-nums text-right",
                    parseFloat(simResult.funding_24h) >= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatUsd(simResult.funding_24h)}
                </span>
                <span className="text-muted-foreground">Breakeven</span>
                <span className="font-mono tabular-nums text-right">
                  {formatDays(simResult.breakeven_days)}
                </span>

                <span className="col-span-2 border-t border-border/30 my-1" />

                <span className="text-muted-foreground">Next P&L</span>
                <span
                  className={cn(
                    "font-mono tabular-nums text-right font-medium",
                    parseFloat(simResult.next_pnl) >= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatUsd(simResult.next_pnl)}
                </span>
                <span className="text-muted-foreground">LCM P&L</span>
                <span
                  className={cn(
                    "font-mono tabular-nums text-right font-medium",
                    parseFloat(simResult.lcm_pnl) >= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatUsd(simResult.lcm_pnl)}
                </span>
                <span className="text-muted-foreground">24h P&L</span>
                <span
                  className={cn(
                    "font-mono tabular-nums text-right font-medium",
                    parseFloat(simResult.pnl_24h) >= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatUsd(simResult.pnl_24h)}
                </span>
              </div>
            )}
          </div>
        </div>

        {/* Full loop payment schedule — uses simulation result when available, falls back to arb data */}
        {hasLoopPayments && (
          <div>
            <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
              Full loop ({arb.full_loop.duration_hours}h) &mdash;{" "}
              {(loopPayments.length || fallbackPayments.length)} payments &mdash; Net:{" "}
              <span
                className={cn(
                  "font-mono",
                  parseFloat(arb.full_loop.net_pct_per_loop) >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {formatRate(arb.full_loop.net_pct_per_loop)}
              </span>
              {simResult && (
                <span className="ml-1 text-muted-foreground font-mono">
                  ({formatUsd(simResult.lcm_pnl)})
                </span>
              )}
            </div>
            <div className="overflow-x-auto">
              <table className="text-xs w-full">
                <thead>
                  <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    <th className="pr-3 pb-1 text-left font-medium">Hour</th>
                    <th className="pr-3 pb-1 text-left font-medium">Exchange</th>
                    <th className="pr-3 pb-1 text-left font-medium">Side</th>
                    <th className="pr-3 pb-1 text-right font-medium">Rate</th>
                    <th className="pr-3 pb-1 text-right font-medium">P&L</th>
                    <th className="pr-3 pb-1 text-right font-medium">Cumulative</th>
                    {loopPayments.length > 0 && (
                      <>
                        <th className="pl-3 pb-1 text-right font-medium">$ P&L</th>
                        <th className="pl-3 pb-1 text-right font-medium">$ Cum.</th>
                      </>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {loopPayments.length > 0
                    ? loopPayments.map((p, i) => {
                        const pnl = parseFloat(p.payment_pct);
                        const cum = parseFloat(p.cumulative);
                        const pnlUsd = parseFloat(p.payment_usd);
                        const cumUsd = parseFloat(p.cumulative_usd);
                        return (
                          <tr key={i} className="border-t border-border/30">
                            <td className="pr-3 py-0.5 font-mono tabular-nums">
                              {p.at_hour}h
                            </td>
                            <td className="pr-3 py-0.5">
                              <ExchangeBadge exchangeId={p.exchange_id} />
                            </td>
                            <td className="pr-3 py-0.5 text-muted-foreground">
                              {p.side}
                            </td>
                            <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                              {formatRate(p.rate)}
                            </td>
                            <td
                              className={cn(
                                "pr-3 py-0.5 text-right font-mono tabular-nums",
                                pnl >= 0
                                  ? "text-emerald-600 dark:text-emerald-400"
                                  : "text-red-600 dark:text-red-400"
                              )}
                            >
                              {formatRate(p.payment_pct)}
                            </td>
                            <td
                              className={cn(
                                "pr-3 py-0.5 text-right font-mono tabular-nums font-medium",
                                cum >= 0
                                  ? "text-emerald-600 dark:text-emerald-400"
                                  : "text-red-600 dark:text-red-400"
                              )}
                            >
                              {formatRate(p.cumulative)}
                            </td>
                            <td
                              className={cn(
                                "pl-3 py-0.5 text-right font-mono tabular-nums",
                                pnlUsd >= 0
                                  ? "text-emerald-600 dark:text-emerald-400"
                                  : "text-red-600 dark:text-red-400"
                              )}
                            >
                              {formatUsd(p.payment_usd)}
                            </td>
                            <td
                              className={cn(
                                "pl-3 py-0.5 text-right font-mono tabular-nums font-medium",
                                cumUsd >= 0
                                  ? "text-emerald-600 dark:text-emerald-400"
                                  : "text-red-600 dark:text-red-400"
                              )}
                            >
                              {formatUsd(p.cumulative_usd)}
                            </td>
                          </tr>
                        );
                      })
                    : fallbackPayments.map((p, i) => {
                        const pnl = parseFloat(p.payment_pct);
                        const cum = parseFloat(p.cumulative);
                        return (
                          <tr key={i} className="border-t border-border/30">
                            <td className="pr-3 py-0.5 font-mono tabular-nums">
                              {p.at_hour}h
                            </td>
                            <td className="pr-3 py-0.5">
                              <ExchangeBadge exchangeId={p.exchange_id} />
                            </td>
                            <td className="pr-3 py-0.5 text-muted-foreground">
                              {p.side}
                            </td>
                            <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                              {formatRate(p.rate)}
                            </td>
                            <td
                              className={cn(
                                "pr-3 py-0.5 text-right font-mono tabular-nums",
                                pnl >= 0
                                  ? "text-emerald-600 dark:text-emerald-400"
                                  : "text-red-600 dark:text-red-400"
                              )}
                            >
                              {formatRate(p.payment_pct)}
                            </td>
                            <td
                              className={cn(
                                "pr-3 py-0.5 text-right font-mono tabular-nums font-medium",
                                cum >= 0
                                  ? "text-emerald-600 dark:text-emerald-400"
                                  : "text-red-600 dark:text-red-400"
                              )}
                            >
                              {formatRate(p.cumulative)}
                            </td>
                          </tr>
                        );
                      })}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main table component
// ---------------------------------------------------------------------------

export function ArbitrageTable() {
  const [sort, setSort] = useState<SortState>({
    key: "funding_spread",
    desc: true,
  });
  const [selectedArbKey, setSelectedArbKey] = useState<string | null>(null);
  const [openPanel, setOpenPanel] = useState<"none" | "filters">("none");
  const [openPositionArb, setOpenPositionArb] = useState<FundingArbitrage | null>(null);

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
    symbols,
    exchanges,
    volumeMin,
    volumeMax,
    spreadMin,
    spreadMax,
    fundingSpreadMin,
    fundingSpreadMax,
    fundingLoopMin,
    fundingLoopMax,
    activeFilterCount,
    setSymbols,
    setExchanges,
    setVolumeMin,
    setVolumeMax,
    setSpreadMin,
    setSpreadMax,
    setFundingSpreadMin,
    setFundingSpreadMax,
    setFundingLoopMin,
    setFundingLoopMax,
    clearAll,
    filterData,
  } = useArbitrageFilters();

  const {
    data: arbitrages,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["funding-arbitrage"],
    queryFn: fetchFundingArbitrage,
  });

  const filteredData = useMemo(
    () => (arbitrages ? filterData(arbitrages) : []),
    [arbitrages, filterData]
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
      set.add(a.long.exchange_id);
      set.add(a.short.exchange_id);
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
    (arb: FundingArbitrage) =>
      `${arb.symbol}::${arb.long.exchange_id}::${arb.short.exchange_id}`,
    []
  );

  const selectRow = useCallback(
    (arb: FundingArbitrage) => {
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

  const colCount = 5;

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">
          Failed to load arbitrage data: {error.message}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0">
      {/* Left: table area — scrollable */}
      <div className="flex-1 min-w-0 overflow-y-auto px-4 py-4 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-screen-xl space-y-4">
        <div className="flex items-center justify-between gap-4">
          <div className="text-xs text-muted-foreground">
            Funding rate arbitrage opportunities (Long + Short, cross-exchange)
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
            {/* Filters button */}
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
            <FilterPanel
              filters={{
                symbols,
                exchanges,
                volumeMin,
                volumeMax,
                spreadMin,
                spreadMax,
                fundingSpreadMin,
                fundingSpreadMax,
                fundingLoopMin,
                fundingLoopMax,
              }}
              actions={{
                setSymbols,
                setExchanges,
                setVolumeMin,
                setVolumeMax,
                setSpreadMin,
                setSpreadMax,
                setFundingSpreadMin,
                setFundingSpreadMax,
                setFundingLoopMin,
                setFundingLoopMax,
                clearAll,
              }}
              availableSymbols={availableSymbols}
              availableExchanges={availableExchanges}
            />
          )}
        </div>

        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>
                  <SortButton
                    label="Symbol"
                    sortKey="symbol"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </TableHead>
                <TableHead>Pair</TableHead>
                <TableHead>Funding Rate</TableHead>
                <TableHead>
                  <SortButton
                    label="Funding Spread"
                    sortKey="funding_spread"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </TableHead>
                <TableHead>
                  <SortButton
                    label="Open Spread, %"
                    sortKey="open_spread"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </TableHead>
              </TableRow>
            </TableHeader>

            {isLoading ? (
              <TableBody>
                {Array.from({ length: 10 }).map((_, i) => (
                  <Fragment key={i}>
                    <TableRow className="border-0">
                      <TableCell rowSpan={2}>
                        <Skeleton className="h-4 w-20" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-32" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-40" />
                      </TableCell>
                      <TableCell rowSpan={2}>
                        <Skeleton className="h-4 w-24" />
                      </TableCell>
                      <TableCell rowSpan={2}>
                        <Skeleton className="h-4 w-16" />
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell>
                        <Skeleton className="h-4 w-32" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-40" />
                      </TableCell>
                    </TableRow>
                  </Fragment>
                ))}
              </TableBody>
            ) : sortedData.length === 0 ? (
              <TableBody>
                <TableRow>
                  <TableCell
                    colSpan={colCount}
                    className="h-24 text-center text-muted-foreground"
                  >
                    {activeFilterCount > 0
                      ? "No results match the current filters."
                      : "No arbitrage opportunities found. Waiting for data from collector and worker..."}
                  </TableCell>
                </TableRow>
              </TableBody>
            ) : (
              sortedData.map((arb) => {
                const rowKey = getRowKey(arb);
                return (
                  <DataRowPair
                    key={rowKey}
                    arb={arb}
                    isSelected={selectedArbKey === rowKey}
                    onSelect={() => selectRow(arb)}
                    onOpenPosition={() => setOpenPositionArb(arb)}
                  />
                );
              })
            )}
          </Table>
        </div>
        </div>
      </div>

      {/* Right: detail panel — fills height, scrolls independently */}
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
              onOpenPosition={() => setOpenPositionArb(selectedArb)}
            />
          </div>
        </>
      )}

      {/* Open position dialog */}
      {openPositionArb && (
        <OpenPositionDialog
          arb={openPositionArb}
          open={!!openPositionArb}
          onOpenChange={(open) => {
            if (!open) setOpenPositionArb(null);
          }}
        />
      )}
    </div>
  );
}
