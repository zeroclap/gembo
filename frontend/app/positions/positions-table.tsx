"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useMemo, useCallback, Fragment } from "react";
import {
  fetchPositions,
  closePosition,
  deletePosition,
  fetchPositionOrders,
  fetchPositionFunding,
  type Position,
  type PositionStatus,
  type ExchangeOrder,
  type FundingPayment,
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
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
  X,
  Trash2,
  Loader2,
  ChevronDown,
  ChevronUp,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

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

function formatUsd(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "$0.00";
  const abs = Math.abs(num);
  const sign = num >= 0 ? "+" : "-";
  if (abs >= 1000) {
    return `${sign}$${abs.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }
  return `${sign}$${abs.toFixed(4)}`;
}

function formatQty(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "-";
  if (num >= 1) return num.toFixed(4);
  return num.toFixed(8);
}

function formatRate(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  const pct = num * 100;
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
}

function formatTimeShort(iso: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "-";
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
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

const statusConfig: Record<
  PositionStatus,
  { label: string; className: string }
> = {
  pending: {
    label: "Pending",
    className:
      "bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/20",
  },
  opening: {
    label: "Opening",
    className:
      "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20",
  },
  open: {
    label: "Open",
    className:
      "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  },
  partially_open: {
    label: "Partial",
    className:
      "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
  },
  closing: {
    label: "Closing",
    className:
      "bg-orange-500/10 text-orange-600 dark:text-orange-400 border-orange-500/20",
  },
  closed: {
    label: "Closed",
    className:
      "bg-muted text-muted-foreground border-border",
  },
  failed: {
    label: "Failed",
    className:
      "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20",
  },
};

function StatusBadge({ status }: { status: PositionStatus }) {
  const cfg = statusConfig[status] ?? statusConfig.pending;
  return (
    <Badge variant="outline" className={cn("text-[0.625rem]", cfg.className)}>
      {cfg.label}
    </Badge>
  );
}

function PnlValue({ value, className }: { value: string; className?: string }) {
  const num = parseFloat(value);
  const isPositive = num > 0;
  const isNegative = num < 0;
  return (
    <span
      className={cn(
        "font-mono tabular-nums",
        isPositive && "text-emerald-600 dark:text-emerald-400",
        isNegative && "text-red-600 dark:text-red-400",
        !isPositive && !isNegative && "text-muted-foreground",
        className
      )}
    >
      {formatUsd(value)}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Status filter tabs
// ---------------------------------------------------------------------------

const statusFilters: { label: string; value: string }[] = [
  { label: "All", value: "" },
  { label: "Active", value: "pending,opening,open,partially_open,closing" },
  { label: "Open", value: "open,partially_open" },
  { label: "Closed", value: "closed" },
  { label: "Failed", value: "failed" },
];

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

type SortKey = "symbol" | "status" | "pnl" | "funding" | "created";

interface SortState {
  key: SortKey;
  desc: boolean;
}

function getSortValue(p: Position, key: SortKey): number | string {
  switch (key) {
    case "symbol":
      return p.symbol;
    case "status":
      return p.status;
    case "pnl": {
      const unrealized =
        parseFloat(p.long_unrealized_pnl) +
        parseFloat(p.short_unrealized_pnl);
      return isNaN(unrealized) ? 0 : unrealized;
    }
    case "funding":
      return parseFloat(p.total_funding_pnl) || 0;
    case "created":
      return new Date(p.created_at).getTime() || 0;
  }
}

function sortPositions(data: Position[], sort: SortState): Position[] {
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
// Expanded detail (orders + funding payments)
// ---------------------------------------------------------------------------

function ExpandedDetail({ positionId }: { positionId: string }) {
  const { data: orders, isLoading: ordersLoading } = useQuery({
    queryKey: ["position-orders", positionId],
    queryFn: () => fetchPositionOrders(positionId),
  });

  const { data: funding, isLoading: fundingLoading } = useQuery({
    queryKey: ["position-funding", positionId],
    queryFn: () => fetchPositionFunding(positionId),
  });

  return (
    <div className="border-t bg-muted/50 dark:bg-muted/40 px-4 py-3">
      <div className="flex gap-6">
        {/* Orders */}
        <div className="min-w-0 flex-1">
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-1.5">
            Orders
          </div>
          {ordersLoading ? (
            <div className="space-y-1">
              {Array.from({ length: 2 }).map((_, i) => (
                <Skeleton key={i} className="h-4 w-full" />
              ))}
            </div>
          ) : !orders || orders.length === 0 ? (
            <div className="text-xs text-muted-foreground">
              No orders yet.
            </div>
          ) : (
            <table className="text-xs w-full">
              <thead>
                <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                  <th className="pr-3 pb-1 text-left font-medium">Exchange</th>
                  <th className="pr-3 pb-1 text-left font-medium">Leg</th>
                  <th className="pr-3 pb-1 text-left font-medium">Action</th>
                  <th className="pr-3 pb-1 text-left font-medium">Side</th>
                  <th className="pr-3 pb-1 text-right font-medium">Qty</th>
                  <th className="pr-3 pb-1 text-right font-medium">
                    Avg Price
                  </th>
                  <th className="pr-3 pb-1 text-right font-medium">Fee</th>
                  <th className="pr-3 pb-1 text-left font-medium">Status</th>
                  <th className="pb-1 text-left font-medium">Time</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((o: ExchangeOrder) => (
                  <tr key={o.id} className="border-t border-border/30">
                    <td className="pr-3 py-0.5">
                      <ExchangeBadge exchangeId={o.exchange_id} />
                    </td>
                    <td className="pr-3 py-0.5">
                      <span
                        className={cn(
                          "text-[0.625rem] font-medium uppercase",
                          o.leg === "long"
                            ? "text-emerald-600 dark:text-emerald-400"
                            : "text-red-600 dark:text-red-400"
                        )}
                      >
                        {o.leg}
                      </span>
                    </td>
                    <td className="pr-3 py-0.5 text-muted-foreground">
                      {o.action}
                    </td>
                    <td className="pr-3 py-0.5 text-muted-foreground">
                      {o.side}
                    </td>
                    <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                      {formatQty(o.filled_qty)}
                    </td>
                    <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                      {formatPrice(o.average_price)}
                    </td>
                    <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-muted-foreground">
                      {formatUsd(o.fee_cost)}
                    </td>
                    <td className="pr-3 py-0.5">
                      <span
                        className={cn(
                          "text-[0.625rem]",
                          o.status === "filled" &&
                            "text-emerald-600 dark:text-emerald-400",
                          o.status === "failed" &&
                            "text-red-600 dark:text-red-400",
                          o.status === "cancelled" &&
                            "text-amber-600 dark:text-amber-400"
                        )}
                      >
                        {o.status}
                      </span>
                    </td>
                    <td className="py-0.5 text-muted-foreground">
                      {formatTimeShort(o.created_at)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Funding Payments */}
        <div className="min-w-0 shrink-0 border-l border-border/40 pl-6">
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-1.5">
            Funding Payments
          </div>
          {fundingLoading ? (
            <div className="space-y-1">
              {Array.from({ length: 2 }).map((_, i) => (
                <Skeleton key={i} className="h-4 w-40" />
              ))}
            </div>
          ) : !funding || funding.length === 0 ? (
            <div className="text-xs text-muted-foreground">
              No funding payments yet.
            </div>
          ) : (
            <table className="text-xs">
              <thead>
                <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                  <th className="pr-3 pb-1 text-left font-medium">Exchange</th>
                  <th className="pr-3 pb-1 text-left font-medium">Leg</th>
                  <th className="pr-3 pb-1 text-right font-medium">Rate</th>
                  <th className="pr-3 pb-1 text-right font-medium">Payment</th>
                  <th className="pb-1 text-left font-medium">Time</th>
                </tr>
              </thead>
              <tbody>
                {funding.map((f: FundingPayment) => {
                  const payment = parseFloat(f.payment);
                  return (
                    <tr key={f.id} className="border-t border-border/30">
                      <td className="pr-3 py-0.5">
                        <ExchangeBadge exchangeId={f.exchange_id} />
                      </td>
                      <td className="pr-3 py-0.5">
                        <span
                          className={cn(
                            "text-[0.625rem] font-medium uppercase",
                            f.leg === "long"
                              ? "text-emerald-600 dark:text-emerald-400"
                              : "text-red-600 dark:text-red-400"
                          )}
                        >
                          {f.leg}
                        </span>
                      </td>
                      <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                        {formatRate(f.rate)}
                      </td>
                      <td
                        className={cn(
                          "pr-3 py-0.5 text-right font-mono tabular-nums",
                          payment >= 0
                            ? "text-emerald-600 dark:text-emerald-400"
                            : "text-red-600 dark:text-red-400"
                        )}
                      >
                        {formatUsd(f.payment)}
                      </td>
                      <td className="py-0.5 text-muted-foreground">
                        {formatTimeShort(f.funded_at)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Position row
// ---------------------------------------------------------------------------

function PositionRow({
  position,
  isExpanded,
  onToggle,
}: {
  position: Position;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const queryClient = useQueryClient();
  const closeMutation = useMutation({
    mutationFn: closePosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deletePosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
    },
  });

  const unrealizedPnl =
    parseFloat(position.long_unrealized_pnl) +
    parseFloat(position.short_unrealized_pnl);
  const unrealizedStr = isNaN(unrealizedPnl) ? "0" : String(unrealizedPnl);
  const canClose =
    position.status === "open" || position.status === "partially_open";
  const canDelete = position.status === "pending";

  return (
    <tbody
      className={cn(
        "group/row cursor-pointer border-b transition-colors hover:bg-muted/50",
        isExpanded && "bg-muted/20"
      )}
      onClick={onToggle}
    >
      {/* Main row: Long leg */}
      <tr>
        {/* Symbol + Status — spans both rows */}
        <td className="p-2 align-middle whitespace-nowrap" rowSpan={2}>
          <div className="flex flex-col gap-1">
            <span className="font-medium">{position.symbol}</span>
            <StatusBadge status={position.status} />
          </div>
        </td>

        {/* Long leg */}
        <td className="p-2 align-middle whitespace-nowrap">
          <div className="flex items-center gap-1.5">
            <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
              <ArrowUp className="h-3 w-3 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={position.long_exchange_id} />
            <span className="flex-1" />
            <span className="font-mono tabular-nums text-muted-foreground text-xs">
              {formatPrice(position.long_entry_price)}
            </span>
          </div>
        </td>

        {/* Size — spans both rows */}
        <td
          className="p-2 align-middle whitespace-nowrap text-right"
          rowSpan={2}
        >
          <div className="flex flex-col items-end gap-0.5">
            <span className="font-mono tabular-nums text-xs">
              {position.size_type === "usd" ? "$" : ""}
              {formatQty(position.requested_size)}
              {position.size_type === "base" ? " base" : ""}
            </span>
            <span className="text-[0.625rem] text-muted-foreground uppercase">
              {position.size_type}
            </span>
          </div>
        </td>

        {/* Unrealized P&L — spans both rows */}
        <td
          className="p-2 align-middle whitespace-nowrap text-right"
          rowSpan={2}
        >
          <PnlValue value={unrealizedStr} className="text-xs" />
        </td>

        {/* Funding P&L — spans both rows */}
        <td
          className="p-2 align-middle whitespace-nowrap text-right"
          rowSpan={2}
        >
          <PnlValue value={position.total_funding_pnl} className="text-xs" />
        </td>

        {/* Created — spans both rows */}
        <td
          className="p-2 align-middle whitespace-nowrap text-right"
          rowSpan={2}
        >
          <div className="flex flex-col items-end gap-0.5">
            <span className="text-xs text-muted-foreground">
              {formatTimeShort(position.created_at)}
            </span>
            {position.opened_at && (
              <span className="text-[0.625rem] text-muted-foreground">
                opened {formatTimeShort(position.opened_at)}
              </span>
            )}
          </div>
        </td>

        {/* Actions — spans both rows */}
        <td className="p-2 align-middle whitespace-nowrap" rowSpan={2}>
          <div className="flex items-center gap-1">
            {canClose && (
              <Button
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-xs text-red-600 dark:text-red-400 hover:text-red-700 hover:bg-red-500/10"
                disabled={closeMutation.isPending}
                onClick={(e) => {
                  e.stopPropagation();
                  closeMutation.mutate(position.id);
                }}
              >
                {closeMutation.isPending ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <X className="h-3 w-3" />
                )}
                <span className="ml-1">Close</span>
              </Button>
            )}
            {canDelete && (
              <Button
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-xs text-muted-foreground hover:text-red-600 hover:bg-red-500/10"
                disabled={deleteMutation.isPending}
                onClick={(e) => {
                  e.stopPropagation();
                  deleteMutation.mutate(position.id);
                }}
              >
                {deleteMutation.isPending ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Trash2 className="h-3 w-3" />
                )}
                <span className="ml-1">Delete</span>
              </Button>
            )}
            <button
              className="text-muted-foreground hover:text-foreground transition-colors"
              onClick={(e) => {
                e.stopPropagation();
                onToggle();
              }}
            >
              {isExpanded ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </button>
          </div>
        </td>
      </tr>

      {/* Second row: Short leg */}
      <tr>
        <td className="p-2 align-middle whitespace-nowrap">
          <div className="flex items-center gap-1.5">
            <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
              <ArrowDown className="h-3 w-3 text-white" strokeWidth={2.5} />
            </span>
            <ExchangeBadge exchangeId={position.short_exchange_id} />
            <span className="flex-1" />
            <span className="font-mono tabular-nums text-muted-foreground text-xs">
              {formatPrice(position.short_entry_price)}
            </span>
          </div>
        </td>
      </tr>

      {/* Expanded detail row */}
      {isExpanded && (
        <tr>
          <td colSpan={7} className="p-0">
            <ExpandedDetail positionId={position.id} />
          </td>
        </tr>
      )}
    </tbody>
  );
}

// ---------------------------------------------------------------------------
// Main table
// ---------------------------------------------------------------------------

export function PositionsTable() {
  const [statusFilter, setStatusFilter] = useState("");
  const [sort, setSort] = useState<SortState>({ key: "created", desc: true });
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  const {
    data: positions,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["positions", statusFilter],
    queryFn: () =>
      fetchPositions(statusFilter ? { status: statusFilter } : undefined),
  });

  const sortedData = useMemo(
    () => (positions ? sortPositions(positions, sort) : []),
    [positions, sort]
  );

  const toggleSort = useCallback((key: SortKey) => {
    setSort((prev) => ({
      key,
      desc: prev.key === key ? !prev.desc : true,
    }));
  }, []);

  const toggleRow = useCallback((id: string) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">
          Failed to load positions: {error.message}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <div className="text-xs text-muted-foreground">
          Funding rate arbitrage positions
        </div>
        {positions && (
          <Badge variant="outline" className="font-mono">
            {positions.length} position{positions.length !== 1 ? "s" : ""}
          </Badge>
        )}
      </div>

      {/* Status filter tabs */}
      <div className="flex items-center gap-1">
        {statusFilters.map((f) => (
          <Button
            key={f.value}
            variant="ghost"
            size="sm"
            className={cn(
              "h-7 px-3 text-xs",
              statusFilter === f.value
                ? "bg-muted text-foreground"
                : "text-muted-foreground hover:text-foreground"
            )}
            onClick={() => setStatusFilter(f.value)}
          >
            {f.label}
          </Button>
        ))}
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
              <TableHead className="text-right">Size</TableHead>
              <TableHead className="text-right">
                <SortButton
                  label="Unrealized"
                  sortKey="pnl"
                  current={sort}
                  onToggle={toggleSort}
                />
              </TableHead>
              <TableHead className="text-right">
                <SortButton
                  label="Funding"
                  sortKey="funding"
                  current={sort}
                  onToggle={toggleSort}
                />
              </TableHead>
              <TableHead className="text-right">
                <SortButton
                  label="Created"
                  sortKey="created"
                  current={sort}
                  onToggle={toggleSort}
                />
              </TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>

          {isLoading ? (
            <TableBody>
              {Array.from({ length: 5 }).map((_, i) => (
                <Fragment key={i}>
                  <TableRow className="border-0">
                    <TableCell rowSpan={2}>
                      <Skeleton className="h-4 w-20" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-4 w-32" />
                    </TableCell>
                    <TableCell rowSpan={2}>
                      <Skeleton className="h-4 w-16" />
                    </TableCell>
                    <TableCell rowSpan={2}>
                      <Skeleton className="h-4 w-16" />
                    </TableCell>
                    <TableCell rowSpan={2}>
                      <Skeleton className="h-4 w-16" />
                    </TableCell>
                    <TableCell rowSpan={2}>
                      <Skeleton className="h-4 w-24" />
                    </TableCell>
                    <TableCell rowSpan={2}>
                      <Skeleton className="h-4 w-12" />
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>
                      <Skeleton className="h-4 w-32" />
                    </TableCell>
                  </TableRow>
                </Fragment>
              ))}
            </TableBody>
          ) : sortedData.length === 0 ? (
            <TableBody>
              <TableRow>
                <TableCell
                  colSpan={7}
                  className="h-24 text-center text-muted-foreground"
                >
                  {statusFilter
                    ? "No positions match the current filter."
                    : "No positions found. Open a position from the Arbitrage page."}
                </TableCell>
              </TableRow>
            </TableBody>
          ) : (
            sortedData.map((p) => (
              <PositionRow
                key={p.id}
                position={p}
                isExpanded={expandedRows.has(p.id)}
                onToggle={() => toggleRow(p.id)}
              />
            ))
          )}
        </Table>
      </div>
    </div>
  );
}
