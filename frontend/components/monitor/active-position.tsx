"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { closePosition, type Position } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ExchangeBadge,
  StatusBadge,
  PnlValue,
  SectionHeader,
} from "@/components/monitor/shared";
import {
  formatPrice,
  formatQty,
  formatUsdPlain,
  formatDuration,
  formatTimeShort,
} from "@/lib/format";
import { cn } from "@/lib/utils";
import {
  X,
  Loader2,
  TrendingUp,
  TrendingDown,
  Clock,
  DollarSign,
  Activity,
  AlertTriangle,
} from "lucide-react";

interface ActivePositionProps {
  position: Position | null;
  isLoading: boolean;
}

export function ActivePosition({
  position,
  isLoading,
}: ActivePositionProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border bg-card/50 p-4 space-y-3">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-20 w-full" />
      </div>
    );
  }

  if (!position) {
    return (
      <div className="rounded-lg border border-dashed bg-card/30 px-4 py-6 flex items-center justify-center">
        <div className="text-xs text-muted-foreground">
          No active position for this pair
        </div>
      </div>
    );
  }

  return <PositionDetail position={position} />;
}

function PositionDetail({
  position,
}: {
  position: Position;
}) {
  const queryClient = useQueryClient();

  const closeMutation = useMutation({
    mutationFn: closePosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
    },
  });

  const canClose =
    position.status === "open" || position.status === "partially_open";

  // Computed metrics
  const longUnrealized = parseFloat(position.long_unrealized_pnl) || 0;
  const shortUnrealized = parseFloat(position.short_unrealized_pnl) || 0;
  const totalUnrealized = longUnrealized + shortUnrealized;
  const fundingPnl = parseFloat(position.total_funding_pnl) || 0;
  const totalPnl = totalUnrealized + fundingPnl;

  const longEntryFees = parseFloat(position.long_entry_fees) || 0;
  const shortEntryFees = parseFloat(position.short_entry_fees) || 0;
  const totalEntryFees = longEntryFees + shortEntryFees;

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-2.5">
        <div className="flex items-center gap-2">
          <SectionHeader>
            <Activity className="inline h-3 w-3 mr-0.5 align-text-top" />
            Active Position
          </SectionHeader>
          <StatusBadge status={position.status} />
          {position.arbitrage_type && (
            <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider px-1.5 py-0.5 rounded-md bg-muted">
              {position.arbitrage_type}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1.5">
          {canClose && (
            <Button
              variant="outline"
              size="sm"
              className="h-7 px-2 text-xs text-red-600 dark:text-red-400 border-red-500/30 hover:bg-red-500/10"
              disabled={closeMutation.isPending}
              onClick={() => closeMutation.mutate(position.id)}
            >
              {closeMutation.isPending ? (
                <Loader2 className="h-3 w-3 animate-spin mr-1" />
              ) : (
                <X className="h-3 w-3 mr-1" />
              )}
              Close
            </Button>
          )}
        </div>
      </div>

      <div className="p-4 space-y-4">
        {/* P&L Summary — top-level, prominent */}
        <div className="grid grid-cols-3 gap-3">
          <div className="text-center">
            <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-1">
              Unrealized
            </div>
            <PnlValue
              value={String(totalUnrealized)}
              className="text-sm font-medium"
            />
          </div>
          <div className="text-center">
            <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-1">
              Funding
            </div>
            <PnlValue
              value={position.total_funding_pnl}
              className="text-sm font-medium"
            />
          </div>
          <div className="text-center">
            <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-1">
              Total P&L
            </div>
            <PnlValue
              value={String(totalPnl)}
              className="text-base font-semibold"
            />
          </div>
        </div>

        <Separator />

        {/* Position details table */}
        <div>
          <SectionHeader className="mb-2">
            <DollarSign className="inline h-3 w-3 mr-0.5 align-text-top" />
            Position Details
          </SectionHeader>
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
              <tr className="border-t border-border/30">
                <td className="pr-4 py-0.5 text-muted-foreground">Exchange</td>
                <td className="pr-4 py-0.5 text-right">
                  <ExchangeBadge exchangeId={position.long_exchange_id} />
                </td>
                <td className="py-0.5 text-right">
                  <ExchangeBadge exchangeId={position.short_exchange_id} />
                </td>
              </tr>
              <tr className="border-t border-border/30">
                <td className="pr-4 py-0.5 text-muted-foreground">
                  Entry price
                </td>
                <td className="pr-4 py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(position.long_entry_price)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(position.short_entry_price)}
                </td>
              </tr>
              <tr className="border-t border-border/30">
                <td className="pr-4 py-0.5 text-muted-foreground">
                  Mark price
                </td>
                <td className="pr-4 py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(position.long_mark_price)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(position.short_mark_price)}
                </td>
              </tr>
              <tr className="border-t border-border/30">
                <td className="pr-4 py-0.5 text-muted-foreground">
                  Filled qty
                </td>
                <td className="pr-4 py-0.5 text-right font-mono tabular-nums">
                  {formatQty(position.long_filled_qty)}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums">
                  {formatQty(position.short_filled_qty)}
                </td>
              </tr>
              <tr className="border-t border-border/30">
                <td className="pr-4 py-0.5 text-muted-foreground">
                  Unrealized
                </td>
                <td className="pr-4 py-0.5 text-right">
                  <PnlValue
                    value={position.long_unrealized_pnl}
                    className="text-[0.625rem]"
                  />
                </td>
                <td className="py-0.5 text-right">
                  <PnlValue
                    value={position.short_unrealized_pnl}
                    className="text-[0.625rem]"
                  />
                </td>
              </tr>
              {(position.long_liquidation_price !== "0" ||
                position.short_liquidation_price !== "0") && (
                <tr className="border-t border-border/30">
                  <td className="pr-4 py-0.5 text-muted-foreground">
                    Liquidation
                  </td>
                  <td className="pr-4 py-0.5 text-right font-mono tabular-nums text-amber-600 dark:text-amber-400">
                    {formatPrice(position.long_liquidation_price)}
                  </td>
                  <td className="py-0.5 text-right font-mono tabular-nums text-amber-600 dark:text-amber-400">
                    {formatPrice(position.short_liquidation_price)}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Size & Capital */}
        <div>
          <SectionHeader className="mb-2">Size &amp; Capital</SectionHeader>
          <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
            <span className="text-muted-foreground">Requested size</span>
            <span className="font-mono tabular-nums text-right">
              {position.size_type === "usd" ? "$" : ""}
              {formatQty(position.requested_size)}
              <span className="ml-1 text-[0.625rem] text-muted-foreground uppercase">
                {position.size_type}
              </span>
            </span>
            <span className="text-muted-foreground">Entry fees</span>
            <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
              {formatUsdPlain(String(totalEntryFees))}
            </span>
            {position.leverage > 0 && (
              <>
                <span className="text-muted-foreground">Leverage</span>
                <span className="font-mono tabular-nums text-right">
                  {position.leverage}x
                </span>
              </>
            )}
          </div>
        </div>

        {/* Stepped execution progress */}
        {position.steps_total > 0 && (
          <div>
            <SectionHeader className="mb-2">Stepped Execution</SectionHeader>
            <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <span className="text-muted-foreground">Progress</span>
              <span className="font-mono tabular-nums text-right">
                {position.steps_completed} / {position.steps_total} steps
              </span>
              {position.stop_reason && (
                <>
                  <span className="text-muted-foreground">Stop reason</span>
                  <span className="text-right">
                    <span className="text-[0.625rem] text-amber-600 dark:text-amber-400">
                      {position.stop_reason.replace(/_/g, " ")}
                    </span>
                  </span>
                </>
              )}
            </div>
            <div className="mt-2 h-1.5 w-full rounded-full bg-muted overflow-hidden">
              <div
                className={cn(
                  "h-full rounded-full transition-all",
                  position.stop_reason
                    ? "bg-amber-500"
                    : position.steps_completed === position.steps_total
                      ? "bg-emerald-500"
                      : "bg-violet-500"
                )}
                style={{
                  width: `${(position.steps_completed / position.steps_total) * 100}%`,
                }}
              />
            </div>
          </div>
        )}

        {/* Timing */}
        <div>
          <SectionHeader className="mb-2">
            <Clock className="inline h-3 w-3 mr-0.5 align-text-top" />
            Timing
          </SectionHeader>
          <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
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
            <span className="text-muted-foreground">Duration</span>
            <span className="font-mono tabular-nums text-right">
              {formatDuration(
                position.opened_at || position.created_at,
                position.closed_at
              )}
            </span>
          </div>
        </div>

        {/* Error message */}
        {position.error_message && (
          <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 flex items-start gap-2">
            <AlertTriangle className="h-3.5 w-3.5 text-destructive shrink-0 mt-0.5" />
            <p className="text-xs text-destructive">{position.error_message}</p>
          </div>
        )}
      </div>
    </div>
  );
}
