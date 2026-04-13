"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useQuery } from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  fetchFundingArbitrage,
  type FundingArbitrage,
} from "@/lib/api";
import { formatPrice } from "@/lib/format";
import { ExchangeBadge } from "@/components/arbitrage/shared";
import {
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
  ArrowLeftRight,
  X,
  Search,
  Check,
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
  longPrice: string;
  shortPrice: string;
  lcmSpread: number;
  nextSpread: number;
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
      longPrice: arb.long.price,
      shortPrice: arb.short.price,
      lcmSpread: lcm,
      nextSpread: next,
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
      longPrice: arb.short.price,
      shortPrice: arb.long.price,
      lcmSpread: -lcm,
      nextSpread: -next,
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

  return (
    <div
      className={cn(
        "group/row cursor-pointer border-b transition-colors hover:bg-muted/50 text-[11px] leading-tight",
        isActive && "bg-violet-500/5 dark:bg-violet-500/10 border-violet-500/20"
      )}
      onClick={onClick}
    >
      <div className="flex max-w-4xl mx-auto">
        {/* Col 1: Symbol + Exchange badges */}
        <div className="flex flex-col justify-center px-3 py-2 min-w-[110px] flex-[1.2]">
          <div className="flex items-center gap-1 whitespace-nowrap font-medium text-xs">
            {isActive && (
              <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />
            )}
            {pair.symbol}
          </div>
          <div className="flex items-center gap-1 mt-0.5">
            <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
              <ArrowUp className="h-2 w-2 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={pair.longExchange} />
            <span className="text-muted-foreground/40 text-[9px]">/</span>
            <span className="inline-flex items-center justify-center h-3 w-3 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
              <ArrowDown className="h-2 w-2 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={pair.shortExchange} />
          </div>
        </div>

        {/* Col 2: Funding Spread */}
        <div className="flex flex-col justify-center px-3 py-2 min-w-[120px] flex-1 whitespace-nowrap">
          <span
            className={cn(
              "font-mono tabular-nums font-medium text-xs",
              spreadVal >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {formatPct(spreadVal)}
          </span>
          <div className="flex items-center gap-2 mt-0.5 text-[10px]">
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
            <span className="text-muted-foreground/40">/</span>
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
          </div>
        </div>

        {/* Col 3: Price Spread */}
        <div className="flex flex-col justify-center px-3 py-2 min-w-[100px] flex-1 whitespace-nowrap">
          <span className="font-mono tabular-nums font-medium text-xs text-muted-foreground">
            {formatPct(pair.openSpread)}
          </span>
          <div className="flex items-center gap-2 mt-0.5 text-[10px]">
            <span className="font-mono tabular-nums text-muted-foreground/70">
              {formatPrice(pair.longPrice)}
            </span>
            <span className="text-muted-foreground/40">/</span>
            <span className="font-mono tabular-nums text-muted-foreground/70">
              {formatPrice(pair.shortPrice)}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// PairSelectorPanel
// ---------------------------------------------------------------------------

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
      {/* Toolbar */}
      <div className="border-b shrink-0">
        <div className="flex items-center gap-1.5 px-3 py-2.5 max-w-4xl mx-auto">
          {/* Symbol search */}
          <div className="flex-1 min-w-0 relative">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground pointer-events-none z-10" />
            <input
              type="text"
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
              placeholder="Search symbol or exchange..."
              className="w-full h-7 rounded-md border bg-background pl-6 pr-2 text-xs outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
            />
          </div>

          {/* Spread mode */}
          <SpreadModeDropdown value={spreadMode} onChange={onSpreadModeChange} />

          {/* Close */}
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
      </div>

      {/* Table header */}
      <div className="border-b bg-background text-xs text-foreground font-medium shrink-0">
        <div className="flex items-center max-w-4xl mx-auto">
          <div className="px-3 py-2 min-w-[110px] flex-[1.2]">
            <SortButton
              label="Symbol"
              sortKey="symbol"
              current={sort}
              onToggle={toggleSort}
            />
          </div>
          <div className="px-3 py-2 min-w-[120px] flex-1">
            <SortButton
              label="F. Spread"
              sortKey="funding_spread"
              current={sort}
              onToggle={toggleSort}
            />
          </div>
          <div className="px-3 py-2 min-w-[100px] flex-1">
            <SortButton
              label="P. Spread"
              sortKey="open_spread"
              current={sort}
              onToggle={toggleSort}
            />
          </div>
        </div>
      </div>

      {/* Virtualized rows */}
      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto min-h-0">
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

  const { data: arbitrages } = useQuery({
    queryKey: ["funding-arbitrage"],
    queryFn: fetchFundingArbitrage,
  });

  // Build both directions for every pair
  const allPairs = useMemo(
    () => (arbitrages ? buildDirectionalPairs(arbitrages) : []),
    [arbitrages]
  );

  // Simple text search — no filtering applied, just substring match on symbol/exchanges
  const searchedPairs = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return allPairs;
    return allPairs.filter(
      (p) =>
        p.symbol.toLowerCase().includes(q) ||
        p.longExchange.toLowerCase().includes(q) ||
        p.shortExchange.toLowerCase().includes(q)
    );
  }, [allPairs, search]);

  const sortedPairs = useMemo(
    () => sortPairs(searchedPairs, sort, spreadMode),
    [searchedPairs, sort, spreadMode]
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
  };

  if (!open) return null;

  return createPortal(
    <>
      {/* Desktop: full-page overlay */}
      <div className="hidden md:flex fixed inset-0 z-50">
        <div
          className="absolute inset-0 bg-background/70 backdrop-blur-sm"
          onClick={() => onOpenChange(false)}
        />
        <div className="relative z-10 flex flex-col w-full h-full bg-background shadow-2xl animate-in fade-in-0 slide-in-from-top-1 duration-150">
          <PairSelectorPanel {...panelProps} />
        </div>
      </div>

      {/* Mobile: bottom sheet */}
      <div className="md:hidden fixed inset-0 z-50">
        <div
          className="absolute inset-0 bg-background/70 backdrop-blur-sm"
          onClick={() => onOpenChange(false)}
        />
        <div className="absolute bottom-0 left-0 right-0 h-[90dvh] bg-background border-t rounded-t-xl shadow-2xl flex flex-col animate-in slide-in-from-bottom duration-200">
          <div className="flex justify-center pt-2.5 pb-0.5 shrink-0">
            <div className="w-8 h-1 rounded-full bg-muted-foreground/30" />
          </div>
          <PairSelectorPanel {...panelProps} />
        </div>
      </div>
    </>,
    document.body
  );
}
