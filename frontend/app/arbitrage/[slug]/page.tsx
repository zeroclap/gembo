"use client";

import { useParams, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useMemo, useCallback } from "react";
import {
  fetchFundingArbitrage,
  createPosition,
  type FundingArbitrage,
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
  ArrowUp,
  ArrowDown,
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

/** Encode arb key "SYMBOL::LONG::SHORT" -> URL-safe slug. */
function encodeArbSlug(symbol: string, longEx: string, shortEx: string): string {
  return `${encodeURIComponent(symbol)}--${encodeURIComponent(longEx)}--${encodeURIComponent(shortEx)}`;
}

/** Decode slug back to [symbol, longExchange, shortExchange]. */
function decodeArbSlug(slug: string): [string, string, string] | null {
  const parts = slug.split("--");
  if (parts.length !== 3) return null;
  return [
    decodeURIComponent(parts[0]),
    decodeURIComponent(parts[1]),
    decodeURIComponent(parts[2]),
  ];
}

// Export for use in navigation
export { encodeArbSlug };

// ---------------------------------------------------------------------------
// Position Form
// ---------------------------------------------------------------------------

function PositionForm({
  arb,
  onSuccess,
}: {
  arb: FundingArbitrage;
  onSuccess: () => void;
}) {
  const queryClient = useQueryClient();

  // Basic fields
  const [sizeType, setSizeType] = useState<SizeType>("usd");
  const [size, setSize] = useState("1000");

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
      long_exchange_id: arb.long.exchange_id,
      short_exchange_id: arb.short.exchange_id,
      size_type: sizeType,
      requested_size: size,
    };

    // Only include stepped fields if they have meaningful values.
    if (portionSize.trim()) req.portion_size = portionSize;
    if (priceMin.trim()) req.price_min = priceMin;
    if (priceMax.trim()) req.price_max = priceMax;
    if (slippagePct.trim()) req.slippage_pct = slippagePct;
    const lev = parseInt(leverage, 10);
    if (!isNaN(lev) && lev > 0) req.leverage = lev;
    if (marginCapital.trim()) req.margin_capital = marginCapital;

    mutation.mutate(req);
  }, [arb, sizeType, size, portionSize, priceMin, priceMax, slippagePct, leverage, marginCapital, mutation]);

  // Compute estimated steps for preview
  const estimatedSteps = useMemo(() => {
    const total = parseFloat(size);
    const portion = parseFloat(portionSize);
    if (!total || !portion || total <= 0 || portion <= 0) return null;
    return Math.ceil(total / portion);
  }, [size, portionSize]);

  return (
    <div className="space-y-5">
      {/* Section: Basic */}
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
          ? `Open Position (${estimatedSteps} steps)`
          : "Open Position"}
      </Button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Left panel — Opportunity Context
// ---------------------------------------------------------------------------

