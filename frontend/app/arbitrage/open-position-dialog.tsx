"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  createPosition,
  type FundingArbitrage,
  type SizeType,
} from "@/lib/api";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown, Loader2 } from "lucide-react";

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

function formatRate(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  const pct = num * 100;
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
}

function formatAPR(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(2)}%`;
}

export function OpenPositionDialog({
  arb,
  open,
  onOpenChange,
}: {
  arb: FundingArbitrage;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [sizeType, setSizeType] = useState<SizeType>("usd");
  const [size, setSize] = useState("1000");
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: createPosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
      onOpenChange(false);
      setSize("1000");
    },
  });

  const sizeNum = parseFloat(size);
  const isValidSize = !isNaN(sizeNum) && sizeNum > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Open Position &mdash; {arb.symbol}</DialogTitle>
          <DialogDescription>
            Open a funding rate arbitrage position. Long on one exchange, short
            on another.
          </DialogDescription>
        </DialogHeader>

        {/* Pair summary */}
        <div className="rounded-lg border p-3">
          <div className="flex flex-col gap-2">
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                <ArrowUp className="h-3 w-3 text-white" strokeWidth={2.5} />
              </span>
              <span className="text-xs text-muted-foreground">Long</span>
              <ExchangeBadge exchangeId={arb.long.exchange_id} />
              <span className="flex-1" />
              <span
                className={cn(
                  "text-xs font-mono tabular-nums",
                  parseFloat(arb.long.funding_rate) >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {formatRate(arb.long.funding_rate)}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                <ArrowDown className="h-3 w-3 text-white" strokeWidth={2.5} />
              </span>
              <span className="text-xs text-muted-foreground">Short</span>
              <ExchangeBadge exchangeId={arb.short.exchange_id} />
              <span className="flex-1" />
              <span
                className={cn(
                  "text-xs font-mono tabular-nums",
                  parseFloat(arb.short.funding_rate) >= 0
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-red-600 dark:text-red-400"
                )}
              >
                {formatRate(arb.short.funding_rate)}
              </span>
            </div>
            <div className="flex items-center justify-between border-t pt-2 mt-1">
              <span className="text-xs text-muted-foreground">Net APR</span>
              <span
                className={cn(
                  "text-xs font-mono tabular-nums font-medium",
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

        {/* Size type toggle */}
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Size Type</Label>
            <div className="flex gap-1">
              <Button
                variant={sizeType === "usd" ? "default" : "outline"}
                size="sm"
                className="h-7 flex-1 text-xs"
                onClick={() => setSizeType("usd")}
              >
                USD Notional
              </Button>
              <Button
                variant={sizeType === "base" ? "default" : "outline"}
                size="sm"
                className="h-7 flex-1 text-xs"
                onClick={() => setSizeType("base")}
              >
                Base Quantity
              </Button>
            </div>
          </div>

          {/* Size input */}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="position-size" className="text-xs">
              {sizeType === "usd" ? "Amount (USD)" : "Quantity (Base)"}
            </Label>
            <Input
              id="position-size"
              type="text"
              inputMode="decimal"
              placeholder={sizeType === "usd" ? "1000" : "0.1"}
              value={size}
              onChange={(e) => setSize(e.target.value)}
              className="font-mono"
            />
          </div>
        </div>

        {/* Error */}
        {mutation.error && (
          <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2">
            <p className="text-xs text-destructive">
              {mutation.error.message}
            </p>
          </div>
        )}

        <DialogFooter>
          <Button
            variant="outline"
            size="sm"
            className="text-xs"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            size="sm"
            className="text-xs"
            disabled={!isValidSize || mutation.isPending}
            onClick={() => {
              mutation.mutate({
                symbol: arb.symbol,
                long_exchange_id: arb.long.exchange_id,
                short_exchange_id: arb.short.exchange_id,
                size_type: sizeType,
                requested_size: size,
              });
            }}
          >
            {mutation.isPending && (
              <Loader2 className="h-3 w-3 animate-spin mr-1" />
            )}
            Open Position
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
