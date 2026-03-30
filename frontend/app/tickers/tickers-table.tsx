"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { useState, useMemo } from "react";
import { fetchAllTickers, fetchExchanges, flushAllData, type TickerRow, type Ticker } from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { ArrowUpDown, ArrowUp, ArrowDown, Search, Trash2 } from "lucide-react";

function formatPrice(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
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

function formatChange(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(2)}%`;
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

function TickerSubRow({ ticker }: { ticker: Ticker }) {
  const change = parseFloat(ticker.change_24h);
  const isPositive = change >= 0;

  return (
    <div className="flex items-center gap-4 py-0.5 text-xs">
      <div className="w-14 shrink-0">
        <ExchangeBadge exchangeId={ticker.exchange_id} />
      </div>
      <div className="w-24 text-right font-mono tabular-nums">
        {formatPrice(ticker.price)}
      </div>
      <div className="w-24 text-right font-mono tabular-nums text-muted-foreground">
        {formatPrice(ticker.bid)}
      </div>
      <div className="w-24 text-right font-mono tabular-nums text-muted-foreground">
        {formatPrice(ticker.ask)}
      </div>
      <div className="w-24 text-right font-mono tabular-nums text-muted-foreground">
        {formatPrice(ticker.high_24h)}
      </div>
      <div className="w-24 text-right font-mono tabular-nums text-muted-foreground">
        {formatPrice(ticker.low_24h)}
      </div>
      <div className="w-24 text-right font-mono tabular-nums">
        {formatVolume(ticker.volume_24h)}
      </div>
      <div
        className={cn(
          "w-20 text-right font-mono tabular-nums font-medium",
          isPositive ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
        )}
      >
        {formatChange(ticker.change_24h)}
      </div>
    </div>
  );
}

function SortIcon({ column }: { column: { getIsSorted: () => false | "asc" | "desc" } }) {
  const sorted = column.getIsSorted();
  if (sorted === "asc") return <ArrowUp className="ml-1 h-3 w-3" />;
  if (sorted === "desc") return <ArrowDown className="ml-1 h-3 w-3" />;
  return <ArrowUpDown className="ml-1 h-3 w-3 opacity-30" />;
}

export function TickersTable() {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [globalFilter, setGlobalFilter] = useState("");

  const queryClient = useQueryClient();

  const { data: tickerRows, isLoading, error } = useQuery({
    queryKey: ["tickers"],
    queryFn: fetchAllTickers,
  });

  const { data: exchanges } = useQuery({
    queryKey: ["exchanges"],
    queryFn: fetchExchanges,
  });

  const flushMutation = useMutation({
    mutationFn: flushAllData,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tickers"] });
      queryClient.invalidateQueries({ queryKey: ["exchanges"] });
    },
  });

  const columns = useMemo<ColumnDef<TickerRow>[]>(
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
        id: "exchanges",
        header: "Exchanges",
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.tickers.length}
          </span>
        ),
      },
      {
        id: "price",
        header: ({ column }) => (
          <button
            className="flex items-center font-medium"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            Price
            <SortIcon column={column} />
          </button>
        ),
        accessorFn: (row) => {
          const first = row.tickers[0];
          return first ? parseFloat(first.price) : 0;
        },
        cell: ({ row }) => {
          const first = row.original.tickers[0];
          return first ? (
            <span className="font-mono tabular-nums">{formatPrice(first.price)}</span>
          ) : (
            "-"
          );
        },
      },
      {
        id: "change_24h",
        header: ({ column }) => (
          <button
            className="flex items-center font-medium"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            24h Change
            <SortIcon column={column} />
          </button>
        ),
        accessorFn: (row) => {
          const first = row.tickers[0];
          return first ? parseFloat(first.change_24h) : 0;
        },
        cell: ({ row }) => {
          const first = row.original.tickers[0];
          if (!first) return "-";
          const change = parseFloat(first.change_24h);
          const isPositive = change >= 0;
          return (
            <span
              className={cn(
                "font-mono tabular-nums font-medium",
                isPositive
                  ? "text-emerald-600 dark:text-emerald-400"
                  : "text-red-600 dark:text-red-400"
              )}
            >
              {formatChange(first.change_24h)}
            </span>
          );
        },
      },
      {
        id: "volume_24h",
        header: ({ column }) => (
          <button
            className="flex items-center font-medium"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          >
            24h Volume
            <SortIcon column={column} />
          </button>
        ),
        accessorFn: (row) => {
          const first = row.tickers[0];
          return first ? parseFloat(first.volume_24h) : 0;
        },
        cell: ({ row }) => {
          const first = row.original.tickers[0];
          return first ? (
            <span className="font-mono tabular-nums">{formatVolume(first.volume_24h)}</span>
          ) : (
            "-"
          );
        },
      },
    ],
    []
  );

  const table = useReactTable({
    data: tickerRows ?? [],
    columns,
    state: { sorting, globalFilter },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    globalFilterFn: (row, _columnId, filterValue) => {
      return row.original.symbol
        .toLowerCase()
        .includes((filterValue as string).toLowerCase());
    },
  });

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">Failed to load tickers: {error.message}</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <div className="relative max-w-sm flex-1">
          <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search symbols..."
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="h-8 w-full rounded-md border bg-background pl-8 pr-3 text-xs outline-none ring-ring focus-visible:ring-2"
          />
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {exchanges && (
            <div className="flex items-center gap-1.5">
              {exchanges.map((ex) => (
                <ExchangeBadge key={ex} exchangeId={ex} />
              ))}
            </div>
          )}
          {tickerRows && (
            <Badge variant="outline" className="font-mono">
              {tickerRows.length} symbols
            </Badge>
          )}
          <Button
            variant="outline"
            size="sm"
            className="h-7 gap-1.5 text-xs text-destructive hover:bg-destructive/10 hover:text-destructive"
            disabled={flushMutation.isPending}
            onClick={() => {
              if (window.confirm("Flush all stored data (markets, tickers, funding rates) from Redis?")) {
                flushMutation.mutate();
              }
            }}
          >
            <Trash2 className="h-3 w-3" />
            {flushMutation.isPending ? "Flushing..." : "Flush data"}
          </Button>
        </div>
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
              Array.from({ length: 20 }).map((_, i) => (
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
                  No tickers found.
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} className="group">
                  <TableCell colSpan={columns.length} className="p-0">
                    {/* Summary row */}
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
                    {/* Exchange sub-rows */}
                    <div className="border-t bg-muted/30 px-4 py-1.5">
                      <div className="flex items-center gap-4 pb-1 text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                        <div className="w-14 shrink-0">Exchange</div>
                        <div className="w-24 text-right">Price</div>
                        <div className="w-24 text-right">Bid</div>
                        <div className="w-24 text-right">Ask</div>
                        <div className="w-24 text-right">High 24h</div>
                        <div className="w-24 text-right">Low 24h</div>
                        <div className="w-24 text-right">Volume 24h</div>
                        <div className="w-20 text-right">Change 24h</div>
                      </div>
                      {row.original.tickers.map((ticker) => (
                        <TickerSubRow
                          key={ticker.exchange_id}
                          ticker={ticker}
                        />
                      ))}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