function OpportunityContext({ arb }: { arb: FundingArbitrage }) {
  const lcmVal = parseFloat(arb.full_loop.net_pct_per_loop) * 100;
  const nextVal = parseFloat(arb.next_payment.payment_pct) * 100;

  const detailRows: {
    label: string;
    long: React.ReactNode;
    short: React.ReactNode;
  }[] = [
    {
      label: "Exchange",
      long: <ExchangeBadge exchangeId={arb.long.exchange_id} />,
      short: <ExchangeBadge exchangeId={arb.short.exchange_id} />,
    },
    {
      label: "Price",
      long: formatPrice(arb.long.price),
      short: formatPrice(arb.short.price),
    },
    {
      label: "Bid / Ask",
      long: `${formatPrice(arb.long.bid)} / ${formatPrice(arb.long.ask)}`,
      short: `${formatPrice(arb.short.bid)} / ${formatPrice(arb.short.ask)}`,
    },
    {
      label: "Mark",
      long: formatPrice(arb.long.mark_price),
      short: formatPrice(arb.short.mark_price),
    },
    {
      label: "Index",
      long: formatPrice(arb.long.index_price),
      short: formatPrice(arb.short.index_price),
    },
    {
      label: "Volume 24h",
      long: formatVolume(arb.long.volume_24h),
      short: formatVolume(arb.short.volume_24h),
    },
    {
      label: "Funding Rate",
      long: (
        <span
          className={cn(
            "font-mono",
            parseFloat(arb.long.funding_rate) >= 0
              ? "text-emerald-600 dark:text-emerald-400"
              : "text-red-600 dark:text-red-400"
          )}
        >
          {formatRate(arb.long.funding_rate)}
        </span>
      ),
      short: (
        <span
          className={cn(
            "font-mono",
            parseFloat(arb.short.funding_rate) >= 0
              ? "text-emerald-600 dark:text-emerald-400"
              : "text-red-600 dark:text-red-400"
          )}
        >
          {formatRate(arb.short.funding_rate)}
        </span>
      ),
    },
    {
      label: "Rate 8h",
      long: formatRate(arb.long.rate_8h),
      short: formatRate(arb.short.rate_8h),
    },
    {
      label: "Taker Fee",
      long: `${arb.long.taker_fee_pct}%`,
      short: `${arb.short.taker_fee_pct}%`,
    },
    {
      label: "Interval",
      long: `${arb.long.interval_hours}h`,
      short: `${arb.short.interval_hours}h`,
    },
    {
      label: "Contract Size",
      long: arb.long.contract_size,
      short: arb.short.contract_size,
    },
  ];

  return (
    <div className="space-y-5">
      {/* Funding spread summary */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Funding Spread
        </div>
        <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
          <span className="text-muted-foreground">Next payment</span>
          <span
            className={cn(
              "font-mono tabular-nums text-right font-medium",
              nextVal >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {nextVal >= 0 ? "+" : ""}{nextVal.toFixed(4)}%
          </span>
          <span className="text-muted-foreground">LCM loop ({arb.full_loop.duration_hours}h)</span>
          <span
            className={cn(
              "font-mono tabular-nums text-right font-medium",
              lcmVal >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {lcmVal >= 0 ? "+" : ""}{lcmVal.toFixed(4)}%
          </span>
          <span className="text-muted-foreground">Open spread</span>
          <span className="font-mono tabular-nums text-right">
            {formatPct(arb.open_spread)}
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
          <span className="text-muted-foreground">Net APR</span>
          <span
            className={cn(
              "font-mono tabular-nums text-right font-medium",
              parseFloat(arb.fees.net_apr_after_fees) >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {formatAPR(arb.fees.net_apr_after_fees)}
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
            {detailRows.map((row) => (
              <tr key={row.label} className="border-t border-border/30">
                <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                  {row.label}
                </td>
                <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {row.long}
                </td>
                <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {row.short}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Separator />

      {/* Full loop payments */}
      <div className="space-y-3">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Full Loop ({arb.full_loop.duration_hours}h) &mdash;{" "}
          {arb.full_loop.payments.length} payments
        </div>
        <table className="text-xs w-full">
          <thead>
            <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
              <th className="pr-3 pb-1 text-left font-medium">Hour</th>
              <th className="pr-3 pb-1 text-left font-medium">Exchange</th>
              <th className="pr-3 pb-1 text-left font-medium">Side</th>
              <th className="pr-3 pb-1 text-right font-medium">Rate</th>
              <th className="pb-1 text-right font-medium">Cum.</th>
            </tr>
          </thead>
          <tbody>
            {arb.full_loop.payments.map((p, i) => {
              const cumVal = parseFloat(p.cumulative) * 100;
              return (
                <tr key={i} className="border-t border-border/30">
                  <td className="pr-3 py-0.5 font-mono tabular-nums text-muted-foreground">
                    {p.at_hour}h
                  </td>
                  <td className="pr-3 py-0.5">
                    <ExchangeBadge exchangeId={p.exchange_id} />
                  </td>
                  <td className="pr-3 py-0.5">
                    <span
                      className={cn(
                        "text-[0.625rem] font-medium uppercase",
                        p.side === "long"
                          ? "text-emerald-600 dark:text-emerald-400"
                          : "text-red-600 dark:text-red-400"
                      )}
                    >
                      {p.side}
                    </span>
                  </td>
                  <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                    {formatRate(p.rate)}
                  </td>
                  <td
                    className={cn(
                      "py-0.5 text-right font-mono tabular-nums font-medium",
                      cumVal >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {cumVal >= 0 ? "+" : ""}{cumVal.toFixed(4)}%
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function ArbitrageControlPanel() {
  const params = useParams<{ slug: string }>();
  const router = useRouter();

  const slug = params.slug;
  const decoded = useMemo(() => decodeArbSlug(slug), [slug]);

  const { data: arbitrages, isLoading, error } = useQuery({
    queryKey: ["funding-arbitrage"],
    queryFn: fetchFundingArbitrage,
  });

  const arb = useMemo(() => {
    if (!arbitrages || !decoded) return null;
    const [symbol, longEx, shortEx] = decoded;
    return (
      arbitrages.find(
        (a) =>
          a.symbol === symbol &&
          a.long.exchange_id === longEx &&
          a.short.exchange_id === shortEx
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
          <p className="text-sm text-muted-foreground">Invalid arbitrage link.</p>
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
            Failed to load arbitrage data: {error.message}
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
    const [symbol, longEx, shortEx] = decoded;
    return (
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="text-center space-y-3">
          <p className="text-sm text-muted-foreground">
            Arbitrage opportunity not found: {symbol} ({longEx}/{shortEx})
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
                  <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                    <ArrowUp className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
                  </span>
                  <ExchangeBadge exchangeId={arb.long.exchange_id} />
                </span>
                <span className="text-muted-foreground/50">/</span>
                <span className="inline-flex items-center gap-1">
                  <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                    <ArrowDown className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
                  </span>
                  <ExchangeBadge exchangeId={arb.short.exchange_id} />
                </span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Badge
              variant="outline"
              className={cn(
                "font-mono text-xs",
                parseFloat(arb.fees.net_apr_after_fees) >= 0
                  ? "border-emerald-500/30 text-emerald-600 dark:text-emerald-400"
                  : "border-red-500/30 text-red-600 dark:text-red-400"
              )}
            >
              APR {formatAPR(arb.fees.net_apr_after_fees)}
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
                    <h2 className="text-sm font-medium">Open Position</h2>
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
