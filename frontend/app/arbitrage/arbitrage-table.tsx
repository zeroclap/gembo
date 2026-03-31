"use client";

import { useQuery } from "@tanstack/react-query";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { useState, useMemo, useEffect } from "react";
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
import { ArrowUpDown, ArrowUp, ArrowDown, Clock, TrendingUp, TrendingDown } from "lucide-react";

function formatPct(value: string, decimals = 4): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(decimals)}%`;
}

function formatRate(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  // Rate is a fraction, multiply by 100 for display.
  const pct = num * 100;
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
}

function formatAPR(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(2)}%`;
}

function formatPrice(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "-";
  if (num >= 1) return num.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  if (num >= 0.01) return num.toLocaleString(undefined, { minimumFractionDigits: 4, maximumFractionDigits: 4 });
  return num.toLocaleString(undefined, { minimumFractionDigits: 6, maximumFractionDigits: 8 });
}

function formatVolume(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  if (num >= 1_000_000_000) return `${(num / 1_000_000_000).toFixed(2)}B`;
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(2)}M`;
  if (num >= 1_000) return `${(num / 1_000).toFixed(2)}K`;
  return num.toFixed(2);
}

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
      setRemaining(`${h}h ${m.toString().padStart(2, "0")}m ${s.toString().padStart(2, "0")}s`);
    }

    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [targetTime]);

  return <span className="font-mono tabular-nums">{remaining}</span>;
}

function SortIcon({ column }: { column: { getIsSorted: () => false | "asc" | "desc" } }) {
  const sorted = column.getIsSorted();
  if (sorted === "asc") return <ArrowUp className="ml-1 h-3 w-3" />;
  if (sorted === "desc") return <ArrowDown className="ml-1 h-3 w-3" />;
  return <ArrowUpDown className="ml-1 h-3 w-3 opacity-30" />;
}

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
            <span className="text-muted-foreground">Rate</span>
            <span className="font-mono tabular-nums text-right">{formatRate(arb.long.funding_rate)}</span>
            <span className="text-muted-foreground">Interval</span>
            <span className="font-mono tabular-nums text-right">{arb.long.interval_hours}h</span>
            <span className="text-muted-foreground">Rate 8h</span>
            <span className="font-mono tabular-nums text-right">{formatRate(arb.long.rate_8h)}</span>
            <span className="text-muted-foreground">Price</span>
            <span className="font-mono tabular-nums text-right">{formatPrice(arb.long.price)}</span>
            <span className="text-muted-foreground">Bid / Ask</span>
            <span className="font-mono tabular-nums text-right">{formatPrice(arb.long.bid)} / {formatPrice(arb.long.ask)}</span>
            <span className="text-muted-foreground">Mark</span>
            <span className="font-mono tabular-nums text-right">{formatPrice(arb.long.mark_price)}</span>
            <span className="text-muted-foreground">Volume 24h</span>
            <span className="font-mono tabular-nums text-right">{formatVolume(arb.long.volume_24h)}</span>
            <span className="text-muted-foreground">Taker fee</span>
            <span className="font-mono tabular-nums text-right">{arb.long.taker_fee_pct}%</span>
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
            <span className="text-muted-foreground">Rate</span>
            <span className="font-mono tabular-nums text-right">{formatRate(arb.short.funding_rate)}</span>
            <span className="text-muted-foreground">Interval</span>
            <span className="font-mono tabular-nums text-right">{arb.short.interval_hours}h</span>
            <span className="text-muted-foreground">Rate 8h</span>
            <span className="font-mono tabular-nums text-right">{formatRate(arb.short.rate_8h)}</span>
            <span className="text-muted-foreground">Price</span>
            <span className="font-mono tabular-nums text-right">{formatPrice(arb.short.price)}</span>
            <span className="text-muted-foreground">Bid / Ask</span>
            <span className="font-mono tabular-nums text-right">{formatPrice(arb.short.bid)} / {formatPrice(arb.short.ask)}</span>
            <span className="text-muted-foreground">Mark</span>
            <span className="font-mono tabular-nums text-right">{formatPrice(arb.short.mark_price)}</span>
            <span className="text-muted-foreground">Volume 24h</span>
            <span className="font-mono tabular-nums text-right">{formatVolume(arb.short.volume_24h)}</span>
            <span className="text-muted-foreground">Taker fee</span>
            <span className="font-mono tabular-nums text-right">{arb.short.taker_fee_pct}%</span>
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
            <span className="font-mono tabular-nums">{formatPct(arb.fees.open_fees_pct)}</span>
          </div>
          <div>
            <span className="text-muted-foreground">Close: </span>
            <span className="font-mono tabular-nums">{formatPct(arb.fees.close_fees_pct)}</span>
          </div>
          <div>
            <span className="text-muted-foreground">Round-trip: </span>
            <span className="font-mono tabular-nums">{formatPct(arb.fees.round_trip_fees_pct)}</span>
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

      {/* Next payment */}
      <div className="space-y-1">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Next funding payment
        </div>
        <div className="flex items-center gap-4 text-xs">
          <ExchangeBadge exchangeId={arb.next_payment.exchange_id} />
          <span className="text-muted-foreground">{arb.next_payment.side}</span>
          <span className="font-mono tabular-nums">rate: {formatRate(arb.next_payment.rate)}</span>
          <span
            className={cn(
              "font-mono tabular-nums font-medium",
              parseFloat(arb.next_payment.payment_pct) >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            P&L: {formatRate(arb.next_payment.payment_pct)}
          </span>
          <div className="flex items-center gap-1 text-muted-foreground">
            <Clock className="h-3 w-3" />
            <Countdown targetTime={arb.next_payment.at} />
          </div>
        </div>
      </div>

      {/* Full loop payment schedule */}
      {payments.length > 0 && (
        <div className="space-y-1.5">
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            Full loop ({arb.full_loop.duration_hours}h) &mdash; {payments.length} payments &mdash; Net:{" "}
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
                      <td className="pr-3 py-0.5 font-mono tabular-nums">{p.at_hour}h</td>
                      <td className="pr-3 py-0.5">
                        <ExchangeBadge exchangeId={p.exchange_id} />
                      </td>
                      <td className="pr-3 py-0.5 text-muted-foreground">{p.side}</td>
                      <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                        {formatRate(p.rate)}
                      </td>
                      <td
                        className={cn(
                          "pr-3 py-0.5 text-right font-mono tabular-nums",
                          pnl >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
                        )}
                      >
                        {formatRate(p.payment_pct)}
                      </td>
                      <td
                        className={cn(
                          "py-0.5 text-right font-mono tabular-nums font-medium",
                          cum >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
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

export function ArbitrageTable() {
  const [sorting, setSorting] = useState<SortingState>([
    { id: "apr", desc: true },
  ]);
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  const { data: arbitrages, isLoading, error } = useQuery({
    queryKey: ["funding-arbitrage"],
    queryFn: fetchFundingArbitrage,
  });

  function toggleRow(symbol: string) {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(symbol)) {
        next.delete(symbol);
      } else {
        next.add(symbol);
      }
      return next;
    });
  }

  const columns = useMemo<ColumnDef<FundingArbitrage>[]>(
    () => [
      {
        accessorKey: "symbol",
        header: ({ column }) => (
          <button
            className="flex items-center font-medium"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            Symbol
            <SortIcon column={column} />
          </button>
        ),
        cell: ({ row }) => (
          <span className="font-medium">{row.original.symbol}</span>
        ),
      },
      {
        id: "direction",
        header: "Direction",
        cell: ({ row }) => (
          <div className="flex items-center gap-1 text-xs">
            <span className="text-emerald-600 dark:text-emerald-400">L</span>
            <ExchangeBadge exchangeId={row.original.long.exchange_id} />
            <span className="text-muted-foreground">/</span>
            <span className="text-red-600 dark:text-red-400">S</span>
            <ExchangeBadge exchangeId={row.original.short.exchange_id} />
          </div>
        ),
      },
      {
        id: "net_rate_8h",
        header: ({ column }) => (
          <button
            className="flex items-center font-medium"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            Net Rate 8h
            <SortIcon column={column} />
          </button>
        ),
        accessorFn: (row) => parseFloat(row.net_rate_8h),
        cell: ({ row }) => {
          const val = parseFloat(row.original.net_rate_8h);
          return (
            <span
              className={cn(
                "font-mono tabular-nums",
                val >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
              )}
            >
              {formatRate(row.original.net_rate_8h)}
            </span>
          );
        },
      },
      {
        id: "apr",
        header: ({ column }) => (
          <button
            className="flex items-center font-medium"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            APR
            <SortIcon column={column} />
          </button>
        ),
        accessorFn: (row) => parseFloat(row.apr),
        cell: ({ row }) => {
          const val = parseFloat(row.original.apr);
          return (
            <span
              className={cn(
                "font-mono tabular-nums font-medium",
                val >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
              )}
            >
              {formatAPR(row.original.apr)}
            </span>
          );
        },
      },
      {
        id: "spread",
        header: ({ column }) => (
          <button
            className="flex items-center font-medium"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            Spread
            <SortIcon column={column} />
          </button>
        ),
        accessorFn: (row) => parseFloat(row.open_spread),
        cell: ({ row }) => (
          <span className="font-mono tabular-nums text-muted-foreground">
            {formatPct(row.original.open_spread)}
          </span>
        ),
      },
      {
        id: "fees",
        header: "Fees RT",
        cell: ({ row }) => (
          <span className="font-mono tabular-nums text-muted-foreground">
            {formatPct(row.original.fees.round_trip_fees_pct)}
          </span>
        ),
      },
      {
        id: "next_payment",
        header: "Next Payment",
        cell: ({ row }) => {
          const np = row.original.next_payment;
          const pnl = parseFloat(np.payment_pct);
          return (
            <div className="flex items-center gap-1.5 text-xs">
              <Countdown targetTime={np.at} />
              <span
                className={cn(
                  "font-mono tabular-nums",
                  pnl >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
                )}
              >
                {formatRate(np.payment_pct)}
              </span>
            </div>
          );
        },
      },
    ],
    []
  );

  const table = useReactTable({
    data: arbitrages ?? [],
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">Failed to load arbitrage data: {error.message}</p>
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
          <Badge variant="outline" className="font-mono">
            {arbitrages.length} pairs
          </Badge>
        )}
      </div>

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 15 }).map((_, i) => (
                <TableRow key={i}>
                  {columns.map((_, j) => (
                    <TableCell key={j}>
                      <Skeleton className="h-4 w-full" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center text-muted-foreground">
                  No arbitrage opportunities found. Waiting for data from collector and worker...
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => {
                const isExpanded = expandedRows.has(row.original.symbol);
                return (
                  <TableRow
                    key={row.id}
                    className="group cursor-pointer"
                    onClick={() => toggleRow(row.original.symbol)}
                  >
                    <TableCell colSpan={columns.length} className="p-0">
                      <div className="flex items-center px-2 py-2">
                        {row.getVisibleCells().map((cell) => (
                          <div
                            key={cell.id}
                            className="px-2"
                            style={{ width: cell.column.getSize() }}
                          >
                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                          </div>
                        ))}
                      </div>
                      {isExpanded && <ExpandedRow arb={row.original} />}
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
