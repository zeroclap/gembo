"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useQuery } from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  fetchFundingArbitrage,
  fetchPositions,
  type FundingArbitrage,
  type Position,
} from "@/lib/api";
import { formatPrice, formatCountdown } from "@/lib/format";
import { ExchangeBadge } from "@/components/arbitrage/shared";
import {
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
  ArrowLeftRight,
  X,
  Search,
  Check,
  Filter,
  Star,
} from "lucide-react";
import { cn } from "@/lib/utils";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export interface PairSelection {
  symbol: string;
  longExchange: string;
  shortExchange: string;
}

interface PairCommandProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelect: (pair: PairSelection) => void;
  currentPair?: PairSelection;
}

// ---------------------------------------------------------------------------
// Directional pair — one row per direction
// ---------------------------------------------------------------------------

interface DirectionalPair {
  symbol: string;
  longExchange: string;
  shortExchange: string;
  // metrics from the underlying arb entry (direction-adjusted)
  fundingSpread: number; // funding spread value for sorting
  openSpread: number;    // open spread (same either way)
  longRate: number;
  shortRate: number;
  longInterval: number;
  shortInterval: number;
  longNextFunding: string;
  shortNextFunding: string;
  longPrice: string;
  shortPrice: string;
  lcmSpread: number;
  nextSpread: number;
  recommended: boolean;
}

function buildDirectionalPairs(
  arbitrages: FundingArbitrage[]
): DirectionalPair[] {
  const pairs: DirectionalPair[] = [];

  for (const arb of arbitrages) {
    const lcm = parseFloat(arb.full_loop.net_pct_per_loop) * 100;
    const next = parseFloat(arb.next_payment.payment_pct) * 100;
    const openSpread = parseFloat(arb.open_spread);

    // Direction A: as-is (long = arb.long, short = arb.short)
    pairs.push({
      symbol: arb.symbol,
      longExchange: arb.long.exchange_id,
      shortExchange: arb.short.exchange_id,
      fundingSpread: lcm,
      openSpread,
      longRate: parseFloat(arb.long.funding_rate) * 100,
      shortRate: parseFloat(arb.short.funding_rate) * 100,
      longInterval: arb.long.interval_hours,
      shortInterval: arb.short.interval_hours,
      longNextFunding: arb.long.next_funding_at,
      shortNextFunding: arb.short.next_funding_at,
      longPrice: arb.long.price,
      shortPrice: arb.short.price,
      lcmSpread: lcm,
      nextSpread: next,
      recommended: arb.recommended,
    });

    // Direction B: swapped (long = arb.short, short = arb.long)
    pairs.push({
      symbol: arb.symbol,
      longExchange: arb.short.exchange_id,
      shortExchange: arb.long.exchange_id,
      fundingSpread: -lcm,
      openSpread: -openSpread,
      longRate: parseFloat(arb.short.funding_rate) * 100,
      shortRate: parseFloat(arb.long.funding_rate) * 100,
      longInterval: arb.short.interval_hours,
      shortInterval: arb.long.interval_hours,
      longNextFunding: arb.short.next_funding_at,
      shortNextFunding: arb.long.next_funding_at,
      longPrice: arb.short.price,
      shortPrice: arb.long.price,
      lcmSpread: -lcm,
      nextSpread: -next,
      recommended: false,
    });
  }

  return pairs;
}

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

function formatPct(value: number, decimals = 4): string {
  if (isNaN(value)) return "-";
  return `${value >= 0 ? "+" : ""}${value.toFixed(decimals)}%`;
}

// ---------------------------------------------------------------------------
// Spread mode
// ---------------------------------------------------------------------------

type SpreadMode = "lcm" | "next";

const SPREAD_MODE_LABELS: Record<SpreadMode, string> = {
  lcm: "LCM",
  next: "Next",
};

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

type SortKey = "symbol" | "funding_spread" | "open_spread";

interface SortState {
  key: SortKey;
  desc: boolean;
}

function getSortValue(pair: DirectionalPair, key: SortKey, spreadMode: SpreadMode): number | string {
  switch (key) {
    case "symbol":
      return pair.symbol;
    case "funding_spread":
      return spreadMode === "lcm" ? pair.lcmSpread : pair.nextSpread;
    case "open_spread":
      return pair.openSpread;
  }
}

