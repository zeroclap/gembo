"use client";

import { useQuery } from "@tanstack/react-query";
import { useState, useMemo, useEffect, useCallback, Fragment } from "react";
import { fetchFundingArbitrage, type FundingArbitrage } from "@/lib/api";
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
} from "lucide-react";
import { useArbitrageFilters } from "@/hooks/use-arbitrage-filters";
import { FilterBar } from "./filter-bar";

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

// ---------------------------------------------------------------------------
// Small components
// ---------------------------------------------------------------------------

function ExchangeBadge({ exchangeId }: { exchangeId: string }) {
  const colorMap: Record<string, string> = {
    bybit: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
    okx: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
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

function Countdown({ targetTime }: { targetTime: string }) {
  const [remaining, setRemaining] = useState("");

  useEffect(() => {
    function update() {
      const target = new Date(targetTime).getTime();
      const now = Date.now();
      const diff = Math.max(0, Math.floor((target - now) / 1000));

      if (diff === 0) {
        setRemaining("now");
        return;
      }

      const h = Math.floor(diff / 3600);
      const m = Math.floor((diff % 3600) / 60);
      const s = diff % 60;
      setRemaining(
        `${h}h ${m.toString().padStart(2, "0")}m ${s.toString().padStart(2, "0")}s`
      );
    }

    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [targetTime]);

  return <span className="font-mono tabular-nums">{remaining}</span>;
}

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

type SortKey = "symbol" | "funding_spread" | "open_spread";

interface SortState {
  key: SortKey;
  desc: boolean;
}

function getSortValue(arb: FundingArbitrage, key: SortKey): number | string {
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
// Expanded detail row (shown on click)
// ---------------------------------------------------------------------------

function ExpandedRow({ arb }: { arb: FundingArbitrage }) {
  const payments = arb.full_loop.payments ?? [];

  return (
    <div className="border-t bg-muted/30 px-4 py-3 space-y-3">
      {/* Leg details */}
      <div className="grid grid-cols-2 gap-4">
        {/* Long leg */}
        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5 text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            <TrendingUp className="h-3 w-3 text-emerald-500" />
            Long
            <ExchangeBadge exchangeId={arb.long.exchange_id} />
          </div>
          <div className="grid grid-cols-2 gap-x-4 gap-y-0.5 text-xs">
            <span className="text-muted-foreground">Bid / Ask</span>
            <span className="font-mono tabular-nums text-right">
              {formatPrice(arb.long.bid)} / {formatPrice(arb.long.ask)}
            </span>
            <span className="text-muted-foreground">Mark</span>
            <span className="font-mono tabular-nums text-right">
              {formatPrice(arb.long.mark_price)}
            </span>
            <span className="text-muted-foreground">Index</span>
            <span className="font-mono tabular-nums text-right">
              {formatPrice(arb.long.index_price)}
            </span>
            <span className="text-muted-foreground">Volume 24h</span>
            <span className="font-mono tabular-nums text-right">
              {formatVolume(arb.long.volume_24h)}
            </span>
            <span className="text-muted-foreground">Taker fee</span>
            <span className="font-mono tabular-nums text-right">
              {arb.long.taker_fee_pct}%
            </span>
            <span className="text-muted-foreground">Rate 8h</span>
            <span className="font-mono tabular-nums text-right">
              {formatRate(arb.long.rate_8h)}
            </span>
          </div>
        </div>

        {/* Short leg */}
        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5 text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            <TrendingDown className="h-3 w-3 text-red-500" />
            Short
            <ExchangeBadge exchangeId={arb.short.exchange_id} />
          </div>
          <div className="grid grid-cols-2 gap-x-4 gap-y-0.5 text-xs">
            <span className="text-muted-foreground">Bid / Ask</span>
            <span className="font-mono tabular-nums text-right">
              {formatPrice(arb.short.bid)} / {formatPrice(arb.short.ask)}
            </span>
            <span className="text-muted-foreground">Mark</span>
            <span className="font-mono tabular-nums text-right">
              {formatPrice(arb.short.mark_price)}
            </span>
            <span className="text-muted-foreground">Index</span>
            <span className="font-mono tabular-nums text-right">
              {formatPrice(arb.short.index_price)}
            </span>
            <span className="text-muted-foreground">Volume 24h</span>
            <span className="font-mono tabular-nums text-right">
              {formatVolume(arb.short.volume_24h)}
            </span>
            <span className="text-muted-foreground">Taker fee</span>
            <span className="font-mono tabular-nums text-right">
              {arb.short.taker_fee_pct}%
            </span>
            <span className="text-muted-foreground">Rate 8h</span>
            <span className="font-mono tabular-nums text-right">
              {formatRate(arb.short.rate_8h)}
            </span>
          </div>
        </div>
      </div>

      {/* Fees summary */}
      <div className="space-y-1">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Fee breakdown
        </div>
        <div className="flex items-center gap-6 text-xs">
          <div>
            <span className="text-muted-foreground">Open: </span>
            <span className="font-mono tabular-nums">
              {formatPct(arb.fees.open_fees_pct)}
            </span>
          </div>
          <div>
            <span className="text-muted-foreground">Close: </span>
            <span className="font-mono tabular-nums">
              {formatPct(arb.fees.close_fees_pct)}
            </span>
          </div>
          <div>
            <span className="text-muted-foreground">Round-trip: </span>
            <span className="font-mono tabular-nums">
              {formatPct(arb.fees.round_trip_fees_pct)}
            </span>
          </div>
          <div>
            <span className="text-muted-foreground">Net APR after fees: </span>
            <span
              className={cn(
                "font-mono tabular-nums font-medium",
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

      {/* Full loop payment schedule */}
      {payments.length > 0 && (
        <div className="space-y-1.5">
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            Full loop ({arb.full_loop.duration_hours}h) &mdash;{" "}
            {payments.length} payments &mdash; Net:{" "}
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
          </div>
          <div className="overflow-x-auto">
            <table className="text-xs">
              <thead>
                <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                  <th className="pr-3 pb-1 text-left font-medium">Hour</th>
                  <th className="pr-3 pb-1 text-left font-medium">Exchange</th>
                  <th className="pr-3 pb-1 text-left font-medium">Side</th>
                  <th className="pr-3 pb-1 text-right font-medium">Rate</th>
                  <th className="pr-3 pb-1 text-right font-medium">P&L</th>
                  <th className="pb-1 text-right font-medium">Cumulative</th>
                </tr>
              </thead>
              <tbody>
                {payments.map((p, i) => {
                  const cum = parseFloat(p.cumulative);
                  const pnl = parseFloat(p.payment_pct);
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
                          "py-0.5 text-right font-mono tabular-nums font-medium",
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
  );
}

// ---------------------------------------------------------------------------
// Two-line data row (rendered inside its own <tbody> for group hover)
// ---------------------------------------------------------------------------

const COL_COUNT = 5;

function DataRowPair({
  arb,
  isExpanded,
  onToggle,
}: {
  arb: FundingArbitrage;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const fundingSpreadVal = parseFloat(arb.full_loop.net_pct_per_loop) * 100;
  const upcomingVal = parseFloat(arb.next_payment.payment_pct) * 100;

  const longRate = parseFloat(arb.long.funding_rate) * 100;
  const shortRate = parseFloat(arb.short.funding_rate) * 100;

  return (
    <tbody
      className={cn(
        "group/row cursor-pointer border-b transition-colors hover:bg-muted/50",
        isExpanded && "bg-muted/20"
      )}
      onClick={onToggle}
    >
      {/* Line 1: Long leg */}
      <tr>
        {/* Symbol — spans both lines */}
        <td
          className="p-2 align-middle whitespace-nowrap font-medium"
          rowSpan={2}
        >
          {arb.symbol}
        </td>

        {/* Pair: Long — direction, exchange, price (right-aligned) */}
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

        {/* Funding Spread — spans both lines, two labeled rows */}
        <td
          className="p-2 align-middle whitespace-nowrap"
          rowSpan={2}
        >
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-1.5">
              <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider w-12">
                lcm
              </span>
              <span
                className={cn(
                  "font-mono tabular-nums font-medium",
                  fundingSpreadVal >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {fundingSpreadVal >= 0 ? "+" : ""}
                {fundingSpreadVal.toFixed(4)}%
              </span>
              <span className="text-[0.625rem] text-muted-foreground">
                {arb.full_loop.duration_hours}h
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider w-12">
                next
              </span>
              <span
                className={cn(
                  "font-mono tabular-nums",
                  upcomingVal >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {upcomingVal >= 0 ? "+" : ""}
                {upcomingVal.toFixed(4)}%
              </span>
              <span className="text-[0.625rem] text-muted-foreground">
                {arb.next_payment.side === "long"
                  ? arb.long.interval_hours
                  : arb.short.interval_hours}h
              </span>
            </div>
          </div>
        </td>

        {/* Open Spread — spans both lines */}
        <td
          className="p-2 align-middle whitespace-nowrap"
          rowSpan={2}
        >
          <span className="font-mono tabular-nums text-muted-foreground">
            {formatPct(arb.open_spread)}
          </span>
        </td>
      </tr>

      {/* Line 2: Short leg */}
      <tr>
        {/* Pair: Short — direction, exchange, price (right-aligned) */}
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

      {/* Expanded detail row */}
      {isExpanded && (
        <tr>
          <td colSpan={COL_COUNT} className="p-0">
            <ExpandedRow arb={arb} />
          </td>
        </tr>
      )}
    </tbody>
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
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

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

  const toggleSort = useCallback(
    (key: SortKey) => {
      setSort((prev) => ({
        key,
        desc: prev.key === key ? !prev.desc : true,
      }));
    },
    []
  );

  const toggleRow = useCallback((symbol: string) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(symbol)) {
        next.delete(symbol);
      } else {
        next.add(symbol);
      }
      return next;
    });
  }, []);

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
    <div className="space-y-4">
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

      <FilterBar
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
        activeFilterCount={activeFilterCount}
        availableSymbols={availableSymbols}
        availableExchanges={availableExchanges}
      />

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
                  colSpan={COL_COUNT}
                  className="h-24 text-center text-muted-foreground"
                >
                  {activeFilterCount > 0
                    ? "No results match the current filters."
                    : "No arbitrage opportunities found. Waiting for data from collector and worker..."}
                </TableCell>
              </TableRow>
            </TableBody>
          ) : (
            sortedData.map((arb) => (
              <DataRowPair
                key={arb.symbol}
                arb={arb}
                isExpanded={expandedRows.has(arb.symbol)}
                onToggle={() => toggleRow(arb.symbol)}
              />
            ))
          )}
        </Table>
      </div>
    </div>
  );
}
