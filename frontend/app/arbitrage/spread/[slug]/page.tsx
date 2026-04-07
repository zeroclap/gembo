"use client";

import { useParams, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useMemo, useCallback } from "react";
import {
  fetchSpreadArbitrage,
  createPosition,
  type SpreadArbitrage,
  type SizeType,
  type CreatePositionRequest,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  ArrowLeft,
  TrendingUp,
  TrendingDown,
  Loader2,
  Info,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Formatting helpers (kept local to avoid cross-file imports for now)
// ---------------------------------------------------------------------------

function formatPct(value: string, decimals = 4): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(decimals)}%`;
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

function formatUsd(value: string): string {
  const num = parseFloat(value);
  if (!isFinite(num)) return "-";
  const abs = Math.abs(num);
  if (abs >= 1000) {
    return `$${abs.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  }
  return `$${abs.toFixed(2)}`;
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

// ---------------------------------------------------------------------------
// Slug encoding / decoding
// ---------------------------------------------------------------------------

/** Decode slug back to [symbol, buyExchange, sellExchange]. */
function decodeSpreadSlug(slug: string): [string, string, string] | null {
  const parts = slug.split("--");
  if (parts.length !== 3) return null;
  return [
    decodeURIComponent(parts[0]),
    decodeURIComponent(parts[1]),
    decodeURIComponent(parts[2]),
  ];
}

// ---------------------------------------------------------------------------
// Duration helpers
// ---------------------------------------------------------------------------

function formatDuration(seconds: number): string {
  if (seconds <= 0) return "No limit";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (h > 0 && m > 0) return `${h}h ${m}m`;
  if (h > 0) return `${h}h`;
  return `${m}m`;
}

const DURATION_PRESETS = [
  { label: "1h", seconds: 3600 },
  { label: "4h", seconds: 14400 },
  { label: "12h", seconds: 43200 },
  { label: "24h", seconds: 86400 },
  { label: "48h", seconds: 172800 },
  { label: "7d", seconds: 604800 },
  { label: "None", seconds: 0 },
];

// ---------------------------------------------------------------------------
// Position Form
// ---------------------------------------------------------------------------

