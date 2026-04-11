"use client";

import { useQuery } from "@tanstack/react-query";
import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  fetchFundingArbitrage,
  simulateArbitrage,
  type FundingArbitrage,
  type SimulationResult,
  type SimulationRow,
  type SimulationEvent,
  type LoopSimulation,
  type SimSizeType,
} from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
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
  ChevronRight,
  ChevronUp,
  RotateCcw,
  X,
  Loader2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useArbitrageFilters } from "@/hooks/use-arbitrage-filters";
import { FilterPanel } from "./filter-bar";
import { Countdown } from "./countdown";
import { useRouter } from "next/navigation";
import { encodeArbSlug } from "./[slug]/page";

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

/** Format hold hours as a compact label, e.g. "24h", "7d", "48h". */
function formatHoldLabel(value: string): string {
  const h = parseFloat(value);
  if (!isFinite(h) || h <= 0) return "24h";
  if (h % 24 === 0 && h >= 24) return `${h / 24}d`;
  return `${h}h`;
}

// ---------------------------------------------------------------------------
// Small components
// ---------------------------------------------------------------------------

/**
 * Derive a short interval label for the next funding payment.
 * Uses the backend-computed next_payment info + leg interval hours.
 */
function nextPaymentLabel(arb: FundingArbitrage): string {
  const np = arb.next_payment;
  if (!np.at) return "";

  const longAt = new Date(arb.long.next_funding_at).getTime();
  const shortAt = new Date(arb.short.next_funding_at).getTime();
  const diff = Math.abs(longAt - shortAt);

  // If both legs fire at the same time (within 60s), show combined label.
  if (diff <= 60_000) {
    const intervals = [arb.long.interval_hours, arb.short.interval_hours].sort(
      (a, b) => a - b
    );
    return intervals[0] === intervals[1]
      ? `${intervals[0]}h`
      : `${intervals[0]}h+${intervals[1]}h`;
  }

  // Otherwise show the interval of the side that fires next.
  if (np.side === "long") return `${arb.long.interval_hours}h`;
  return `${arb.short.interval_hours}h`;
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
// Two-line data row (div-based for virtualization)
// ---------------------------------------------------------------------------

function DataRowPair({
  arb,
  isSelected,
  onSelect,
}: {
  arb: FundingArbitrage;
  isSelected: boolean;
  onSelect: () => void;
}) {
  const lcmVal = parseFloat(arb.full_loop.net_pct_per_loop) * 100;
  const nextVal = parseFloat(arb.next_payment.payment_pct) * 100;
  const nextLabel = nextPaymentLabel(arb);
  const rate24hVal = (parseFloat(arb.net_rate_8h) || 0) * 3 * 100;

  const longRate = parseFloat(arb.long.funding_rate) * 100;
  const shortRate = parseFloat(arb.short.funding_rate) * 100;

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
        <div className="flex items-center px-2 py-0.5 min-w-[90px] flex-[1]">
          <div className="flex items-center gap-1 whitespace-nowrap font-medium text-xs">
            {isSelected && (
              <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />
            )}
            {arb.symbol}
          </div>
        </div>

        {/* Cols 2-3: Pair + Funding Rate */}
        <div className="flex-[3.5] min-w-0 flex flex-col justify-center gap-0.5 py-0.5">
          {/* Long leg */}
          <div className="flex">
            <div className="flex-[1.5] min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
                <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                  <ArrowUp className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
                </span>
                <ExchangeBadge exchangeId={arb.long.exchange_id} />
                <span className="flex-1" />
                <span className="font-mono tabular-nums text-muted-foreground">
                  {formatPrice(arb.long.price)}
                </span>
              </div>
            </div>
            <div className="flex-[2] min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
                <span
                  className={cn(
                    "font-mono tabular-nums",
                    longRate <= 0
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatRate(arb.long.funding_rate)}
                </span>
                <span className="text-muted-foreground text-[10px]">
                  {arb.long.interval_hours}h
                </span>
                <span className="text-muted-foreground text-[10px]">
                  <Clock className="inline h-2.5 w-2.5 mr-0.5 align-text-top" />
                  <Countdown targetTime={arb.long.next_funding_at} />
                </span>
              </div>
            </div>
          </div>
          {/* Short leg */}
          <div className="flex">
            <div className="flex-[1.5] min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
                <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                  <ArrowDown className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
                </span>
                <ExchangeBadge exchangeId={arb.short.exchange_id} />
                <span className="flex-1" />
                <span className="font-mono tabular-nums text-muted-foreground">
                  {formatPrice(arb.short.price)}
                </span>
              </div>
            </div>
            <div className="flex-[2] min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
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
                <span className="text-muted-foreground text-[10px]">
                  {arb.short.interval_hours}h
                </span>
                <span className="text-muted-foreground text-[10px]">
                  <Clock className="inline h-2.5 w-2.5 mr-0.5 align-text-top" />
                  <Countdown targetTime={arb.short.next_funding_at} />
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Col 4: Funding Spread */}
        <div className="flex items-center px-2 py-0.5 min-w-[160px] flex-[2] whitespace-nowrap">
          <div className="flex flex-col leading-[1.15rem]">
            <div className="flex items-center gap-1">
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider w-8">
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
              <span className="text-[10px] text-muted-foreground">
                {nextLabel}
              </span>
            </div>
            <div className="flex items-center gap-1">
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider w-8">
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
              <span className="text-[10px] text-muted-foreground">
                {arb.full_loop.duration_hours}h
              </span>
            </div>
            <div className="flex items-center gap-1">
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider w-8">
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
        </div>

        {/* Col 5: Open Spread */}
        <div className="flex items-center px-2 py-0.5 min-w-[100px] flex-[1] whitespace-nowrap">
          <span className="font-mono tabular-nums text-muted-foreground">
            {formatPct(arb.open_spread)}
          </span>
        </div>
      </div>
    </div>
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

const HOLD_PRESETS = [
  { label: "8h", value: "8" },
  { label: "24h", value: "24" },
  { label: "48h", value: "48" },
  { label: "7d", value: "168" },
  { label: "30d", value: "720" },
];

// ---------------------------------------------------------------------------
// Simulation table row — flat, no expansion, all info inline, uniform height
// ---------------------------------------------------------------------------

function SimulationTableRow({
  row,
  pnlUsd,
  cumUsd,
  isFee,
  isExpanded,
  onToggle,
}: {
  row: SimulationRow;
  pnlUsd: number;
  cumUsd: number;
  isFee: boolean;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const eventCount = row.events.length;

  return (
    <tr
      className={cn(
        "border-t border-border/30 h-[34px] align-middle cursor-pointer hover:bg-muted/20",
        isFee && "bg-muted/30"
      )}
      onClick={onToggle}
    >
      {/* Time — first column */}
      <td className="pr-3 py-0.5 font-mono tabular-nums text-muted-foreground text-[0.625rem] align-middle">
        <span className="flex items-center gap-1">
          <ChevronRight
            className={cn(
              "size-3 shrink-0 text-muted-foreground/60 transition-transform duration-150",
              isExpanded && "rotate-90"
            )}
          />
          <span>
            {row.at
              ? new Date(row.at).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                  hour12: false,
                })
              : "-"}
          </span>
        </span>
      </td>

      {/* Hour */}
      <td className="pr-3 py-0.5 font-mono tabular-nums text-[0.625rem] align-middle">
        {row.label}
      </td>

      {/* Event — summary showing leg count */}
      <td className="pr-3 py-0.5 align-middle">
        <span className="text-muted-foreground text-[0.5625rem]">
          {isFee
            ? (row.kind === "open" ? "Open" : "Close")
            : `${eventCount} ${eventCount === 1 ? "leg" : "legs"}`}
        </span>
      </td>

      {/* Rate — empty for summary row */}
      <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-[0.625rem] text-muted-foreground align-middle">
        -
      </td>

      {/* $ P&L */}
      <td
        className={cn(
          "pr-3 py-0.5 text-right font-mono tabular-nums text-[0.625rem] align-middle",
          pnlUsd >= 0
            ? "text-emerald-600 dark:text-emerald-400"
            : "text-red-600 dark:text-red-400"
        )}
      >
        {formatUsd(row.pnl_usd)}
      </td>

      {/* $ Cumulative */}
      <td
        className={cn(
          "pr-3 py-0.5 text-right font-mono tabular-nums text-[0.625rem] font-medium align-middle",
          cumUsd >= 0
            ? "text-emerald-600 dark:text-emerald-400"
            : "text-red-600 dark:text-red-400"
        )}
      >
        {formatUsd(row.cumulative_usd)}
      </td>
    </tr>
  );
}

/** Expanded event row showing per-exchange detail. */
function SimulationEventRow({ event }: { event: SimulationEvent }) {
  const pnlUsd = parseFloat(event.pnl_usd);
  return (
    <tr className="h-[34px] align-middle bg-muted/10">
      {/* Time — empty for event rows */}
      <td className="pr-3 py-0.5 align-middle" />

      {/* Hour — empty for event rows */}
      <td className="pr-3 py-0.5 align-middle" />

      {/* Event — exchange badge + side */}
      <td className="pr-3 py-0.5 align-middle">
        <span className="flex items-center gap-1 pl-4">
          <ExchangeBadge exchangeId={event.exchange_id} />
          <span className="text-muted-foreground text-[0.5625rem]">
            {event.side}
          </span>
        </span>
      </td>

      {/* Rate */}
      <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-muted-foreground text-[0.5625rem] align-middle">
        {event.rate ? formatRate(event.rate) : "-"}
      </td>

      {/* $ P&L */}
      <td
        className={cn(
          "pr-3 py-0.5 text-right font-mono tabular-nums text-[0.5625rem] align-middle",
          pnlUsd >= 0
            ? "text-emerald-600/70 dark:text-emerald-400/70"
            : "text-red-600/70 dark:text-red-400/70"
        )}
      >
        {formatUsd(event.pnl_usd)}
      </td>

      {/* $ Cumulative — empty for event rows */}
      <td className="pr-3 py-0.5 align-middle" />
    </tr>
  );
}

// ---------------------------------------------------------------------------
// Right-side detail panel
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Virtualized payment schedule list
// ---------------------------------------------------------------------------

const SCHEDULE_ROW_HEIGHT = 34;

/** Shared colgroup so header + every virtualized row table align columns. */
function ScheduleColGroup() {
  return (
    <colgroup>
      <col className="w-[12%]" />
      <col className="w-[12%]" />
      <col className="w-[28%]" />
      <col className="w-[16%]" />
      <col className="w-[16%]" />
      <col className="w-[16%]" />
    </colgroup>
  );
}

/** A flattened item: either a parent row or an event under an expanded parent. */
type FlatItem =
  | { type: "parent"; row: SimulationRow; index: number }
  | { type: "event"; event: SimulationEvent; parentIndex: number };

function ScheduleVirtualList({ rows }: { rows: SimulationRow[] }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [expandedSet, setExpandedSet] = useState<Set<number>>(() => new Set());

  const toggleExpanded = useCallback((index: number) => {
    setExpandedSet((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }, []);

  // Reset expanded state when rows change (e.g. new simulation)
  useEffect(() => {
    setExpandedSet(new Set());
  }, [rows]);

  // Build flattened list: parent rows + expanded events
  const flatItems = useMemo<FlatItem[]>(() => {
    const items: FlatItem[] = [];
    for (let i = 0; i < rows.length; i++) {
      items.push({ type: "parent", row: rows[i], index: i });
      if (expandedSet.has(i)) {
        for (const event of rows[i].events) {
          items.push({ type: "event", event, parentIndex: i });
        }
      }
    }
    return items;
  }, [rows, expandedSet]);

  const virtualizer = useVirtualizer({
    count: flatItems.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => SCHEDULE_ROW_HEIGHT,
    overscan: 5,
  });

  return (
    <div ref={scrollRef} className="flex-1 min-h-0 overflow-y-auto px-4">
      <div style={{ height: virtualizer.getTotalSize(), position: "relative" }} className="w-full">
        {virtualizer.getVirtualItems().map((vRow) => {
          const item = flatItems[vRow.index];

          if (item.type === "event") {
            return (
              <div
                key={`event-${item.parentIndex}-${vRow.index}`}
                data-index={vRow.index}
                ref={virtualizer.measureElement}
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${vRow.start}px)`,
                }}
              >
                <table className="text-xs w-full table-fixed">
                  <ScheduleColGroup />
                  <tbody>
                    <SimulationEventRow event={item.event} />
                  </tbody>
                </table>
              </div>
            );
          }

          const row = item.row;
          const pnlUsd = parseFloat(row.pnl_usd);
          const cumUsd = parseFloat(row.cumulative_usd);
          const isFee = row.kind === "open" || row.kind === "close";
          const isExpanded = expandedSet.has(item.index);
          return (
            <div
              key={`parent-${item.index}`}
              data-index={vRow.index}
              ref={virtualizer.measureElement}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                transform: `translateY(${vRow.start}px)`,
              }}
            >
              <table className="text-xs w-full table-fixed">
                <ScheduleColGroup />
                <tbody>
                  <SimulationTableRow
                    row={row}
                    pnlUsd={pnlUsd}
                    cumUsd={cumUsd}
                    isFee={isFee}
                    isExpanded={isExpanded}
                    onToggle={() => toggleExpanded(item.index)}
                  />
                </tbody>
              </table>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Detail panel
// ---------------------------------------------------------------------------

function DetailPanel({
  arb,
  onClose,
  onNavigate,
}: {
  arb: FundingArbitrage;
  onClose: () => void;
  onNavigate: () => void;
}) {
  const [sizeType, setSizeType] = useState<SimSizeType>("usd");
  const [positionSize, setPositionSize] = useState("100");
  const [quantity, setQuantity] = useState("100");
  const [slippagePct, setSlippagePct] = useState("0.05");
  const [holdHours, setHoldHours] = useState("24");
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
        hold_hours: holdHours || "24",
      });
      setSimResult(result);
    } catch (err) {
      setSimError(err instanceof Error ? err.message : "Simulation failed");
    } finally {
      setSimLoading(false);
    }
  }, [arb, sizeType, positionSize, quantity, slippagePct, holdHours]);

  // Auto-trigger simulation when the panel opens.
  const didAutoRun = useRef(false);
  useEffect(() => {
    if (!didAutoRun.current) {
      didAutoRun.current = true;
      runSimulation();
    }
  }, [runSimulation]);

  // Hold simulation rows from the result.
  const activeLoop: LoopSimulation | null = simResult?.hold_simulation ?? null;
  const activeRows: SimulationRow[] = activeLoop?.rows ?? [];

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

        {/* Details tab — leg comparison + fee breakdown */}
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
          </div>
        </TabsContent>

        {/* Simulation tab — inputs + sub-tabs (details / breakdown) */}
        <TabsContent value="simulation" className="flex-1 min-h-0 flex flex-col overflow-hidden">
          <div className="px-4 pt-4 pb-2 space-y-3 shrink-0">
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
                <div className="w-16 shrink-0 space-y-1">
                  <label className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    Hold, h
                  </label>
                  <input
                    type="text"
                    inputMode="decimal"
                    value={holdHours}
                    onChange={(e) => setHoldHours(e.target.value)}
                    placeholder="24"
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
                <span className="mx-1 text-border">|</span>
                {HOLD_PRESETS.map((preset) => (
                  <button
                    key={preset.value}
                    type="button"
                    className={cn(
                      "rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium transition-colors",
                      holdHours === preset.value
                        ? "bg-violet-500/15 text-violet-600 dark:text-violet-400"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground"
                    )}
                    onClick={() => setHoldHours(preset.value)}
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

          {/* Sub-tabs: Details / Breakdown */}
           <Tabs defaultValue="sim-details" className="flex-1 min-h-0 flex flex-col gap-0">
            <div className="flex justify-end px-4 shrink-0">
              <TabsList className="h-6 p-0.5 text-[0.625rem]">
                <TabsTrigger value="sim-details" className="h-5 px-2 text-[0.625rem]">Summary</TabsTrigger>
                <TabsTrigger value="sim-breakdown" className="h-5 px-2 text-[0.625rem]">Breakdown</TabsTrigger>
              </TabsList>
            </div>

            {/* Sub-tab: Summary — calculation results grid */}
            <TabsContent value="sim-details" className="flex-1 min-h-0 overflow-y-auto">
              <div className="px-4 py-3">
                {simResult ? (
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
                    <span className="text-muted-foreground">Funding/{formatHoldLabel(simResult.hold_hours)}</span>
                    <span
                      className={cn(
                        "font-mono tabular-nums text-right",
                        parseFloat(simResult.funding_hold) >= 0
                          ? "text-emerald-600 dark:text-emerald-400"
                          : "text-red-600 dark:text-red-400"
                      )}
                    >
                      {formatUsd(simResult.funding_hold)}
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
                    <span className="text-muted-foreground">{formatHoldLabel(simResult.hold_hours)} P&L</span>
                    <span
                      className={cn(
                        "font-mono tabular-nums text-right font-medium",
                        parseFloat(simResult.pnl_hold) >= 0
                          ? "text-emerald-600 dark:text-emerald-400"
                          : "text-red-600 dark:text-red-400"
                      )}
                    >
                      {formatUsd(simResult.pnl_hold)}
                    </span>
                  </div>
                ) : (
                  <div className="text-xs text-muted-foreground">
                    Run a simulation to see calculation details.
                  </div>
                )}
              </div>
            </TabsContent>

            {/* Sub-tab: Breakdown — virtualized payment schedule */}
            <TabsContent value="sim-breakdown" className="flex-1 min-h-0 flex flex-col overflow-hidden">
              {activeRows.length > 0 && activeLoop ? (
                <>
                  <div className="px-4 py-2 shrink-0">
                    <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                      Hold ({activeLoop.duration_hours}h)
                      {" "}&mdash;{" "}
                      {activeRows.filter((r) => r.kind === "funding").length} payments
                      {" "}&mdash; Net:{" "}
                      <span
                        className={cn(
                          "font-mono",
                          parseFloat(activeLoop.net_pnl_pct) >= 0
                            ? "text-emerald-600 dark:text-emerald-400"
                            : "text-red-600 dark:text-red-400"
                        )}
                      >
                        {formatPct(String(parseFloat(activeLoop.net_pnl_pct) * 100))}
                      </span>
                      <span className="ml-1 text-muted-foreground font-mono">
                        ({formatUsd(activeLoop.net_pnl_usd)})
                      </span>
                    </div>
                  </div>
                  <div className="px-4 shrink-0">
                    <table className="text-xs w-full table-fixed">
                      <ScheduleColGroup />
                      <thead>
                        <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                          <th className="pr-3 pb-1 text-left font-medium">Time</th>
                          <th className="pr-3 pb-1 text-left font-medium">Hour</th>
                          <th className="pr-3 pb-1 text-left font-medium">Event</th>
                          <th className="pr-3 pb-1 text-right font-medium">Rate</th>
                          <th className="pr-3 pb-1 text-right font-medium">$ P&L</th>
                          <th className="pr-3 pb-1 text-right font-medium">$ Cum.</th>
                        </tr>
                      </thead>
                    </table>
                  </div>
                  <ScheduleVirtualList rows={activeRows} />
                </>
              ) : (
                <div className="px-4 py-3 text-xs text-muted-foreground">
                  Run a simulation to see the payment breakdown.
                </div>
              )}
            </TabsContent>
          </Tabs>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main table component
// ---------------------------------------------------------------------------

export function ArbitrageTable() {
  const router = useRouter();
  const [sort, setSort] = useState<SortState>({
    key: "funding_spread",
    desc: true,
  });
  const [selectedArbKey, setSelectedArbKey] = useState<string | null>(null);
  const [openPanel, setOpenPanel] = useState<"none" | "filters">("none");

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

  // Virtualization
  const ROW_HEIGHT = 40; // compact two sub-rows
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
          Failed to load arbitrage data: {error.message}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0">
      {/* Left: table area */}
      <div className="flex-1 min-w-0 flex flex-col">
        {/* Toolbar & filters — not scrollable */}
        <div className="px-4 pt-3 pb-2 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-screen-xl space-y-2">
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
              <div className="flex-[3.5] min-w-0 flex">
                <div className="flex-[1.5] min-w-0 px-2 py-1.5">Pair</div>
                <div className="flex-[2] min-w-0 px-2 py-1.5">Funding Rate</div>
              </div>
              <div className="px-2 py-1.5 min-w-[160px] flex-[2]">
                <SortButton
                  label="Funding Spread"
                  sortKey="funding_spread"
                  current={sort}
                  onToggle={toggleSort}
                />
              </div>
              <div className="px-2 py-1.5 min-w-[100px] flex-[1]">
                <SortButton
                  label="Open Spread, %"
                  sortKey="open_spread"
                  current={sort}
                  onToggle={toggleSort}
                />
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
                  : "No arbitrage opportunities found. Waiting for data from collector and worker..."}
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
              onNavigate={() => {
                const slug = encodeArbSlug(
                  selectedArb.symbol,
                  selectedArb.long.exchange_id,
                  selectedArb.short.exchange_id
                );
                router.push(`/arbitrage/${slug}`);
              }}
            />
          </div>
        </>
      )}

      {/* Open position dialog removed — navigation to /arbitrage/[slug] used instead */}
    </div>
  );
}
