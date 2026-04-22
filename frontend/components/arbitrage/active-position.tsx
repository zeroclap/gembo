"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { adjustPositionSize, closePosition, fetchExchangeCredentials, transferMargin, type Position } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  ExchangeBadge,
  StatusBadge,
  PnlValue,
  SectionHeader,
} from "@/components/arbitrage/shared";
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
  Activity,
  AlertTriangle,
  Settings2,
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
  const [modifyOpen, setModifyOpen] = useState(false);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);

  const closeMutation = useMutation({
    mutationFn: closePosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
      setCloseConfirmOpen(false);
    },
  });

  const { data: credentials } = useQuery({
    queryKey: ["credentials"],
    queryFn: fetchExchangeCredentials,
    staleTime: 60_000,
  });
  const credLabel = (id: string) => {
    const c = credentials?.find((x) => x.id === id);
    return c ? c.title : id.slice(-8);
  };

  const canClose =
    position.status === "open" || position.status === "partially_open";
  const canModify =
    position.status === "open" || position.status === "partially_open";

  // Computed metrics
  const longUnrealized = parseFloat(position.long_unrealized_pnl) || 0;
  const shortUnrealized = parseFloat(position.short_unrealized_pnl) || 0;
  const priceChangePnl = longUnrealized + shortUnrealized;
  const fundingPnl = parseFloat(position.total_funding_pnl) || 0;

  const longEntryFees = parseFloat(position.long_entry_fees) || 0;
  const shortEntryFees = parseFloat(position.short_entry_fees) || 0;
  const totalEntryFees = longEntryFees + shortEntryFees;

  // Close fees: populated on close; while open, show as 0 / est.
  const longCloseFees = parseFloat(position.long_close_fees) || 0;
  const shortCloseFees = parseFloat(position.short_close_fees) || 0;
  const totalCloseFees = longCloseFees + shortCloseFees;

  // Realized = entry fees (negative) + funding PnL
  const realizedPnl = -Math.abs(totalEntryFees) + fundingPnl;
  // Unrealized = price change PnL + close fees (negative while open = 0; negative when closed)
  const unrealizedPnl = priceChangePnl - Math.abs(totalCloseFees);
  // Total
  const totalPnl = realizedPnl + unrealizedPnl;

  // Estimated close fee while open: use entry fee rate × notional as proxy.
  // Entry fee rate = totalEntryFees / (longEntryCost + shortEntryCost).
  const longNotional = parseFloat(position.long_notional) || 0;
  const shortNotional = parseFloat(position.short_notional) || 0;
  const longEntryCost = parseFloat(position.long_entry_cost) || 0;
  const shortEntryCost = parseFloat(position.short_entry_cost) || 0;
  const totalEntryCost = longEntryCost + shortEntryCost;
  const estCloseFee = totalEntryCost > 0
    ? (Math.abs(totalEntryFees) / totalEntryCost) * (longNotional + shortNotional)
    : 0;
  const estTotalPnl = totalPnl - estCloseFee;

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
        </div>
        <div className="flex items-center gap-1.5">
          {canModify && (
            <Popover open={modifyOpen} onOpenChange={setModifyOpen}>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className={cn(
                    "h-7 px-2 text-xs transition-colors",
                    modifyOpen && "bg-muted text-foreground"
                  )}
                >
                  <Settings2 className="h-3 w-3 mr-1" />
                  Modify
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-72 p-3 space-y-2" align="end">
                <ModifySizePanel position={position} />
                {(position.long_margin_mode === "isolated" || position.short_margin_mode === "isolated") && (
                  <>
                    <Separator />
                    <ModifyMarginPanel position={position} />
                  </>
                )}
              </PopoverContent>
            </Popover>
          )}
          {canClose && (
            <Popover open={closeConfirmOpen} onOpenChange={setCloseConfirmOpen}>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2 text-xs text-red-600 dark:text-red-400 border-red-500/30 hover:bg-red-500/10"
                  disabled={closeMutation.isPending}
                >
                  {closeMutation.isPending ? (
                    <Loader2 className="h-3 w-3 animate-spin mr-1" />
                  ) : (
                    <X className="h-3 w-3 mr-1" />
                  )}
                  Close
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-64 p-3" align="end">
                <div className="space-y-1.5">
                  <p className="text-xs font-medium">Close position?</p>
                  <p className="text-[0.625rem] text-muted-foreground">
                    Market orders will be placed on both legs.
                  </p>
                  <Separator className="my-1" />
                  <div className="space-y-0.5">
                    <div className="flex justify-between items-baseline">
                      <span className="text-[0.625rem] text-muted-foreground">Current P&L</span>
                      <PnlValue value={String(totalPnl)} className="text-[0.625rem]" />
                    </div>
                    <div className="flex justify-between items-baseline">
                      <span className="text-[0.625rem] text-muted-foreground">Est. close fee</span>
                      <span className="text-[0.625rem] font-mono tabular-nums text-muted-foreground">
                        −{formatUsdPlain(String(estCloseFee))}
                      </span>
                    </div>
                    <Separator className="opacity-50 my-0.5" />
                    <div className="flex justify-between items-baseline">
                      <span className="text-[0.625rem] font-medium">Est. P&L after close</span>
                      <PnlValue value={String(estTotalPnl)} className="text-[0.625rem] font-medium" />
                    </div>
                  </div>
                  <div className="flex gap-1.5 pt-0.5">
                    <Button
                      size="sm"
                      className="w-full h-7 text-xs bg-red-600 hover:bg-red-700 text-white"
                      disabled={closeMutation.isPending}
                      onClick={() => closeMutation.mutate(position.id)}
                    >
                      {closeMutation.isPending ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : "Confirm Close"}
                    </Button>
                  </div>
                  {closeMutation.error && (
                    <p className="text-[0.625rem] text-destructive">
                      {(closeMutation.error as Error).message}
                    </p>
                  )}
                </div>
              </PopoverContent>
            </Popover>
          )}
        </div>
      </div>

      <div className="p-4 space-y-4">
        {/* P&L */}
        <div className="space-y-2">
          {/* Total — full-width prominent row */}
          <div className="flex items-baseline justify-between">
            <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
              Total P&amp;L
            </span>
            <PnlValue value={String(totalPnl)} className="text-base font-semibold" />
          </div>

          {/* Realized / Unrealized — 2-col sub-table */}
          <div className="grid grid-cols-2 gap-px rounded-md overflow-hidden border border-border/40">
            {/* Realized */}
            <div className="bg-muted/20 px-2.5 py-2 space-y-1.5">
              <div className="flex items-baseline justify-between">
                <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">Realized</span>
                <PnlValue value={String(realizedPnl)} className="text-xs font-medium" />
              </div>
              <div className="border-t border-border/30 pt-1.5 space-y-0.5">
                <div className="flex items-baseline justify-between">
                  <span className="text-[0.625rem] text-muted-foreground">Funding</span>
                  <PnlValue value={String(fundingPnl)} className="text-[0.625rem]" />
                </div>
                <div className="flex items-baseline justify-between">
                  <span className="text-[0.625rem] text-muted-foreground">Entry fees</span>
                  <span className="text-[0.625rem] font-mono tabular-nums text-red-600 dark:text-red-400">
                    -{formatUsdPlain(String(Math.abs(totalEntryFees)))}
                  </span>
                </div>
              </div>
            </div>

            {/* Unrealized */}
            <div className="bg-muted/20 px-2.5 py-2 space-y-1.5">
              <div className="flex items-baseline justify-between">
                <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">Unrealized</span>
                <PnlValue value={String(unrealizedPnl)} className="text-xs font-medium" />
              </div>
              <div className="border-t border-border/30 pt-1.5 space-y-0.5">
                <div className="flex items-baseline justify-between">
                  <span className="text-[0.625rem] text-muted-foreground">Price Δ</span>
                  <PnlValue value={String(priceChangePnl)} className="text-[0.625rem]" />
                </div>
                <div className="flex items-baseline justify-between">
                  <span className="text-[0.625rem] text-muted-foreground">Close fees</span>
                  {totalCloseFees > 0 ? (
                    <span className="text-[0.625rem] font-mono tabular-nums text-red-600 dark:text-red-400">
                      -{formatUsdPlain(String(Math.abs(totalCloseFees)))}
                    </span>
                  ) : (
                    <span className="text-[0.625rem] text-muted-foreground/50">—</span>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>

        <Separator />

        {/* Leg cards */}
        <div className="grid grid-cols-2 gap-2">
          {(["long", "short"] as const).map((side) => {
            const isLong = side === "long";
            const exchangeId = isLong ? position.long_exchange_id : position.short_exchange_id;
            const credId    = isLong ? position.long_credential_id : position.short_credential_id;
            const entryPx   = isLong ? position.long_entry_price : position.short_entry_price;
            const markPx    = isLong ? position.long_mark_price : position.short_mark_price;
            const filledQty = isLong ? position.long_filled_qty : position.short_filled_qty;
            const baseQty   = isLong ? position.long_filled_base_qty : position.short_filled_base_qty;
            const contractSz= isLong ? position.long_contract_size : position.short_contract_size;
            const leverage  = isLong ? position.long_leverage : position.short_leverage;
            const marginMode= isLong ? position.long_margin_mode : position.short_margin_mode;
            const liqPx     = isLong ? position.long_liquidation_price : position.short_liquidation_price;
            const unrealized= isLong ? position.long_unrealized_pnl : position.short_unrealized_pnl;
            const initMargin= isLong ? position.long_initial_margin : position.short_initial_margin;
            const maintMargin=isLong ? position.long_maintenance_margin : position.short_maintenance_margin;
            const collateral= isLong ? position.long_collateral : position.short_collateral;
            const marginRatio=isLong ? position.long_margin_ratio : position.short_margin_ratio;
            const adlQuantile=isLong ? (position.long_adl_quantile ?? 0) : (position.short_adl_quantile ?? 0);

            const ratio = parseFloat(marginRatio) || 0;
            const ratioColor = ratio > 0.8
              ? "text-red-600 dark:text-red-400"
              : ratio > 0.5
                ? "text-amber-600 dark:text-amber-400"
                : "text-muted-foreground";

            return (
              <div key={side} className="rounded-md border border-border/50 bg-muted/10 p-2.5 space-y-2">
                {/* Leg header */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5">
                    {isLong
                      ? <TrendingUp className="h-3 w-3 text-emerald-500" />
                      : <TrendingDown className="h-3 w-3 text-red-500" />}
                    <span className={cn(
                      "text-[0.625rem] font-semibold uppercase tracking-wider",
                      isLong ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
                    )}>
                      {side}
                    </span>
                  </div>
                  <ExchangeBadge exchangeId={exchangeId} />
                </div>

                {/* Credential */}
                <div className="text-[0.625rem] text-muted-foreground truncate">
                  {credLabel(credId)}
                </div>

                <Separator className="opacity-50" />

                {/* Price */}
                <div className="space-y-0.5">
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">Entry</span>
                    <span className="text-xs font-mono tabular-nums">{formatPrice(entryPx)}</span>
                  </div>
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">Mark</span>
                    <span className="text-xs font-mono tabular-nums">{formatPrice(markPx)}</span>
                  </div>
                  {liqPx !== "0" && liqPx !== "" && (
                    <div className="flex justify-between items-baseline">
                      <span className="text-[0.625rem] text-muted-foreground">Liq.</span>
                      <span className="text-[0.625rem] font-mono tabular-nums text-amber-600 dark:text-amber-400">
                        {formatPrice(liqPx)}
                      </span>
                    </div>
                  )}
                </div>

                <Separator className="opacity-50" />

                {/* Qty */}
                <div className="space-y-0.5">
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">Qty</span>
                    <span className="text-xs font-mono tabular-nums">{formatQty(filledQty)}</span>
                  </div>
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">Base</span>
                    <span className="text-xs font-mono tabular-nums">{formatQty(baseQty)}</span>
                  </div>
                  {parseFloat(contractSz) > 0 && (
                    <div className="flex justify-between items-baseline">
                      <span className="text-[0.625rem] text-muted-foreground">Contract</span>
                      <span className="text-[0.625rem] font-mono tabular-nums text-muted-foreground">
                        {formatQty(contractSz)}
                      </span>
                    </div>
                  )}
                </div>

                <Separator className="opacity-50" />

                {/* Setup */}
                <div className="flex items-center justify-between gap-1">
                  {leverage > 0 && (
                    <span className="text-[0.625rem] font-mono text-muted-foreground">{leverage}×</span>
                  )}
                  {marginMode && (
                    <span className={cn(
                      "text-[0.625rem] font-medium px-1 py-0.5 rounded ml-auto",
                      marginMode === "isolated"
                        ? "bg-amber-500/10 text-amber-600 dark:text-amber-400"
                        : "bg-muted text-muted-foreground"
                    )}>
                      {marginMode}
                    </span>
                  )}
                </div>

                <Separator className="opacity-50" />

                {/* P&L + margin */}
                <div className="space-y-0.5">
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">Unrealized</span>
                    <PnlValue value={unrealized} className="text-[0.625rem]" />
                  </div>
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">Collateral</span>
                    <span className="text-[0.625rem] font-mono tabular-nums">
                      {parseFloat(collateral) > 0 ? formatUsdPlain(collateral) : "—"}
                    </span>
                  </div>
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">IM</span>
                    <span className="text-[0.625rem] font-mono tabular-nums">
                      {parseFloat(initMargin) > 0 ? formatUsdPlain(initMargin) : "—"}
                    </span>
                  </div>
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">MM</span>
                    <span className="text-[0.625rem] font-mono tabular-nums">
                      {parseFloat(maintMargin) > 0 ? formatUsdPlain(maintMargin) : "—"}
                    </span>
                  </div>
                  <div className="flex justify-between items-baseline">
                    <span className="text-[0.625rem] text-muted-foreground">Ratio</span>
                    {ratio > 0 ? (
                      <span className={cn("text-[0.625rem] font-mono tabular-nums", ratioColor)}>
                        {(ratio * 100).toFixed(1)}%
                      </span>
                    ) : (
                      <span className="text-[0.625rem] font-mono tabular-nums text-muted-foreground">—</span>
                    )}
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-[0.625rem] text-muted-foreground">ADL</span>
                    {adlQuantile > 0 ? (
                      <div className="flex items-end gap-px">
                        {[1, 2, 3, 4, 5].map((level) => {
                          const filled = level <= adlQuantile;
                          const color = adlQuantile >= 4
                            ? filled ? "bg-red-500 dark:bg-red-400" : "bg-muted-foreground/20"
                            : adlQuantile === 3
                              ? filled ? "bg-amber-500 dark:bg-amber-400" : "bg-muted-foreground/20"
                              : filled ? "bg-emerald-500 dark:bg-emerald-400" : "bg-muted-foreground/20";
                          return (
                            <div
                              key={level}
                              style={{ height: `${4 + level * 2}px` }}
                              className={cn("w-1.5 rounded-sm transition-colors", color)}
                            />
                          );
                        })}
                      </div>
                    ) : (
                      <span className="text-[0.625rem] font-mono tabular-nums text-muted-foreground">—</span>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {/* Stepped execution */}
        {position.steps_total > 0 && (
          <div className="space-y-1.5">
            <div className="flex items-center justify-between text-[0.625rem] text-muted-foreground">
              <span>Steps</span>
              <span className="font-mono tabular-nums">
                {position.steps_completed} / {position.steps_total}
                {position.stop_reason && (
                  <span className="ml-1.5 text-amber-600 dark:text-amber-400">
                    · {position.stop_reason.replace(/_/g, " ")}
                  </span>
                )}
              </span>
            </div>
            <div className="h-1 w-full rounded-full bg-muted overflow-hidden">
              <div
                className={cn(
                  "h-full rounded-full transition-all",
                  position.stop_reason ? "bg-amber-500"
                    : position.steps_completed === position.steps_total ? "bg-emerald-500"
                    : "bg-violet-500"
                )}
                style={{ width: `${(position.steps_completed / position.steps_total) * 100}%` }}
              />
            </div>
          </div>
        )}

        {/* Timing */}
        <div className="flex items-center gap-3 text-[0.625rem] text-muted-foreground">
          <Clock className="h-3 w-3 shrink-0" />
          <span>{formatTimeShort(position.opened_at || position.created_at)}</span>
          <span className="text-border">·</span>
          <span className="font-mono tabular-nums">
            {formatDuration(position.opened_at || position.created_at, position.closed_at)}
          </span>
        </div>

        {/* Error */}
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

// ---------------------------------------------------------------------------
// ModifySizePanel — adjust position size (increase / reduce)
// ---------------------------------------------------------------------------

function ModifySizePanel({ position }: { position: Position }) {
  const queryClient = useQueryClient();
  const [action, setAction] = useState<"increase" | "reduce">("increase");
  const [deltaBaseQty, setDeltaBaseQty] = useState("");

  const adjustMutation = useMutation({
    mutationFn: () =>
      adjustPositionSize(position.id, {
        action,
        delta_base_qty: deltaBaseQty.trim(),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
      setDeltaBaseQty("");
    },
  });

  return (
    <div className="space-y-1.5">
      <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
        Position Size
      </div>

      <div className="flex items-center gap-1.5">
        {/* Inc / Red toggle */}
        <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5 shrink-0">
          {(["increase", "reduce"] as const).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => setAction(s)}
              className={cn(
                "rounded-sm h-5 px-1.5 text-[0.625rem] font-medium transition-colors capitalize whitespace-nowrap",
                action === s
                  ? "bg-background text-foreground shadow-sm border border-border/50"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              {s === "increase" ? "Inc" : "Red"}
            </button>
          ))}
        </div>

        {/* Input */}
        <Input
          type="text"
          inputMode="decimal"
          value={deltaBaseQty}
          onChange={(e) => setDeltaBaseQty(e.target.value)}
          placeholder="base qty"
          className="font-mono tabular-nums text-xs h-7 min-w-0"
        />
      </div>

      <Button
        size="sm"
        className="h-6 px-2 text-xs w-full"
        disabled={!deltaBaseQty.trim() || deltaBaseQty === "0" || adjustMutation.isPending}
        onClick={() => adjustMutation.mutate()}
      >
        {adjustMutation.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : "Apply"}
      </Button>

      {adjustMutation.error && (
        <p className="text-[0.625rem] text-destructive">
          {(adjustMutation.error as Error).message}
        </p>
      )}
      {adjustMutation.isSuccess && (
        <p className="text-[0.625rem] text-emerald-600 dark:text-emerald-400">Size adjusted</p>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ModifyMarginPanel — transfer isolated margin on a leg
// ---------------------------------------------------------------------------

function ModifyMarginPanel({ position }: { position: Position }) {
  const queryClient = useQueryClient();
  const [leg, setLeg] = useState<"long" | "short">("long");
  const [action, setAction] = useState<"increase" | "reduce">("increase");
  const [amount, setAmount] = useState("");

  const adjustMutation = useMutation({
    mutationFn: () => {
      const normalized = amount.trim().replace(/^[-+]/, "");
      const signedAmount = action === "reduce" ? `-${normalized}` : normalized;
      return transferMargin(position.id, { leg, amount: signedAmount });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
      setAmount("");
    },
  });

  return (
    <div className="space-y-1.5">
      <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
        Margin
      </div>

      <div className="flex items-center gap-1.5">
        {/* Leg toggle */}
        <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5 shrink-0">
          {(["long", "short"] as const).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => setLeg(s)}
              className={cn(
                "rounded-sm h-5 px-1.5 text-[0.625rem] font-medium transition-colors capitalize whitespace-nowrap",
                leg === s
                  ? "bg-background text-foreground shadow-sm border border-border/50"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              {s === "long" ? "L" : "S"}
            </button>
          ))}
        </div>

        {/* Inc / Red toggle */}
        <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5 shrink-0">
          {(["increase", "reduce"] as const).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => setAction(s)}
              className={cn(
                "rounded-sm h-5 px-1.5 text-[0.625rem] font-medium transition-colors capitalize whitespace-nowrap",
                action === s
                  ? "bg-background text-foreground shadow-sm border border-border/50"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              {s === "increase" ? "Inc" : "Red"}
            </button>
          ))}
        </div>

        {/* Amount input */}
        <Input
          type="text"
          inputMode="decimal"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          placeholder="USDT"
          className="font-mono tabular-nums text-xs h-7 min-w-0"
        />
      </div>

      <Button
        size="sm"
        className="h-6 px-2 text-xs w-full"
        disabled={!amount.trim() || amount === "0" || adjustMutation.isPending}
        onClick={() => adjustMutation.mutate()}
      >
        {adjustMutation.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : "Apply"}
      </Button>

      {adjustMutation.error && (
        <p className="text-[0.625rem] text-destructive">
          {(adjustMutation.error as Error).message}
        </p>
      )}
      {adjustMutation.isSuccess && (
        <p className="text-[0.625rem] text-emerald-600 dark:text-emerald-400">Margin adjusted</p>
      )}
    </div>
  );
}