function PositionForm({
  arb,
  onSuccess,
}: {
  arb: SpreadArbitrage;
  onSuccess: () => void;
}) {
  const queryClient = useQueryClient();

  // Basic fields
  const [sizeType, setSizeType] = useState<SizeType>("usd");
  const [size, setSize] = useState("1000");

  // Spread-specific fields
  const [targetSpreadPct, setTargetSpreadPct] = useState("0");
  const [stopLossSpreadPct, setStopLossSpreadPct] = useState("");
  const [maxHoldDuration, setMaxHoldDuration] = useState(0); // seconds

  // Stepped execution fields
  const [portionSize, setPortionSize] = useState("");
  const [priceMin, setPriceMin] = useState("");
  const [priceMax, setPriceMax] = useState("");
  const [slippagePct, setSlippagePct] = useState("");
  const [leverage, setLeverage] = useState("");
  const [marginCapital, setMarginCapital] = useState("");

  const mutation = useMutation({
    mutationFn: createPosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
      onSuccess();
    },
  });

  const sizeNum = parseFloat(size);
  const isValidSize = !isNaN(sizeNum) && sizeNum > 0;

  const isStepped = portionSize.trim() !== "" && parseFloat(portionSize) > 0;

  const handleSubmit = useCallback(() => {
    const req: CreatePositionRequest = {
      symbol: arb.symbol,
      long_exchange_id: arb.buy_exchange_id, // buy = long in API
      short_exchange_id: arb.sell_exchange_id, // sell = short in API
      size_type: sizeType,
      requested_size: size,
      arbitrage_type: "spread",
    };

    // Spread-specific fields
    if (targetSpreadPct.trim()) req.target_spread_pct = targetSpreadPct;
    if (stopLossSpreadPct.trim()) req.stop_loss_spread_pct = stopLossSpreadPct;
    if (maxHoldDuration > 0) req.max_hold_duration = maxHoldDuration;

    // Stepped execution fields
    if (portionSize.trim()) req.portion_size = portionSize;
    if (priceMin.trim()) req.price_min = priceMin;
    if (priceMax.trim()) req.price_max = priceMax;
    if (slippagePct.trim()) req.slippage_pct = slippagePct;
    const lev = parseInt(leverage, 10);
    if (!isNaN(lev) && lev > 0) req.leverage = lev;
    if (marginCapital.trim()) req.margin_capital = marginCapital;

    mutation.mutate(req);
  }, [
    arb,
    sizeType,
    size,
    targetSpreadPct,
    stopLossSpreadPct,
    maxHoldDuration,
    portionSize,
    priceMin,
    priceMax,
    slippagePct,
    leverage,
    marginCapital,
    mutation,
  ]);

  // Compute estimated steps for preview
  const estimatedSteps = useMemo(() => {
    const total = parseFloat(size);
    const portion = parseFloat(portionSize);
    if (!total || !portion || total <= 0 || portion <= 0) return null;
    return Math.ceil(total / portion);
  }, [size, portionSize]);

  return (
    <div className="space-y-5">
      {/* Section: Size */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Size
        </div>

        {/* Size type toggle */}
        <div className="space-y-1.5">
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

        {/* Total size */}
        <div className="space-y-1.5">
          <Label htmlFor="total-size" className="text-xs">
            {sizeType === "usd" ? "Total Size (USD)" : "Total Quantity (Base)"}
          </Label>
          <Input
            id="total-size"
            type="text"
            inputMode="decimal"
            placeholder={sizeType === "usd" ? "1000" : "0.1"}
            value={size}
            onChange={(e) => setSize(e.target.value)}
            className="font-mono h-8"
          />
        </div>
      </div>

      <Separator />

      {/* Section: Spread Parameters */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Spread Parameters
        </div>

        {/* Target spread */}
        <div className="space-y-1.5">
          <Label htmlFor="target-spread" className="text-xs">
            Target Spread (%)
          </Label>
          <Input
            id="target-spread"
            type="text"
            inputMode="decimal"
            placeholder="0 = manual close only"
            value={targetSpreadPct}
            onChange={(e) => setTargetSpreadPct(e.target.value)}
            className="font-mono h-8"
          />
          <p className="text-[0.625rem] text-muted-foreground leading-relaxed">
            Close automatically when spread narrows to this level. Set to 0 for manual close.
          </p>
        </div>

        {/* Stop-loss spread */}
        <div className="space-y-1.5">
          <Label htmlFor="stop-loss-spread" className="text-xs">
            Stop-Loss Spread (%)
          </Label>
          <Input
            id="stop-loss-spread"
            type="text"
            inputMode="decimal"
            placeholder="Empty = no stop-loss"
            value={stopLossSpreadPct}
            onChange={(e) => setStopLossSpreadPct(e.target.value)}
            className="font-mono h-8"
          />
          <p className="text-[0.625rem] text-muted-foreground leading-relaxed">
            Close automatically if spread widens beyond this level.
          </p>
        </div>

        {/* Max hold duration */}
        <div className="space-y-1.5">
          <Label className="text-xs">Max Hold Duration</Label>
          <div className="flex flex-wrap gap-1">
            {DURATION_PRESETS.map((preset) => (
              <Button
                key={preset.label}
                variant={maxHoldDuration === preset.seconds ? "default" : "outline"}
                size="sm"
                className="h-7 text-xs px-2"
                onClick={() => setMaxHoldDuration(preset.seconds)}
              >
                {preset.label}
              </Button>
            ))}
          </div>
          <p className="text-[0.625rem] text-muted-foreground leading-relaxed">
            {maxHoldDuration > 0
              ? `Position will auto-close after ${formatDuration(maxHoldDuration)}.`
              : "No time limit on position hold."}
          </p>
        </div>
      </div>

      <Separator />

      {/* Section: Stepped Execution */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            Stepped Execution
          </div>
          {isStepped && estimatedSteps && (
            <Badge variant="outline" className="text-[0.625rem] font-mono">
              {estimatedSteps} steps
            </Badge>
          )}
        </div>

        <div className="text-[0.625rem] text-muted-foreground leading-relaxed">
          Split the total size into smaller portions executed sequentially. Leave empty for single-shot execution.
        </div>

        {/* Portion size */}
        <div className="space-y-1.5">
          <Label htmlFor="portion-size" className="text-xs">
            Portion Size {sizeType === "usd" ? "(USD)" : "(Base)"}
          </Label>
          <Input
            id="portion-size"
            type="text"
            inputMode="decimal"
            placeholder="Empty = single-shot"
            value={portionSize}
            onChange={(e) => setPortionSize(e.target.value)}
            className="font-mono h-8"
          />
        </div>

        {/* Price bounds */}
        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1.5">
            <Label htmlFor="price-min" className="text-xs">
              Price Min
            </Label>
            <Input
              id="price-min"
              type="text"
              inputMode="decimal"
              placeholder="No limit"
              value={priceMin}
              onChange={(e) => setPriceMin(e.target.value)}
              className="font-mono h-8"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="price-max" className="text-xs">
              Price Max
            </Label>
            <Input
              id="price-max"
              type="text"
              inputMode="decimal"
              placeholder="No limit"
              value={priceMax}
              onChange={(e) => setPriceMax(e.target.value)}
              className="font-mono h-8"
            />
          </div>
        </div>

        {/* Slippage */}
        <div className="space-y-1.5">
          <Label htmlFor="slippage-pct" className="text-xs">
            Max Slippage (%)
          </Label>
          <Input
            id="slippage-pct"
            type="text"
            inputMode="decimal"
            placeholder="e.g. 0.1"
            value={slippagePct}
            onChange={(e) => setSlippagePct(e.target.value)}
            className="font-mono h-8"
          />
        </div>
      </div>

      <Separator />

      {/* Section: Capital & Leverage */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Capital &amp; Leverage
        </div>

        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1.5">
            <Label htmlFor="leverage" className="text-xs">
              Leverage
            </Label>
            <Input
              id="leverage"
              type="text"
              inputMode="numeric"
              placeholder="0 = default"
              value={leverage}
              onChange={(e) => setLeverage(e.target.value)}
              className="font-mono h-8"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="margin-capital" className="text-xs">
              Margin Capital ($)
            </Label>
            <Input
              id="margin-capital"
              type="text"
              inputMode="decimal"
              placeholder="Unchecked"
              value={marginCapital}
              onChange={(e) => setMarginCapital(e.target.value)}
              className="font-mono h-8"
            />
          </div>
        </div>
      </div>

      <Separator />

      {/* Preview */}
      {isStepped && estimatedSteps && (
        <div className="rounded-lg border bg-muted/30 p-3 space-y-1">
          <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground flex items-center gap-1">
            <Info className="h-3 w-3" />
            Execution Preview
          </div>
          <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
            <span className="text-muted-foreground">Total size</span>
            <span className="font-mono tabular-nums text-right">
              {sizeType === "usd" ? "$" : ""}{size}
              <span className="ml-1 text-muted-foreground text-[0.625rem] uppercase">{sizeType}</span>
            </span>
            <span className="text-muted-foreground">Portion size</span>
            <span className="font-mono tabular-nums text-right">
              {sizeType === "usd" ? "$" : ""}{portionSize}
              <span className="ml-1 text-muted-foreground text-[0.625rem] uppercase">{sizeType}</span>
            </span>
            <span className="text-muted-foreground">Steps</span>
            <span className="font-mono tabular-nums text-right">{estimatedSteps}</span>
            {priceMin && (
              <>
                <span className="text-muted-foreground">Price min</span>
                <span className="font-mono tabular-nums text-right">${priceMin}</span>
              </>
            )}
            {priceMax && (
              <>
                <span className="text-muted-foreground">Price max</span>
                <span className="font-mono tabular-nums text-right">${priceMax}</span>
              </>
            )}
            {slippagePct && (
              <>
                <span className="text-muted-foreground">Max slippage</span>
                <span className="font-mono tabular-nums text-right">{slippagePct}%</span>
              </>
            )}
          </div>
        </div>
      )}

      {/* Spread params preview */}
      <div className="rounded-lg border bg-muted/30 p-3 space-y-1">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground flex items-center gap-1">
          <Info className="h-3 w-3" />
          Spread Strategy
        </div>
        <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
          <span className="text-muted-foreground">Target spread</span>
          <span className="font-mono tabular-nums text-right">
            {targetSpreadPct && parseFloat(targetSpreadPct) > 0
              ? `${targetSpreadPct}%`
              : "Manual close"}
          </span>
          <span className="text-muted-foreground">Stop-loss spread</span>
          <span className="font-mono tabular-nums text-right">
            {stopLossSpreadPct && parseFloat(stopLossSpreadPct) > 0
              ? `${stopLossSpreadPct}%`
              : "None"}
          </span>
          <span className="text-muted-foreground">Max hold</span>
          <span className="font-mono tabular-nums text-right">
            {formatDuration(maxHoldDuration)}
          </span>
        </div>
      </div>

      {/* Error */}
      {mutation.error && (
        <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2">
          <p className="text-xs text-destructive">{mutation.error.message}</p>
        </div>
      )}

      {/* Success */}
      {mutation.isSuccess && (
        <div className="rounded-md bg-emerald-500/10 border border-emerald-500/20 px-3 py-2">
          <p className="text-xs text-emerald-600 dark:text-emerald-400">
            Position created successfully.
          </p>
        </div>
      )}

      {/* Submit */}
      <Button
        className="w-full"
        size="sm"
        disabled={!isValidSize || mutation.isPending}
        onClick={handleSubmit}
      >
        {mutation.isPending && <Loader2 className="h-3 w-3 animate-spin mr-1" />}
        {isStepped
          ? `Open Spread Position (${estimatedSteps} steps)`
          : "Open Spread Position"}
      </Button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Left panel — Opportunity Context
// ---------------------------------------------------------------------------

function OpportunityContext({ arb }: { arb: SpreadArbitrage }) {
  const netSpreadNum = parseFloat(arb.net_spread_pct);
  const grossSpreadNum = parseFloat(arb.spread_pct);

  const detailRows: {
    label: string;
    buy: React.ReactNode;
    sell: React.ReactNode;
  }[] = [
    {
      label: "Exchange",
      buy: <ExchangeBadge exchangeId={arb.buy_exchange_id} />,
      sell: <ExchangeBadge exchangeId={arb.sell_exchange_id} />,
    },
    {
      label: "Price",
      buy: formatPrice(arb.buy_ask),
      sell: formatPrice(arb.sell_bid),
    },
    {
      label: "Volume 24h",
      buy: formatVolume(arb.buy_volume_24h),
      sell: formatVolume(arb.sell_volume_24h),
    },
    {
      label: "Taker Fee",
      buy: `${arb.fees.buy_taker_fee_pct}%`,
      sell: `${arb.fees.sell_taker_fee_pct}%`,
    },
  ];

  return (
    <div className="space-y-5">
      {/* Spread summary */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Spread Overview
        </div>
        <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
          <span className="text-muted-foreground">Gross spread</span>
          <span
            className={cn(
              "font-mono tabular-nums text-right font-medium",
              grossSpreadNum >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {formatPct(arb.spread_pct)}
          </span>
          <span className="text-muted-foreground">Spread absolute</span>
          <span className="font-mono tabular-nums text-right">
            {formatUsd(arb.spread_abs)}
          </span>
          <span className="text-muted-foreground">Mid price</span>
          <span className="font-mono tabular-nums text-right">
            {formatPrice(arb.mid_price)}
          </span>
          <span className="text-muted-foreground">Net spread</span>
          <span
            className={cn(
              "font-mono tabular-nums text-right font-medium",
              netSpreadNum >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {formatPct(arb.net_spread_pct)}
          </span>
        </div>
      </div>

      <Separator />

      {/* Fee breakdown */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Fees
        </div>
        <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
          <span className="text-muted-foreground">Open</span>
          <span className="font-mono tabular-nums text-right">
            {formatPct(arb.fees.open_fees_pct)}
          </span>
          <span className="text-muted-foreground">Close</span>
          <span className="font-mono tabular-nums text-right">
            {formatPct(arb.fees.close_fees_pct)}
          </span>
          <span className="text-muted-foreground">Round-trip</span>
          <span className="font-mono tabular-nums text-right">
            {formatPct(arb.fees.round_trip_fees_pct)}
          </span>
        </div>
      </div>

      <Separator />

      {/* Leg comparison table */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Leg Comparison
        </div>
        <table className="text-xs w-full">
          <thead>
            <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
              <th className="pr-4 pb-1 text-left font-medium" />
              <th className="pr-4 pb-1 text-right font-medium">
                <span className="inline-flex items-center gap-1">
                  <TrendingUp className="h-3 w-3 text-emerald-500" />
                  Buy
                </span>
              </th>
              <th className="pb-1 text-right font-medium">
                <span className="inline-flex items-center gap-1">
                  <TrendingDown className="h-3 w-3 text-red-500" />
                  Sell
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            {detailRows.map((row) => (
              <tr key={row.label} className="border-t border-border/30">
                <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                  {row.label}
                </td>
                <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {row.buy}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {row.sell}
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
// Main Page
// ---------------------------------------------------------------------------

export default function SpreadControlPanel() {
  const params = useParams<{ slug: string }>();
  const router = useRouter();

  const slug = params.slug;
  const decoded = useMemo(() => decodeSpreadSlug(slug), [slug]);

  const { data: arbitrages, isLoading, error } = useQuery({
    queryKey: ["spread-arbitrage"],
    queryFn: fetchSpreadArbitrage,
  });

  const arb = useMemo(() => {
    if (!arbitrages || !decoded) return null;
    const [symbol, buyEx, sellEx] = decoded;
    return (
      arbitrages.find(
        (a) =>
          a.symbol === symbol &&
          a.buy_exchange_id === buyEx &&
          a.sell_exchange_id === sellEx
      ) ?? null
    );
  }, [arbitrages, decoded]);

  const handleSuccess = useCallback(() => {
    // Navigate to positions page after successful creation
    router.push("/positions");
  }, [router]);

  // Invalid slug
  if (!decoded) {
    return (
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="text-center space-y-3">
          <p className="text-sm text-muted-foreground">Invalid spread arbitrage link.</p>
          <Button variant="outline" size="sm" onClick={() => router.push("/arbitrage")}>
            <ArrowLeft className="h-3 w-3 mr-1" />
            Back to Arbitrage
          </Button>
        </div>
      </div>
    );
  }

  // Loading state
  if (isLoading) {
    return (
      <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
        <div className="border-b px-4 py-3 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-screen-xl flex items-center gap-3">
            <Skeleton className="h-6 w-32" />
            <Skeleton className="h-5 w-20" />
          </div>
        </div>
        <div className="flex-1 min-h-0 overflow-y-auto">
          <div className="mx-auto max-w-screen-xl px-4 py-6 sm:px-6 lg:px-8">
            <div className="grid grid-cols-1 lg:grid-cols-[1fr_380px] gap-6">
              <div className="space-y-4">
                {Array.from({ length: 6 }).map((_, i) => (
                  <Skeleton key={i} className="h-8 w-full" />
                ))}
              </div>
              <div className="space-y-4">
                {Array.from({ length: 4 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="text-center space-y-3">
          <p className="text-sm text-destructive">
            Failed to load spread data: {error.message}
          </p>
          <Button variant="outline" size="sm" onClick={() => router.push("/arbitrage")}>
            <ArrowLeft className="h-3 w-3 mr-1" />
            Back to Arbitrage
          </Button>
        </div>
      </div>
    );
  }

  // Arb not found (data loaded but no match)
  if (!arb) {
    const [symbol, buyEx, sellEx] = decoded;
    return (
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="text-center space-y-3">
          <p className="text-sm text-muted-foreground">
            Spread opportunity not found: {symbol} ({buyEx}/{sellEx})
          </p>
          <p className="text-xs text-muted-foreground">
            The opportunity may no longer be available.
          </p>
          <Button variant="outline" size="sm" onClick={() => router.push("/arbitrage")}>
            <ArrowLeft className="h-3 w-3 mr-1" />
            Back to Arbitrage
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
      {/* Page header */}
      <div className="border-b px-4 py-3 sm:px-6 lg:px-8 shrink-0">
        <div className="mx-auto max-w-screen-xl flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs"
              onClick={() => router.push("/arbitrage")}
            >
              <ArrowLeft className="h-3 w-3 mr-1" />
              Back
            </Button>
            <Separator orientation="vertical" className="h-5" />
            <div>
              <div className="font-medium text-sm">{arb.symbol}</div>
              <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                <span className="inline-flex items-center gap-1">
                  <TrendingUp className="h-3 w-3 text-emerald-500" />
                  <span className="text-[0.5625rem] text-muted-foreground">buy</span>
                  <ExchangeBadge exchangeId={arb.buy_exchange_id} />
                </span>
                <span className="text-muted-foreground/50">/</span>
                <span className="inline-flex items-center gap-1">
                  <TrendingDown className="h-3 w-3 text-red-500" />
                  <span className="text-[0.5625rem] text-muted-foreground">sell</span>
                  <ExchangeBadge exchangeId={arb.sell_exchange_id} />
                </span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Badge
              variant="outline"
              className={cn(
                "font-mono text-xs",
                parseFloat(arb.net_spread_pct) >= 0
                  ? "border-emerald-500/30 text-emerald-600 dark:text-emerald-400"
                  : "border-red-500/30 text-red-600 dark:text-red-400"
              )}
            >
              Net {formatPct(arb.net_spread_pct)}
            </Badge>
            <Badge variant="outline" className="font-mono text-xs text-muted-foreground">
              Spread
            </Badge>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="flex-1 min-h-0 overflow-y-auto">
        <div className="mx-auto max-w-screen-xl px-4 py-6 sm:px-6 lg:px-8">
          <div className="grid grid-cols-1 lg:grid-cols-[1fr_380px] gap-6">
            {/* Left: Opportunity context */}
            <div>
              <OpportunityContext arb={arb} />
            </div>

            {/* Right: Position form */}
            <div>
              <div className="lg:sticky lg:top-6">
                <div className="rounded-lg border bg-card p-4">
                  <div className="flex items-center gap-2 mb-4">
                    <h2 className="text-sm font-medium">Open Spread Position</h2>
                  </div>
                  <PositionForm arb={arb} onSuccess={handleSuccess} />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
