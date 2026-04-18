"use client";

import { useState, useMemo } from "react";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  ExchangeBadge,
  StatusBadge,
  PnlValue,
} from "@/components/arbitrage/shared";
import {
  formatPrice,
  formatQty,
  formatUsdPlain,
  formatUsd,
  formatRate,
  formatTimeShort,
  formatDuration,
} from "@/lib/format";
import { usePositionOrders, usePositionFunding } from "@/hooks/use-arbitrage-data";
import type { Position } from "@/lib/api";
import {
  ChevronDown,
  ChevronUp,
  FileText,
  Coins,
  History,
} from "lucide-react";

// ---------------------------------------------------------------------------
// PositionHistory — closed positions table
// ---------------------------------------------------------------------------

interface PositionHistoryProps {
  positions: Position[];
  isLoading: boolean;
}

export function PositionHistory({ positions, isLoading }: PositionHistoryProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null);

  if (isLoading) {
    return (
      <div className="p-4 space-y-3">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }

  if (positions.length === 0) {
    return (
      <div className="p-6 text-center">
        <History className="h-5 w-5 text-muted-foreground mx-auto mb-2" />
        <div className="text-sm text-muted-foreground">
          No closed positions yet
        </div>
      </div>
    );
  }

  return (
    <div className="divide-y divide-border/50">
      {positions.map((pos) => (
        <PositionRow
          key={pos.id}
          position={pos}
          isExpanded={expandedId === pos.id}
          onToggle={() =>
            setExpandedId((prev) => (prev === pos.id ? null : pos.id))
          }
        />
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// PositionRow — single position with expandable detail
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
  const unrealized =
    (parseFloat(position.long_unrealized_pnl) || 0) +
    (parseFloat(position.short_unrealized_pnl) || 0);
  const fundingPnl = parseFloat(position.total_funding_pnl) || 0;
  const totalPnl = unrealized + fundingPnl;

  return (
    <div>
      {/* Summary row */}
      <button
        type="button"
        onClick={onToggle}
        className={cn(
          "w-full flex items-center gap-3 px-4 py-2.5 text-xs transition-colors hover:bg-muted/30",
          isExpanded && "bg-muted/20"
        )}
      >
        {/* Date */}
        <span className="text-muted-foreground font-mono tabular-nums shrink-0 w-[7rem]">
          {formatTimeShort(position.created_at)}
        </span>

        {/* Status + Type */}
        <div className="flex items-center gap-1.5 shrink-0">
          <StatusBadge status={position.status} />
          {position.arbitrage_type && (
            <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider px-1 py-0.5 rounded bg-muted">
              {position.arbitrage_type}
            </span>
          )}
        </div>

        {/* Exchanges */}
        <div className="flex items-center gap-1 shrink-0">
          <ExchangeBadge exchangeId={position.long_exchange_id} />
          <span className="text-muted-foreground">/</span>
          <ExchangeBadge exchangeId={position.short_exchange_id} />
        </div>

        {/* Size */}
        <span className="font-mono tabular-nums text-muted-foreground ml-auto">
          {position.size_type === "price" ? "$" : ""}
          {formatQty(position.requested_size)}
        </span>

        {/* Duration */}
        <span className="font-mono tabular-nums text-muted-foreground w-[4.5rem] text-right">
          {formatDuration(
            position.opened_at || position.created_at,
            position.closed_at
          )}
        </span>

        {/* P&L */}
        <PnlValue value={String(totalPnl)} className="w-[5rem] text-right" />

        {/* Chevron */}
        {isExpanded ? (
          <ChevronUp className="h-3 w-3 text-muted-foreground shrink-0" />
        ) : (
          <ChevronDown className="h-3 w-3 text-muted-foreground shrink-0" />
        )}
      </button>

      {/* Expanded detail */}
      {isExpanded && <ExpandedDetail position={position} />}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ExpandedDetail — orders and funding panels
// ---------------------------------------------------------------------------

function ExpandedDetail({ position }: { position: Position }) {
  const [tab, setTab] = useState<"overview" | "orders" | "funding">("overview");

  const unrealized =
    (parseFloat(position.long_unrealized_pnl) || 0) +
    (parseFloat(position.short_unrealized_pnl) || 0);
  const fundingPnl = parseFloat(position.total_funding_pnl) || 0;
  const totalPnl = unrealized + fundingPnl;

  return (
    <div className="border-t bg-muted/10 px-4 py-3">
      {/* Tabs */}
      <div className="flex items-center gap-1 mb-3">
        {(["overview", "orders", "funding"] as const).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setTab(t)}
            className={cn(
              "px-2.5 py-1 text-[0.625rem] font-medium rounded-md transition-colors capitalize",
              tab === t
                ? "bg-background text-foreground border"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            {t === "overview" && <FileText className="inline h-3 w-3 mr-1 align-text-top" />}
            {t === "orders" && <FileText className="inline h-3 w-3 mr-1 align-text-top" />}
            {t === "funding" && <Coins className="inline h-3 w-3 mr-1 align-text-top" />}
            {t}
          </button>
        ))}
      </div>

      {tab === "overview" && (
        <div className="space-y-3">
          {/* P&L summary */}
          <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-0.5 text-xs">
            <span className="text-muted-foreground">Unrealized</span>
            <PnlValue value={String(unrealized)} className="text-right text-xs" />
            <span className="text-muted-foreground pl-2">Long</span>
            <PnlValue value={position.long_unrealized_pnl} className="text-right text-xs" />
            <span className="text-muted-foreground pl-2">Short</span>
            <PnlValue value={position.short_unrealized_pnl} className="text-right text-xs" />
            <span className="text-muted-foreground">Funding</span>
            <PnlValue value={position.total_funding_pnl} className="text-right text-xs" />
            <span className="text-muted-foreground font-medium">Total</span>
            <PnlValue value={String(totalPnl)} className="text-right text-xs font-medium" />
          </div>

          {/* Entry/Exit details */}
          <table className="text-xs w-full">
            <thead>
              <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                <th className="pr-3 pb-1 text-left font-medium" />
                <th className="pr-3 pb-1 text-right font-medium">Long</th>
                <th className="pb-1 text-right font-medium">Short</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-t border-border/30">
                <td className="pr-3 py-0.5 text-muted-foreground">Entry price</td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(position.long_entry_price)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(position.short_entry_price)}
                </td>
              </tr>
              {(position.long_exit_price !== "0" ||
                position.short_exit_price !== "0") && (
                <tr className="border-t border-border/30">
                  <td className="pr-3 py-0.5 text-muted-foreground">Exit price</td>
                  <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                    {formatPrice(position.long_exit_price)}
                  </td>
                  <td className="py-0.5 text-right font-mono tabular-nums">
                    {formatPrice(position.short_exit_price)}
                  </td>
                </tr>
              )}
              <tr className="border-t border-border/30">
                <td className="pr-3 py-0.5 text-muted-foreground">Filled qty</td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                  {formatQty(position.long_filled_qty)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums">
                  {formatQty(position.short_filled_qty)}
                </td>
              </tr>
              <tr className="border-t border-border/30">
                <td className="pr-3 py-0.5 text-muted-foreground">Base qty</td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                  {formatQty(position.long_filled_base_qty)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums">
                  {formatQty(position.short_filled_base_qty)}
                </td>
              </tr>
              <tr className="border-t border-border/30">
                <td className="pr-3 py-0.5 text-muted-foreground">Contract size</td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-muted-foreground">
                  {formatQty(position.long_contract_size)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums text-muted-foreground">
                  {formatQty(position.short_contract_size)}
                </td>
              </tr>
              <tr className="border-t border-border/30">
                <td className="pr-3 py-0.5 text-muted-foreground">Entry fees</td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-red-600 dark:text-red-400">
                  {formatUsdPlain(position.long_entry_fees)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums text-red-600 dark:text-red-400">
                  {formatUsdPlain(position.short_entry_fees)}
                </td>
              </tr>
              {(parseFloat(position.long_close_fees) > 0 ||
                parseFloat(position.short_close_fees) > 0) && (
                <tr className="border-t border-border/30">
                  <td className="pr-3 py-0.5 text-muted-foreground">Close fees</td>
                  <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-red-600 dark:text-red-400">
                    {formatUsdPlain(position.long_close_fees)}
                  </td>
                  <td className="py-0.5 text-right font-mono tabular-nums text-red-600 dark:text-red-400">
                    {formatUsdPlain(position.short_close_fees)}
                  </td>
                </tr>
              )}
            </tbody>
          </table>

          {/* Close reason */}
          {position.close_reason && (
            <div className="flex items-center gap-2 text-xs">
              <span className="text-muted-foreground">Close reason</span>
              <Badge variant="outline" className="text-[0.625rem]">
                {position.close_reason.replace(/_/g, " ")}
              </Badge>
            </div>
          )}

          {/* Timing */}
          <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-0.5 text-xs">
            <span className="text-muted-foreground">Created</span>
            <span className="font-mono tabular-nums text-right">
              {formatTimeShort(position.created_at)}
            </span>
            {position.opened_at && (
              <>
                <span className="text-muted-foreground">Opened</span>
                <span className="font-mono tabular-nums text-right">
                  {formatTimeShort(position.opened_at)}
                </span>
              </>
            )}
            {position.closed_at && (
              <>
                <span className="text-muted-foreground">Closed</span>
                <span className="font-mono tabular-nums text-right">
                  {formatTimeShort(position.closed_at)}
                </span>
              </>
            )}
            <span className="text-muted-foreground">Duration</span>
            <span className="font-mono tabular-nums text-right">
              {formatDuration(
                position.opened_at || position.created_at,
                position.closed_at
              )}
            </span>
          </div>

          {/* Error */}
          {position.error_message && (
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive">
              {position.error_message}
            </div>
          )}
        </div>
      )}

      {tab === "orders" && <OrdersPanel positionId={position.id} />}
      {tab === "funding" && <FundingPanel positionId={position.id} />}
    </div>
  );
}

// ---------------------------------------------------------------------------
// OrdersPanel
// ---------------------------------------------------------------------------

function OrdersPanel({ positionId }: { positionId: string }) {
  const { data: orders, isLoading } = usePositionOrders(positionId);

  if (isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    );
  }

  if (!orders || orders.length === 0) {
    return (
      <p className="text-xs text-muted-foreground py-2">No orders yet.</p>
    );
  }

  return (
    <div>
      <p className="text-[0.625rem] text-muted-foreground mb-2">
        {orders.length} order{orders.length !== 1 ? "s" : ""}
      </p>
      <div className="overflow-x-auto">
        <table className="text-xs w-full">
          <thead>
            <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
              <th className="pr-2 pb-1 text-left font-medium">Exchange</th>
              <th className="pr-2 pb-1 text-left font-medium">Leg</th>
              <th className="pr-2 pb-1 text-left font-medium">Action</th>
              <th className="pr-2 pb-1 text-right font-medium">Qty</th>
              <th className="pr-2 pb-1 text-right font-medium">Base Qty</th>
              <th className="pr-2 pb-1 text-right font-medium">Contract</th>
              <th className="pr-2 pb-1 text-right font-medium">Avg Price</th>
              <th className="pr-2 pb-1 text-right font-medium">Cost</th>
              <th className="pr-2 pb-1 text-right font-medium">Fee</th>
              <th className="pr-2 pb-1 text-left font-medium">Status</th>
              <th className="pb-1 text-left font-medium">Time</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((o) => (
              <tr key={o.id} className="border-t border-border/30">
                <td className="pr-2 py-0.5">
                  <ExchangeBadge exchangeId={o.exchange_id} />
                </td>
                <td className="pr-2 py-0.5">
                  <span
                    className={cn(
                      "uppercase font-medium text-[0.625rem]",
                      o.leg === "long"
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {o.leg}
                  </span>
                </td>
                <td className="pr-2 py-0.5 text-muted-foreground uppercase">
                  {o.action}
                </td>
                <td className="pr-2 py-0.5 text-right font-mono tabular-nums">
                  {formatQty(o.filled_qty)} / {formatQty(o.requested_qty)}
                </td>
                <td className="pr-2 py-0.5 text-right font-mono tabular-nums text-muted-foreground">
                  {formatQty(o.filled_base_qty)} / {formatQty(o.requested_base_qty)}
                </td>
                <td className="pr-2 py-0.5 text-right font-mono tabular-nums text-muted-foreground">
                  {formatQty(o.contract_size)}
                </td>
                <td className="pr-2 py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(o.average_price)}
                </td>
                <td className="pr-2 py-0.5 text-right font-mono tabular-nums">
                  {formatUsdPlain(o.cost)}
                </td>
                <td className="pr-2 py-0.5 text-right font-mono tabular-nums text-red-600 dark:text-red-400">
                  {parseFloat(o.fee_cost) > 0 ? formatUsdPlain(o.fee_cost) : "-"}
                </td>
                <td className="pr-2 py-0.5">
                  <span
                    className={cn(
                      "text-[0.625rem]",
                      o.status === "filled" && "text-emerald-600 dark:text-emerald-400",
                      o.status === "failed" && "text-red-600 dark:text-red-400",
                      o.status === "cancelled" && "text-amber-600 dark:text-amber-400"
                    )}
                  >
                    {o.status}
                  </span>
                </td>
                <td className="py-0.5 text-muted-foreground font-mono tabular-nums">
                  {formatTimeShort(o.created_at)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// FundingPanel
// ---------------------------------------------------------------------------

function FundingPanel({ positionId }: { positionId: string }) {
  const { data: payments, isLoading } = usePositionFunding(positionId);

  const rows = useMemo(() => {
    if (!payments) return [];
    let cumulative = 0;
    return payments.map((p) => {
      cumulative += parseFloat(p.payment) || 0;
      return { ...p, cumulative };
    });
  }, [payments]);

  const totalCumulative = rows.length > 0 ? rows[rows.length - 1].cumulative : 0;

  if (isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    );
  }

  if (!payments || payments.length === 0) {
    return (
      <p className="text-xs text-muted-foreground py-2">
        No funding payments yet.
      </p>
    );
  }

  return (
    <div>
      <p className="text-[0.625rem] text-muted-foreground mb-2">
        {payments.length} payment{payments.length !== 1 ? "s" : ""} — Net:{" "}
        <PnlValue value={String(totalCumulative)} className="text-[0.625rem]" />
      </p>
      <div className="overflow-x-auto">
        <table className="text-xs w-full">
          <thead>
            <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
              <th className="pr-2 pb-1 text-left font-medium">Exchange</th>
              <th className="pr-2 pb-1 text-left font-medium">Leg</th>
              <th className="pr-2 pb-1 text-right font-medium">Rate</th>
              <th className="pr-2 pb-1 text-right font-medium">Payment</th>
              <th className="pr-2 pb-1 text-right font-medium">Cum.</th>
              <th className="pb-1 text-left font-medium">Time</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((f) => (
              <tr key={f.id} className="border-t border-border/30">
                <td className="pr-2 py-0.5">
                  <ExchangeBadge exchangeId={f.exchange_id} />
                </td>
                <td className="pr-2 py-0.5">
                  <span
                    className={cn(
                      "uppercase font-medium text-[0.625rem]",
                      f.leg === "long"
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {f.leg}
                  </span>
                </td>
                <td className="pr-2 py-0.5 text-right font-mono tabular-nums">
                  {formatRate(f.rate)}
                </td>
                <td className="pr-2 py-0.5 text-right">
                  <span
                    className={cn(
                      "font-mono tabular-nums",
                      parseFloat(f.payment) >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {formatUsd(f.payment)}
                  </span>
                </td>
                <td className="pr-2 py-0.5 text-right">
                  <span
                    className={cn(
                      "font-mono tabular-nums font-medium",
                      f.cumulative >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {formatUsd(String(f.cumulative))}
                  </span>
                </td>
                <td className="py-0.5 text-muted-foreground font-mono tabular-nums">
                  {formatTimeShort(f.funded_at)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