function sortPairs(
  data: DirectionalPair[],
  sort: SortState,
  spreadMode: SpreadMode
): DirectionalPair[] {
  const sorted = [...data].sort((a, b) => {
    const va = getSortValue(a, sort.key, spreadMode);
    const vb = getSortValue(b, sort.key, spreadMode);
    if (typeof va === "string" && typeof vb === "string") {
      return va.localeCompare(vb);
    }
    return (va as number) - (vb as number);
  });
  if (sort.desc) sorted.reverse();
  return sorted;
}

// ---------------------------------------------------------------------------
// useDropdown — shared open/close with outside-click
// ---------------------------------------------------------------------------

function useDropdown() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false);
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  return { open, setOpen, ref };
}

function iconBtnCls(active: boolean) {
  return cn(
    "shrink-0 flex items-center justify-center h-7 w-7 rounded-md border transition-colors",
    active
      ? "border-primary/40 bg-primary/5 text-primary"
      : "border-transparent bg-background text-muted-foreground hover:bg-muted/60 hover:text-foreground"
  );
}

// ---------------------------------------------------------------------------
// SortButton
// ---------------------------------------------------------------------------

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
      type="button"
      className="flex items-center font-medium cursor-pointer select-none hover:text-foreground transition-colors"
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
// SpreadModeDropdown
// ---------------------------------------------------------------------------

