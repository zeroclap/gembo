"use client";

import { useState, useCallback, useMemo } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  createPosition,
  type CreatePositionRequest,
  type SizeType,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { SectionHeader, LegLabel } from "@/components/monitor/shared";
import { SimulationDialog } from "@/components/monitor/simulation-dialog";
import { cn } from "@/lib/utils";
import { Loader2, FlaskConical } from "lucide-react";

// ---------------------------------------------------------------------------
// Duration presets
// ---------------------------------------------------------------------------

const DURATION_PRESETS = [
  { label: "1h", value: 3600 },
  { label: "4h", value: 14400 },
  { label: "12h", value: 43200 },
  { label: "24h", value: 86400 },
  { label: "48h", value: 172800 },
  { label: "7d", value: 604800 },
  { label: "None", value: 0 },
] as const;

// ---------------------------------------------------------------------------
// OpenPositionPanel — inline sidebar form
// ---------------------------------------------------------------------------

interface OpenPositionPanelProps {
  symbol: string;
  longExchange: string;
  shortExchange: string;
}

export function OpenPositionPanel({
  symbol,
  longExchange,
  shortExchange,
}: OpenPositionPanelProps) {
  const queryClient = useQueryClient();

  // --- Form state ---
  const [sizeType, setSizeType] = useState<SizeType>("usd");
  const [size, setSize] = useState("1000");
  const [portionSize, setPortionSize] = useState("");
  const [priceMin, setPriceMin] = useState("");
  const [priceMax, setPriceMax] = useState("");
  const [slippagePct, setSlippagePct] = useState("");
  const [leverage, setLeverage] = useState("");
  const [marginCapital, setMarginCapital] = useState("");

  // --- Spread parameters (unified — always shown) ---
  const [targetSpreadPct, setTargetSpreadPct] = useState("0");
  const [stopLossSpreadPct, setStopLossSpreadPct] = useState("");
  const [maxHoldDuration, setMaxHoldDuration] = useState(0);

  // --- Simulation dialog ---
  const [simOpen, setSimOpen] = useState(false);

  const isStepped =
    portionSize.trim() !== "" && parseFloat(portionSize) > 0;
  const estimatedSteps = useMemo(() => {
    const total = parseFloat(size);
    const portion = parseFloat(portionSize);
    if (!total || !portion || total <= 0 || portion <= 0) return null;
    return Math.ceil(total / portion);
  }, [size, portionSize]);

  const sizeValid = parseFloat(size) > 0;

  // --- Create position mutation ---
  const createMutation = useMutation({
    mutationFn: createPosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
    },
  });

  const handleSubmit = useCallback(() => {
    const req: CreatePositionRequest = {
      symbol,
      long_exchange_id: longExchange,
      short_exchange_id: shortExchange,
      size_type: sizeType,
      requested_size: size,
    };

    if (portionSize.trim()) req.portion_size = portionSize;
    if (priceMin.trim()) req.price_min = priceMin;
    if (priceMax.trim()) req.price_max = priceMax;
    if (slippagePct.trim()) req.slippage_pct = slippagePct;
    const lev = parseInt(leverage, 10);
    if (!isNaN(lev) && lev > 0) req.leverage = lev;
    if (marginCapital.trim()) req.margin_capital = marginCapital;

    // Spread params (always available)
    if (targetSpreadPct.trim() && targetSpreadPct !== "0")
      req.target_spread_pct = targetSpreadPct;
    if (stopLossSpreadPct.trim())
      req.stop_loss_spread_pct = stopLossSpreadPct;
    if (maxHoldDuration > 0) req.max_hold_duration = maxHoldDuration;

    createMutation.mutate(req);
  }, [
    symbol,
    longExchange,
    shortExchange,
    sizeType,
    size,
    portionSize,
    priceMin,
    priceMax,
    slippagePct,
    leverage,
    marginCapital,
    targetSpreadPct,
    stopLossSpreadPct,
    maxHoldDuration,
    createMutation,
  ]);

  return (
    <>
      <SimulationDialog
        open={simOpen}
        onOpenChange={setSimOpen}
        symbol={symbol}
        longExchange={longExchange}
        shortExchange={shortExchange}
        sizeType={sizeType}
        size={size}
        slippagePct={slippagePct}
      />

      <div className="rounded-lg border bg-card/50 overflow-hidden">
        {/* Header */}
        <div className="border-b px-3 py-2.5">
          <SectionHeader>Open Position</SectionHeader>
          <div className="flex items-center gap-2 mt-1.5">
            <LegLabel side="long" exchangeId={longExchange} />
            <span className="text-[0.625rem] text-muted-foreground">/</span>
            <LegLabel side="short" exchangeId={shortExchange} />
          </div>
        </div>

        <div className="p-3 space-y-3">
          {/* Size type toggle */}
          <div>
            <Label className="text-xs text-muted-foreground">Size</Label>
            <div className="flex gap-1 mt-1 mb-1.5">
              {(["usd", "base"] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setSizeType(t)}
                  className={cn(
                    "px-2.5 py-0.5 text-[0.625rem] font-medium rounded-md transition-colors",
                    sizeType === t
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted text-muted-foreground hover:text-foreground"
                  )}
                >
                  {t === "usd" ? "USD" : "Base"}
                </button>
              ))}
            </div>
            <Input
              type="text"
              inputMode="decimal"
              value={size}
              onChange={(e) => setSize(e.target.value)}
              placeholder={sizeType === "usd" ? "1000" : "0.1"}
              className="font-mono tabular-nums text-xs h-7"
            />
          </div>

          {/* Leverage & Margin */}
          <div className="grid grid-cols-2 gap-2">
            <div>
              <Label className="text-xs text-muted-foreground">Leverage</Label>
              <Input
                type="text"
                inputMode="numeric"
                value={leverage}
                onChange={(e) => setLeverage(e.target.value)}
                placeholder="Auto"
                className="font-mono tabular-nums text-xs h-7 mt-1"
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">Margin</Label>
              <Input
                type="text"
                inputMode="decimal"
                value={marginCapital}
                onChange={(e) => setMarginCapital(e.target.value)}
                placeholder="Auto"
                className="font-mono tabular-nums text-xs h-7 mt-1"
              />
            </div>
          </div>

          <Separator />

          {/* Spread parameters */}
          <div>
            <SectionHeader className="mb-1.5">Spread</SectionHeader>
            <div className="space-y-2">
              <div>
                <Label className="text-xs text-muted-foreground">
                  Target %
                </Label>
                <Input
                  type="text"
                  inputMode="decimal"
                  value={targetSpreadPct}
                  onChange={(e) => setTargetSpreadPct(e.target.value)}
                  placeholder="0 = manual close"
                  className="font-mono tabular-nums text-xs h-7 mt-0.5"
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground">
                  Stop Loss %
                </Label>
                <Input
                  type="text"
                  inputMode="decimal"
                  value={stopLossSpreadPct}
                  onChange={(e) => setStopLossSpreadPct(e.target.value)}
                  placeholder="None"
                  className="font-mono tabular-nums text-xs h-7 mt-0.5"
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground">
                  Max Hold
                </Label>
                <div className="flex flex-wrap gap-1 mt-1">
                  {DURATION_PRESETS.map((preset) => (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() => setMaxHoldDuration(preset.value)}
                      className={cn(
                        "px-1.5 py-0.5 text-[0.625rem] font-medium rounded-md transition-colors",
                        maxHoldDuration === preset.value
                          ? "bg-primary text-primary-foreground"
                          : "bg-muted text-muted-foreground hover:text-foreground"
                      )}
                    >
                      {preset.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>

          <Separator />

          {/* Stepped Execution */}
          <div>
            <SectionHeader className="mb-1.5">
              Stepped Execution
              {isStepped && estimatedSteps && (
                <span className="ml-1 text-violet-600 dark:text-violet-400">
                  ({estimatedSteps} steps)
                </span>
              )}
            </SectionHeader>
            <div className="space-y-2">
              <div>
                <Label className="text-xs text-muted-foreground">
                  Portion Size
                </Label>
                <Input
                  type="text"
                  inputMode="decimal"
                  value={portionSize}
                  onChange={(e) => setPortionSize(e.target.value)}
                  placeholder="Empty = single-shot"
                  className="font-mono tabular-nums text-xs h-7 mt-0.5"
                />
              </div>
              {isStepped && (
                <>
                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <Label className="text-xs text-muted-foreground">
                        Price Min
                      </Label>
                      <Input
                        type="text"
                        inputMode="decimal"
                        value={priceMin}
                        onChange={(e) => setPriceMin(e.target.value)}
                        placeholder="No limit"
                        className="font-mono tabular-nums text-xs h-7 mt-0.5"
                      />
                    </div>
                    <div>
                      <Label className="text-xs text-muted-foreground">
                        Price Max
                      </Label>
                      <Input
                        type="text"
                        inputMode="decimal"
                        value={priceMax}
                        onChange={(e) => setPriceMax(e.target.value)}
                        placeholder="No limit"
                        className="font-mono tabular-nums text-xs h-7 mt-0.5"
                      />
                    </div>
                  </div>
                  <div>
                    <Label className="text-xs text-muted-foreground">
                      Max Slippage %
                    </Label>
                    <Input
                      type="text"
                      inputMode="decimal"
                      value={slippagePct}
                      onChange={(e) => setSlippagePct(e.target.value)}
                      placeholder="No limit"
                      className="font-mono tabular-nums text-xs h-7 mt-0.5"
                    />
                  </div>
                </>
              )}
            </div>
          </div>

          {/* Execution preview (shown when stepped) */}
          {isStepped && estimatedSteps && (
            <>
              <Separator />
              <div className="rounded-md border bg-muted/30 px-2.5 py-2">
                <SectionHeader className="mb-1">Preview</SectionHeader>
                <div className="grid grid-cols-2 gap-x-3 gap-y-0.5 text-[0.625rem]">
                  <span className="text-muted-foreground">Total</span>
                  <span className="font-mono tabular-nums text-right">
                    {sizeType === "usd" ? "$" : ""}
                    {size}
                  </span>
                  <span className="text-muted-foreground">Portion</span>
                  <span className="font-mono tabular-nums text-right">
                    {sizeType === "usd" ? "$" : ""}
                    {portionSize}
                  </span>
                  <span className="text-muted-foreground">Steps</span>
                  <span className="font-mono tabular-nums text-right">
                    {estimatedSteps}
                  </span>
                </div>
              </div>
            </>
          )}

          {/* Error */}
          {createMutation.error && (
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-2.5 py-1.5 text-[0.625rem] text-destructive">
              {createMutation.error.message}
            </div>
          )}

          {/* Success */}
          {createMutation.isSuccess && (
            <div className="rounded-md bg-emerald-500/10 border border-emerald-500/20 px-2.5 py-1.5 text-[0.625rem] text-emerald-600 dark:text-emerald-400">
              Position created successfully
            </div>
          )}

          <Separator />

          {/* Actions */}
          <div className="flex flex-col gap-1.5">
            <Button
              size="sm"
              className="h-7 text-xs w-full"
              disabled={!sizeValid || createMutation.isPending}
              onClick={handleSubmit}
            >
              {createMutation.isPending && (
                <Loader2 className="h-3 w-3 animate-spin mr-1" />
              )}
              {isStepped && estimatedSteps
                ? `Open Position (${estimatedSteps} steps)`
                : "Open Position"}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs w-full"
              disabled={!sizeValid}
              onClick={() => setSimOpen(true)}
            >
              <FlaskConical className="h-3 w-3 mr-1" />
              Simulate
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}