function SpreadModeDropdown({
  value,
  onChange,
}: {
  value: SpreadMode;
  onChange: (v: SpreadMode) => void;
}) {
  const { open, setOpen, ref } = useDropdown();

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        title={`Spread mode: ${SPREAD_MODE_LABELS[value]}`}
        onClick={() => setOpen((v) => !v)}
        className={iconBtnCls(false)}
      >
        <ArrowLeftRight className="h-3.5 w-3.5" />
      </button>

      {open && (
        <div className="absolute right-0 top-full z-50 mt-1 min-w-[110px] rounded-md border bg-popover p-1 shadow-md">
          {(["lcm", "next"] as SpreadMode[]).map((mode) => (
            <button
              key={mode}
              type="button"
              className={cn(
                "flex w-full items-center gap-2 rounded-sm px-2 py-1 text-xs transition-colors hover:bg-accent hover:text-accent-foreground",
                value === mode
                  ? "text-foreground font-medium"
                  : "text-muted-foreground"
              )}
              onClick={() => {
                onChange(mode);
                setOpen(false);
              }}
            >
              <span
                className={cn(
                  "flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border",
                  value === mode
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-muted-foreground/40"
                )}
              >
                {value === mode && <Check className="h-2.5 w-2.5" />}
              </span>
              {SPREAD_MODE_LABELS[mode]}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ExchangeFilterDropdown
// ---------------------------------------------------------------------------

function ExchangeFilterDropdown({
  exchanges,
  selected,
  onToggle,
  onClear,
}: {
  exchanges: string[];
  selected: Set<string>;
  onToggle: (exchange: string) => void;
  onClear: () => void;
}) {
  const { open, setOpen, ref } = useDropdown();
  const hasFilter = selected.size > 0;

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        title={hasFilter ? `Exchanges: ${[...selected].join(", ")}` : "Filter by exchange"}
        onClick={() => setOpen((v) => !v)}
        className={iconBtnCls(hasFilter)}
      >
        <Filter className="h-3.5 w-3.5" />
      </button>

      {open && (
        <div className="absolute right-0 top-full z-50 mt-1 min-w-[130px] rounded-md border bg-popover p-1 shadow-md">
          <button
            type="button"
            className={cn(
              "flex w-full items-center gap-2 rounded-sm px-2 py-1 text-xs transition-colors hover:bg-accent hover:text-accent-foreground",
              !hasFilter
                ? "text-foreground font-medium"
                : "text-muted-foreground"
            )}
            onClick={onClear}
          >
            <span
              className={cn(
                "flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border",
                !hasFilter
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-muted-foreground/40"
              )}
            >
              {!hasFilter && <Check className="h-2.5 w-2.5" />}
            </span>
            All exchanges
          </button>
          {exchanges.map((ex) => {
            const active = selected.has(ex);
            return (
              <button
                key={ex}
                type="button"
                className={cn(
                  "flex w-full items-center gap-2 rounded-sm px-2 py-1 text-xs transition-colors hover:bg-accent hover:text-accent-foreground",
                  active
                    ? "text-foreground font-medium"
                    : "text-muted-foreground"
                )}
                onClick={() => onToggle(ex)}
              >
                <span
                  className={cn(
                    "flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border",
                    active
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-muted-foreground/40"
                  )}
                >
                  {active && <Check className="h-2.5 w-2.5" />}
                </span>
                <span className="uppercase">{ex}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// Grid templates shared between header and rows
const GRID_DESKTOP = "hidden md:grid grid-cols-[1fr_100px_90px_minmax(200px,1.5fr)_100px_100px]";
const GRID_MOBILE = "grid md:hidden grid-cols-[1fr_80px_80px]";

// ---------------------------------------------------------------------------
// LegLine — compact per-leg info for mobile sublines
// ---------------------------------------------------------------------------

function LegLine({
  side,
  exchangeId,
  rate,
  interval,
  nextFunding,
  price,
}: {
  side: "long" | "short";
  exchangeId: string;
  rate: number;
  interval: number;
  nextFunding: string;
  price: string;
}) {
  const isLong = side === "long";
  return (
    <span className="flex items-center gap-1 text-[10px]">
      <span
        className={cn(
          "inline-flex items-center justify-center h-3 w-3 rounded-sm shrink-0",
          isLong ? "bg-emerald-600 dark:bg-emerald-500" : "bg-red-600 dark:bg-red-500"
        )}
      >
        {isLong ? (
          <ArrowUp className="h-2 w-2 text-white" strokeWidth={2.5} />
        ) : (
          <ArrowDown className="h-2 w-2 text-white" strokeWidth={2.5} />
        )}
      </span>
      <ExchangeBadge exchangeId={exchangeId} />
      <span className="font-mono tabular-nums text-muted-foreground/70">{formatPrice(price)}</span>
      <span className="text-muted-foreground/30 select-none">·</span>
      <span
        className={cn(
          "font-mono tabular-nums",
          (isLong ? rate <= 0 : rate >= 0)
            ? "text-emerald-600/70 dark:text-emerald-400/70"
            : "text-red-600/70 dark:text-red-400/70"
        )}
      >
        {formatPct(rate)}
      </span>
      <span className="text-muted-foreground/50 font-mono">{interval}h</span>
      {nextFunding && (
        <>
          <span className="text-muted-foreground/30 select-none">·</span>
          <span className="text-muted-foreground/40 font-mono">{formatCountdown(nextFunding)}</span>
        </>
      )}
    </span>
  );
}

// ---------------------------------------------------------------------------
// DataRowPair
// ---------------------------------------------------------------------------

function DataRowPair({
  pair,
  isActive,
  spreadMode,
  onClick,
}: {
  pair: DirectionalPair;
  isActive: boolean;
  spreadMode: SpreadMode;
  onClick: () => void;
}) {
  const spreadVal = spreadMode === "lcm" ? pair.lcmSpread : pair.nextSpread;

  const activeCls = isActive && "bg-violet-500/5 dark:bg-violet-500/10 border-violet-500/20";

  return (
    <div
      className={cn(
        "group/row cursor-pointer border-b transition-colors hover:bg-muted/50 text-[11px] leading-tight",
        activeCls
      )}
      onClick={onClick}
    >
      {/* ── Mobile: 3 columns ── */}
      <div className={cn(GRID_MOBILE, "items-center")}>
        <div className="px-3 py-2 flex flex-col gap-0.5 min-w-0">
          <div className="flex items-center gap-1 whitespace-nowrap font-medium text-xs">
            {isActive && (
              <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />
            )}
            {pair.recommended && (
              <Star className="h-3 w-3 fill-amber-400 text-amber-400 shrink-0" />
            )}
            {pair.symbol}
          </div>
          <LegLine
            side="long"
            exchangeId={pair.longExchange}
            rate={pair.longRate}
            interval={pair.longInterval}
            nextFunding={pair.longNextFunding}
            price={pair.longPrice}
          />
          <LegLine
            side="short"
            exchangeId={pair.shortExchange}
            rate={pair.shortRate}
            interval={pair.shortInterval}
            nextFunding={pair.shortNextFunding}
            price={pair.shortPrice}
          />
        </div>

        <div className="px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs">
          <span
            className={cn(
              spreadVal >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {formatPct(spreadVal)}
          </span>
        </div>

        <div className={cn(
          "px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs",
          pair.openSpread > 0 ? "text-emerald-600 dark:text-emerald-400" :
          pair.openSpread < 0 ? "text-red-600 dark:text-red-400" : "text-muted-foreground"
        )}>
          {formatPct(pair.openSpread)}
        </div>
      </div>

      {/* ── Desktop: 6 columns ── */}
      <div className={cn(GRID_DESKTOP, "items-center")}>
        {/* Symbol */}
        <div className="px-3 py-2 flex items-center gap-1 whitespace-nowrap font-medium text-xs">
          {isActive && (
            <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />
          )}
          {pair.recommended && (
            <Star className="h-3 w-3 fill-amber-400 text-amber-400 shrink-0" />
          )}
          {pair.symbol}
        </div>

        {/* Exchanges */}
        <div className="px-2 py-2 flex flex-col text-[10px]">
          <span className="flex items-center gap-1">
            <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
              <ArrowUp className="h-2 w-2 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={pair.longExchange} />
          </span>
          <span className="flex items-center gap-1">
            <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
              <ArrowDown className="h-2 w-2 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={pair.shortExchange} />
          </span>
        </div>

        {/* Price */}
        <div className="px-2 py-2 flex flex-col whitespace-nowrap text-[10px] font-mono tabular-nums text-muted-foreground/70">
          <span>{formatPrice(pair.longPrice)}</span>
          <span>{formatPrice(pair.shortPrice)}</span>
        </div>

        {/* Funding Rate */}
        <div className="px-2 py-2 flex flex-col whitespace-nowrap text-[10px]">
          <span className="flex items-center gap-1">
            <span
              className={cn(
                "font-mono tabular-nums",
                pair.longRate <= 0
                  ? "text-emerald-600/70 dark:text-emerald-400/70"
                  : "text-red-600/70 dark:text-red-400/70"
              )}
            >
              {formatPct(pair.longRate)}
            </span>
            <span className="text-muted-foreground/50 font-mono">{pair.longInterval}h</span>
            {pair.longNextFunding && (
              <>
                <span className="text-muted-foreground/30 select-none">·</span>
                <span className="text-muted-foreground/40 font-mono">{formatCountdown(pair.longNextFunding)}</span>
              </>
            )}
          </span>
          <span className="flex items-center gap-1">
            <span
              className={cn(
                "font-mono tabular-nums",
                pair.shortRate >= 0
                  ? "text-emerald-600/70 dark:text-emerald-400/70"
                  : "text-red-600/70 dark:text-red-400/70"
              )}
            >
              {formatPct(pair.shortRate)}
            </span>
            <span className="text-muted-foreground/50 font-mono">{pair.shortInterval}h</span>
            {pair.shortNextFunding && (
              <>
                <span className="text-muted-foreground/30 select-none">·</span>
                <span className="text-muted-foreground/40 font-mono">{formatCountdown(pair.shortNextFunding)}</span>
              </>
            )}
          </span>
        </div>

        {/* F. Spread */}
        <div className="px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs">
          <span
            className={cn(
              spreadVal >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {formatPct(spreadVal)}
          </span>
        </div>

        {/* P. Spread */}
        <div className={cn(
          "px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs",
          pair.openSpread > 0 ? "text-emerald-600 dark:text-emerald-400" :
          pair.openSpread < 0 ? "text-red-600 dark:text-red-400" : "text-muted-foreground"
        )}>
          {formatPct(pair.openSpread)}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// PairSelectorPanel
// ---------------------------------------------------------------------------

const ACTIVE_POSITION_STATUSES = new Set([
  "pending",
  "opening",
  "open",
  "partially_open",
  "closing",
]);


interface PairSelectorPanelProps {
  data: DirectionalPair[];
  sort: SortState;
  activeKey: string | null;
  onSelect: (pair: DirectionalPair) => void;
  onClose: () => void;
  toggleSort: (key: SortKey) => void;
  spreadMode: SpreadMode;
  onSpreadModeChange: (mode: SpreadMode) => void;
  search: string;
  onSearchChange: (v: string) => void;
  exchanges: string[];
  exchangeFilter: Set<string>;
  onToggleExchange: (exchange: string) => void;
  onClearExchangeFilter: () => void;
  ongoingPositions: Position[];
  onSelectPosition: (position: Position) => void;
}

function PairSelectorPanel({
  data,
  sort,
  activeKey,
  onSelect,
  onClose,
  toggleSort,
  spreadMode,
  onSpreadModeChange,
  search,
  onSearchChange,
  exchanges,
  exchangeFilter,
  onToggleExchange,
  onClearExchangeFilter,
  ongoingPositions,
  onSelectPosition,
}: PairSelectorPanelProps) {
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const ROW_HEIGHT = 52;
  const virtualizer = useVirtualizer({
    count: data.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  });

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* Unified header: toolbar + column headers, one bottom border */}
      <div className="border-b shrink-0">
        {/* Toolbar row */}
        <div className="flex items-center gap-1.5 px-3 pt-2.5 pb-2 max-w-4xl mx-auto">
          <div className="flex-1 min-w-0 relative">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground pointer-events-none z-10" />
            <input
              type="text"
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
              placeholder="Search symbol..."
              className="w-full h-7 rounded-md border bg-background pl-6 pr-2 text-xs outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
            />
          </div>
          <ExchangeFilterDropdown
            exchanges={exchanges}
            selected={exchangeFilter}
            onToggle={onToggleExchange}
            onClear={onClearExchangeFilter}
          />
          <SpreadModeDropdown value={spreadMode} onChange={onSpreadModeChange} />
          <button
            type="button"
            onClick={onClose}
            title="Close"
            className={iconBtnCls(false)}
            aria-label="Close"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>

        {/* Column header row */}
        <div className="text-xs text-foreground font-medium border-t">
          <div className={cn(GRID_MOBILE, "items-center")}>
            <div className="px-3 py-1.5">
              <SortButton label="Symbol" sortKey="symbol" current={sort} onToggle={toggleSort} />
            </div>
            <div className="px-2 py-1.5">
              <SortButton label="F. Spread" sortKey="funding_spread" current={sort} onToggle={toggleSort} />
            </div>
            <div className="px-2 py-1.5">
              <SortButton label="P. Spread" sortKey="open_spread" current={sort} onToggle={toggleSort} />
            </div>
          </div>
          <div className={cn(GRID_DESKTOP, "items-center")}>
            <div className="px-3 py-1.5">
              <SortButton label="Symbol" sortKey="symbol" current={sort} onToggle={toggleSort} />
            </div>
            <div className="px-2 py-1.5">Exchanges</div>
            <div className="px-2 py-1.5">Price</div>
            <div className="px-2 py-1.5">Funding Rate</div>
            <div className="px-2 py-1.5">
              <SortButton label="F. Spread" sortKey="funding_spread" current={sort} onToggle={toggleSort} />
            </div>
            <div className="px-2 py-1.5">
              <SortButton label="P. Spread" sortKey="open_spread" current={sort} onToggle={toggleSort} />
            </div>
          </div>
        </div>
      </div>

      {/* Scrollable body */}
      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto min-h-0">
        {/* Ongoing positions pinned at top */}
        {ongoingPositions.length > 0 && (
          <>
            <div className="flex items-center gap-2 px-3 py-1.5 border-b bg-muted/30">
              <span className="text-[0.6rem] font-medium text-muted-foreground uppercase tracking-wider">Open positions</span>
            </div>
            {ongoingPositions.map((pos) => {
              const posKey = `${pos.symbol}::${pos.long_exchange_id}::${pos.short_exchange_id}`;
              const isActive = activeKey === posKey;

              // Extract per-leg funding details from snapshot for the funding-rate column.
              let longRate: number | null = null;
              let shortRate: number | null = null;
              let longInterval: number | null = null;
              let shortInterval: number | null = null;
              let longNextFunding: string | null = null;
              let shortNextFunding: string | null = null;
              if (snap && "long" in snap && snap.long && snap.short) {
                const dir = snap.long.exchange_id === pos.long_exchange_id ? 1 : -1;
                const snapLong  = dir === 1 ? snap.long  : snap.short;
                const snapShort = dir === 1 ? snap.short : snap.long;
                longRate        = parseFloat(snapLong.funding_rate)  * 100;
                shortRate       = parseFloat(snapShort.funding_rate) * 100;
                longInterval    = snapLong.interval_hours;
                shortInterval   = snapShort.interval_hours;
                longNextFunding  = snapLong.next_funding_at  || null;
                shortNextFunding = snapShort.next_funding_at || null;
              }
              let fSpread: number | null = null;
              let pSpread: number | null = null;
              const snap = pos.snapshot as FundingArbitrage | null | Record<string, never>;
              if (snap && "full_loop" in snap && snap.full_loop) {
                const dir = snap.long?.exchange_id === pos.long_exchange_id ? 1 : -1;
                const lcm = parseFloat(snap.full_loop.net_pct_per_loop) * 100 * dir;
                const next = snap.next_payment
                  ? parseFloat(snap.next_payment.payment_pct) * 100 * dir
                  : lcm;
                fSpread = spreadMode === "lcm" ? lcm : next;
                pSpread = parseFloat(snap.open_spread) * dir;
              }
              return (
                <div
                  key={pos.id}
                  className={cn(
                    "group/row cursor-pointer border-b border-l-2 transition-colors hover:bg-muted/50 text-[11px] leading-tight",
                    isActive
                      ? "bg-violet-500/5 dark:bg-violet-500/10 border-l-violet-500 border-violet-500/20"
                      : "border-l-emerald-500 hover:bg-muted/50"
                  )}
                  onClick={() => onSelectPosition(pos)}
                >
                  {/* Mobile */}
                  <div className={cn(GRID_MOBILE, "items-center")}>
                    <div className="px-3 py-2 flex flex-col gap-0.5 min-w-0">
                      <div className="flex items-center gap-1 whitespace-nowrap font-medium text-xs">
                        {isActive && <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />}
                        {pos.symbol}
                      </div>
                      <span className="flex items-center gap-1 text-[10px]">
                        <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                          <ArrowUp className="h-2 w-2 text-white" strokeWidth={2.5} />
                        </span>
                        <ExchangeBadge exchangeId={pos.long_exchange_id} />
                        <span className="font-mono tabular-nums text-muted-foreground/70">{formatPrice(pos.long_mark_price)}</span>
                        {longRate !== null && (
                          <>
                            <span className="text-muted-foreground/30 select-none">·</span>
                            <span className={cn("font-mono tabular-nums", longRate <= 0 ? "text-emerald-600/70 dark:text-emerald-400/70" : "text-red-600/70 dark:text-red-400/70")}>{formatPct(longRate)}</span>
                            {longInterval !== null && <span className="text-muted-foreground/50 font-mono">{longInterval}h</span>}
                          </>
                        )}
                      </span>
                      <span className="flex items-center gap-1 text-[10px]">
                        <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                          <ArrowDown className="h-2 w-2 text-white" strokeWidth={2.5} />
                        </span>
                        <ExchangeBadge exchangeId={pos.short_exchange_id} />
                        <span className="font-mono tabular-nums text-muted-foreground/70">{formatPrice(pos.short_mark_price)}</span>
                        {shortRate !== null && (
                          <>
                            <span className="text-muted-foreground/30 select-none">·</span>
                            <span className={cn("font-mono tabular-nums", shortRate >= 0 ? "text-emerald-600/70 dark:text-emerald-400/70" : "text-red-600/70 dark:text-red-400/70")}>{formatPct(shortRate)}</span>
                            {shortInterval !== null && <span className="text-muted-foreground/50 font-mono">{shortInterval}h</span>}
                          </>
                        )}
                      </span>
                    </div>
                    <div className="px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs">
                      {fSpread !== null
                        ? <span className={fSpread >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"}>{formatPct(fSpread)}</span>
                        : <span className="text-muted-foreground">—</span>}
                    </div>
                    <div className="px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs">
                      {pSpread !== null
                        ? <span className={pSpread > 0 ? "text-emerald-600 dark:text-emerald-400" : pSpread < 0 ? "text-red-600 dark:text-red-400" : "text-muted-foreground"}>{formatPct(pSpread)}</span>
                        : <span className="text-muted-foreground">—</span>}
                    </div>
                  </div>

                  {/* Desktop */}
                  <div className={cn(GRID_DESKTOP, "items-center")}>
                    {/* Symbol */}
                    <div className="px-3 py-2 flex items-center gap-1 whitespace-nowrap font-medium text-xs">
                      {isActive && <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />}
                      {pos.symbol}
                    </div>
                    {/* Exchanges */}
                    <div className="px-2 py-2 flex flex-col text-[10px]">
                      <span className="flex items-center gap-1">
                        <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                          <ArrowUp className="h-2 w-2 text-white" strokeWidth={2.5} />
                        </span>
                        <ExchangeBadge exchangeId={pos.long_exchange_id} />
                      </span>
                      <span className="flex items-center gap-1">
                        <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                          <ArrowDown className="h-2 w-2 text-white" strokeWidth={2.5} />
                        </span>
                        <ExchangeBadge exchangeId={pos.short_exchange_id} />
                      </span>
                    </div>
                    {/* Mark price / entry price */}
                    <div className="px-2 py-2 flex flex-col whitespace-nowrap text-[10px] font-mono tabular-nums">
                      <span className="text-muted-foreground/70">{formatPrice(pos.long_mark_price)}</span>
                      <span className="text-muted-foreground/70">{formatPrice(pos.short_mark_price)}</span>
                    </div>
                    {/* Funding rates */}
                    <div className="px-2 py-2 flex flex-col whitespace-nowrap text-[10px]">
                      <span className="flex items-center gap-1">
                        {longRate !== null ? (
                          <>
                            <span className={cn("font-mono tabular-nums", longRate <= 0 ? "text-emerald-600/70 dark:text-emerald-400/70" : "text-red-600/70 dark:text-red-400/70")}>
                              {formatPct(longRate)}
                            </span>
                            {longInterval !== null && <span className="text-muted-foreground/50 font-mono">{longInterval}h</span>}
                            {longNextFunding && (
                              <>
                                <span className="text-muted-foreground/30 select-none">·</span>
                                <span className="text-muted-foreground/40 font-mono">{formatCountdown(longNextFunding)}</span>
                              </>
                            )}
                          </>
                        ) : <span className="text-muted-foreground/40">—</span>}
                      </span>
                      <span className="flex items-center gap-1">
                        {shortRate !== null ? (
                          <>
                            <span className={cn("font-mono tabular-nums", shortRate >= 0 ? "text-emerald-600/70 dark:text-emerald-400/70" : "text-red-600/70 dark:text-red-400/70")}>
                              {formatPct(shortRate)}
                            </span>
                            {shortInterval !== null && <span className="text-muted-foreground/50 font-mono">{shortInterval}h</span>}
                            {shortNextFunding && (
                              <>
                                <span className="text-muted-foreground/30 select-none">·</span>
                                <span className="text-muted-foreground/40 font-mono">{formatCountdown(shortNextFunding)}</span>
                              </>
                            )}
                          </>
                        ) : <span className="text-muted-foreground/40">—</span>}
                      </span>
                    </div>
                    <div className="px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs">
                      {fSpread !== null
                        ? <span className={fSpread >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"}>{formatPct(fSpread)}</span>
                        : <span className="text-muted-foreground">—</span>}
                    </div>
                    <div className="px-2 py-2 whitespace-nowrap font-mono tabular-nums font-medium text-xs">
                      {pSpread !== null
                        ? <span className={pSpread > 0 ? "text-emerald-600 dark:text-emerald-400" : pSpread < 0 ? "text-red-600 dark:text-red-400" : "text-muted-foreground"}>{formatPct(pSpread)}</span>
                        : <span className="text-muted-foreground">—</span>}
                    </div>
                  </div>
                </div>
              );
            })}
            {/* Divider before main list */}
            <div className="flex items-center gap-2 px-3 py-1.5 border-b bg-muted/30">
              <span className="text-[0.6rem] font-medium text-muted-foreground uppercase tracking-wider">All pairs</span>
            </div>
          </>
        )}

        {/* Virtualized main list */}
        {data.length === 0 ? (
          <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
            {search
              ? "No pairs match your search."
              : "No pairs found. Waiting for data..."}
          </div>
        ) : (
          <div
            style={{
              height: virtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const pair = data[virtualRow.index];
              const key = `${pair.symbol}::${pair.longExchange}::${pair.shortExchange}`;
              return (
                <div
                  key={key}
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
                    pair={pair}
                    isActive={activeKey === key}
                    spreadMode={spreadMode}
                    onClick={() => onSelect(pair)}
                  />
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// PairCommand — exported component
// ---------------------------------------------------------------------------

export function PairCommand({
  open,
  onOpenChange,
  onSelect,
  currentPair,
}: PairCommandProps) {
  const [sort, setSort] = useState<SortState>({
    key: "funding_spread",
    desc: true,
  });
  const [spreadMode, setSpreadMode] = useState<SpreadMode>("lcm");
  const [search, setSearch] = useState("");
  const [exchangeFilter, setExchangeFilter] = useState<Set<string>>(new Set());

  const { data: arbitrages } = useQuery({
    queryKey: ["funding-arbitrage"],
    queryFn: fetchFundingArbitrage,
  });

  const { data: allPositions } = useQuery({
    queryKey: ["positions"],
    queryFn: () => fetchPositions(),
    enabled: open,
  });

  const ongoingPositions = useMemo(
    () => (allPositions ?? []).filter((p) => ACTIVE_POSITION_STATUSES.has(p.status)),
    [allPositions]
  );

  // Build both directions for every pair
  const allPairs = useMemo(
    () => (arbitrages ? buildDirectionalPairs(arbitrages) : []),
    [arbitrages]
  );

  // Unique exchange list for filter dropdown
  const exchanges = useMemo(() => {
    const set = new Set<string>();
    for (const p of allPairs) {
      set.add(p.longExchange);
      set.add(p.shortExchange);
    }
    return [...set].sort();
  }, [allPairs]);

  // Filter by symbol text + exchange dropdown
  const filteredPairs = useMemo(() => {
    const q = search.trim().toLowerCase();
    return allPairs.filter((p) => {
      if (q && !p.symbol.toLowerCase().includes(q)) return false;
      if (exchangeFilter.size === 1) {
        if (!exchangeFilter.has(p.longExchange) && !exchangeFilter.has(p.shortExchange))
          return false;
      } else if (exchangeFilter.size >= 2) {
        if (!exchangeFilter.has(p.longExchange) || !exchangeFilter.has(p.shortExchange))
          return false;
      }
      return true;
    });
  }, [allPairs, search, exchangeFilter]);

  const sortedPairs = useMemo(
    () => sortPairs(filteredPairs, sort, spreadMode),
    [filteredPairs, sort, spreadMode]
  );

  // Escape + body scroll lock
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onOpenChange(false);
    };
    document.addEventListener("keydown", handler);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", handler);
      document.body.style.overflow = "";
    };
  }, [open, onOpenChange]);

  const activeKey = currentPair
    ? `${currentPair.symbol}::${currentPair.longExchange}::${currentPair.shortExchange}`
    : null;

  const toggleSort = useCallback((key: SortKey) => {
    setSort((prev) => ({
      key,
      desc: prev.key === key ? !prev.desc : true,
    }));
  }, []);

  const handleSelect = useCallback(
    (pair: DirectionalPair) => {
      onSelect({
        symbol: pair.symbol,
        longExchange: pair.longExchange,
        shortExchange: pair.shortExchange,
      });
      onOpenChange(false);
    },
    [onSelect, onOpenChange]
  );

  const handleSelectPosition = useCallback(
    (pos: Position) => {
      onSelect({
        symbol: pos.symbol,
        longExchange: pos.long_exchange_id,
        shortExchange: pos.short_exchange_id,
      });
      onOpenChange(false);
    },
    [onSelect, onOpenChange]
  );

  const panelProps: PairSelectorPanelProps = {
    data: sortedPairs,
    sort,
    activeKey,
    onSelect: handleSelect,
    onClose: () => onOpenChange(false),
    toggleSort,
    spreadMode,
    onSpreadModeChange: setSpreadMode,
    search,
    onSearchChange: setSearch,
    exchanges,
    exchangeFilter,
    onToggleExchange: (ex: string) => {
      setExchangeFilter((prev) => {
        const next = new Set(prev);
        if (next.has(ex)) next.delete(ex);
        else next.add(ex);
        return next;
      });
    },
    onClearExchangeFilter: () => setExchangeFilter(new Set()),
    ongoingPositions,
    onSelectPosition: handleSelectPosition,
  };

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex">
      <div
        className="absolute inset-0 bg-background/70 backdrop-blur-sm"
        onClick={() => onOpenChange(false)}
      />
      <div className="relative z-10 flex flex-col w-full h-full bg-background shadow-2xl animate-in fade-in-0 slide-in-from-top-1 duration-150">
        <PairSelectorPanel {...panelProps} />
      </div>
    </div>,
    document.body
  );
}
