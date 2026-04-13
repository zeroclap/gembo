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
import { SectionHeader } from "@/components/arbitrage/shared";
import { SimulationPanelView } from "@/components/arbitrage/simulation-dialog";
import { cn } from "@/lib/utils";
import { Loader2, FlaskConical, ArrowLeft } from "lucide-react";

// ---------------------------------------------------------------------------
// Constants
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

type Strategy = "price_spread" | "funding_rate";

// ---------------------------------------------------------------------------
// OpenPositionPanel — inline sidebar form
// ---------------------------------------------------------------------------

interface OpenPositionPanelProps {
  symbol: string;
  longExchange: string;
  shortExchange: string;
  /** When true, renders without the outer rounded border (for embedding in a sidebar). */
  embedded?: boolean;
}

export function OpenPositionPanel({
  symbol,
  longExchange,
  shortExchange,
  embedded = false,
}: OpenPositionPanelProps) {
  const queryClient = useQueryClient();

  // --- Size ---
  // sizeType "usd" → position_size (denominated in quote, e.g. USDT)
  // sizeType "base" → quantity (number of contracts)
  const [sizeType, setSizeType] = useState<SizeType>("usd");
  const [size, setSize] = useState("1000");
  const [quantity, setQuantity] = useState("1");

  // --- Stepped execution ---
  const [portionSize, setPortionSize] = useState("");
  const [slippagePct, setSlippagePct] = useState("");
  const [slippageEnabled, setSlippageEnabled] = useState(false);

  // --- Leverage: per-exchange, 1–3 only ---
  const [longLeverage, setLongLeverage] = useState(1);
  const [shortLeverage, setShortLeverage] = useState(1);

  // --- Strategy ---
  const [strategy, setStrategy] = useState<Strategy>("price_spread");

  // Price-spread strategy fields
  const [targetSpreadPct, setTargetSpreadPct] = useState("");
  const [stopLossSpreadPct, setStopLossSpreadPct] = useState("");
  const [maxHoldDuration, setMaxHoldDuration] = useState(0);

  // --- View toggle ---
  const [view, setView] = useState<"form" | "simulation">("form");

  // --- Derived ---
  const isStepped =
    portionSize.trim() !== "" && parseFloat(portionSize) > 0;

  const activeSize = sizeType === "usd" ? size : quantity;

  const estimatedSteps = useMemo(() => {
    if (!isStepped) return null;
    const total = parseFloat(activeSize);
    const portion = parseFloat(portionSize);
    if (!total || !portion || total <= 0 || portion <= 0) return null;
    return Math.ceil(total / portion);
  }, [isStepped, activeSize, portionSize]);

  const sizeValid = parseFloat(activeSize) > 0;

  // Apply portion as percentage of total size
  function applyPortionPct(pct: number) {
    const total = parseFloat(activeSize);
    if (!total || total <= 0) return;
    setPortionSize(String(Math.floor((total * pct) / 100)));
  }

  // --- Mutation ---
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
      requested_size: sizeType === "usd" ? size : quantity,
    };

    if (portionSize.trim()) req.portion_size = portionSize;
    if (slippageEnabled && slippagePct.trim()) req.slippage_pct = slippagePct;

    if (longLeverage > 1) req.leverage = longLeverage;

    if (strategy === "price_spread") {
      req.arbitrage_type = "spread";
      if (targetSpreadPct.trim()) req.target_spread_pct = targetSpreadPct;
      if (stopLossSpreadPct.trim()) req.stop_loss_spread_pct = stopLossSpreadPct;
      if (maxHoldDuration > 0) req.max_hold_duration = maxHoldDuration;
    } else {
      // funding_rate: hold as long as profitable — no target spread
      req.arbitrage_type = "funding";
      if (maxHoldDuration > 0) req.max_hold_duration = maxHoldDuration;
    }

    createMutation.mutate(req);
  }, [
    symbol,
    longExchange,
    shortExchange,
    sizeType,
    size,
    quantity,
    portionSize,
    slippageEnabled,
    slippagePct,
    longLeverage,
    strategy,
    targetSpreadPct,
    stopLossSpreadPct,
    maxHoldDuration,
    createMutation,
  ]);

  const simProps = {
    symbol,
    longExchange,
    shortExchange,
    sizeType: sizeType === "usd" ? ("usd" as const) : ("qty" as const),
    size: sizeType === "usd" ? size : quantity,
    slippagePct: slippageEnabled ? slippagePct : "",
    autoRun: true as const,
  };

  return (
    <div
      className={cn(
        "flex flex-col h-full",
        embedded
          ? "overflow-hidden"
          : "rounded-lg border bg-card/50 overflow-hidden"
      )}
    >
      {/* Header */}
      <div className="border-b px-3 py-2.5 shrink-0">
        <div className="flex items-center gap-2">
          {/* Back button — shown when in simulation view */}
          {view === "simulation" && (
            <button
              type="button"
              onClick={() => setView("form")}
              className="flex items-center justify-center h-5 w-5 rounded-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors shrink-0"
              title="Back to form"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
            </button>
          )}
          <SectionHeader>
            {view === "simulation" ? "Simulation" : "Open Position"}
          </SectionHeader>
        </div>
      </div>

      {/* Body — simulation view or form */}
      {view === "simulation" ? (
        <div className="flex flex-col flex-1 min-h-0 overflow-hidden">
          <SimulationPanelView {...simProps} />
        </div>
      ) : null}

      {/* Form — shown only when view === "form" */}
      <div
        className={cn(
          "p-3 space-y-3 overflow-y-auto flex-1",
          view === "simulation" ? "hidden" : undefined
        )}
      >
          {/* ── Size ── */}
          <div>
            {/* USD / QTY toggle */}
            <div className="flex items-center justify-between mb-1.5">
              <Label className="text-xs text-muted-foreground">
                {sizeType === "usd" ? "Price (USDT)" : "Contracts (QTY)"}
              </Label>
              <div className="flex h-6 items-center rounded-md border bg-muted/40 p-0.5">
                {(["usd", "base"] as const).map((t) => (
                  <button
                    key={t}
                    type="button"
                    onClick={() => setSizeType(t)}
                    className={cn(
                      "rounded-sm px-2 text-[0.625rem] font-medium transition-colors h-5",
                      sizeType === t
                        ? "bg-background text-foreground shadow-sm border border-border/50"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    {t === "usd" ? "Price" : "QTY"}
                  </button>
                ))}
              </div>
            </div>
            <Input
              type="text"
              inputMode="decimal"
              value={sizeType === "usd" ? size : quantity}
              onChange={(e) =>
                sizeType === "usd"
                  ? setSize(e.target.value)
                  : setQuantity(e.target.value)
              }
              placeholder={sizeType === "usd" ? "1000" : "1"}
              className="font-mono tabular-nums text-xs h-7"
            />
          </div>

          {/* ── Slippage ── */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="flex items-center gap-1.5 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={slippageEnabled}
                  onChange={(e) => setSlippageEnabled(e.target.checked)}
                  className="h-3 w-3 rounded-sm accent-primary"
                />
                <span className="text-xs text-muted-foreground">Max Slippage %</span>
              </label>
            </div>
            <Input
              type="text"
              inputMode="decimal"
              value={slippagePct}
              onChange={(e) => setSlippagePct(e.target.value)}
              placeholder="No limit"
              disabled={!slippageEnabled}
              className={cn(
                "font-mono tabular-nums text-xs h-7",
                !slippageEnabled && "opacity-40"
              )}
            />
            {slippageEnabled && (
              <div className="flex gap-1 mt-1.5">
                {["0.1", "0.3", "0.5", "1"].map((v) => (
                  <button
                    key={v}
                    type="button"
                    onClick={() => setSlippagePct(v)}
                    className={cn(
                      "flex-1 rounded-md py-0.5 text-[0.625rem] font-medium transition-colors",
                      slippagePct === v
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground"
                    )}
                  >
                    {v}%
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* ── Strategy ── */}
          <div>
            <Label className="text-xs text-muted-foreground">Strategy</Label>
            <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5 mt-1">
              {(["price_spread", "funding_rate"] as const).map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => setStrategy(s)}
                  className={cn(
                    "flex-1 rounded-sm px-2 h-5 text-[0.625rem] font-medium transition-colors",
                    strategy === s
                      ? "bg-background text-foreground shadow-sm border border-border/50"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  {s === "price_spread" ? "Price Spread" : "Funding Rate"}
                </button>
              ))}
            </div>
          </div>

          {/* Strategy-specific fields */}
          {strategy === "price_spread" ? (
            <div className="space-y-2">
              <div>
                <Label className="text-xs text-muted-foreground">
                  Target Spread %
                  <span className="ml-1 text-[0.5625rem] text-muted-foreground/60">
                    (close when spread reaches)
                  </span>
                </Label>
                <Input
                  type="text"
                  inputMode="decimal"
                  value={targetSpreadPct}
                  onChange={(e) => setTargetSpreadPct(e.target.value)}
                  placeholder="e.g. 1.0"
                  className="font-mono tabular-nums text-xs h-7 mt-0.5"
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground">
                  Stop Loss Spread %
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
                <div className="grid grid-cols-7 gap-1 mt-1">
                  {DURATION_PRESETS.map((preset) => (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() => setMaxHoldDuration(preset.value)}
                      className={cn(
                        "px-1 py-0.5 text-[0.625rem] font-medium rounded-md transition-colors",
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
          ) : (
            // Funding rate: hold as long as profitable; optional max duration
            <div>
              <Label className="text-xs text-muted-foreground">
                Max Hold
                <span className="ml-1 text-[0.5625rem] text-muted-foreground/60">
                  (hold while funding profit)
                </span>
              </Label>
              <div className="grid grid-cols-7 gap-1 mt-1">
                {DURATION_PRESETS.map((preset) => (
                  <button
                    key={preset.value}
                    type="button"
                    onClick={() => setMaxHoldDuration(preset.value)}
                    className={cn(
                      "px-1 py-0.5 text-[0.625rem] font-medium rounded-md transition-colors",
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
          )}

          <Separator />

          {/* ── Leverage per exchange ── */}
          <div>
            <SectionHeader className="mb-1.5">Leverage</SectionHeader>
            <div className="grid grid-cols-2 gap-2">
              {/* Long exchange */}
              <div>
                <Label className="text-[0.625rem] text-muted-foreground truncate block mb-1">
                  {longExchange || "Long"}
                </Label>
                <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5">
                  {[1, 2, 3].map((lv) => (
                    <button
                      key={lv}
                      type="button"
                      onClick={() => setLongLeverage(lv)}
                      className={cn(
                        "flex-1 rounded-sm h-5 text-[0.625rem] font-medium transition-colors",
                        longLeverage === lv
                          ? "bg-background text-foreground shadow-sm border border-border/50"
                          : "text-muted-foreground hover:text-foreground"
                      )}
                    >
                      {lv}x
                    </button>
                  ))}
                </div>
              </div>
              {/* Short exchange */}
              <div>
                <Label className="text-[0.625rem] text-muted-foreground truncate block mb-1">
                  {shortExchange || "Short"}
                </Label>
                <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5">
                  {[1, 2, 3].map((lv) => (
                    <button
                      key={lv}
                      type="button"
                      onClick={() => setShortLeverage(lv)}
                      className={cn(
                        "flex-1 rounded-sm h-5 text-[0.625rem] font-medium transition-colors",
                        shortLeverage === lv
                          ? "bg-background text-foreground shadow-sm border border-border/50"
                          : "text-muted-foreground hover:text-foreground"
                      )}
                    >
                      {lv}x
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>

          <Separator />

          {/* ── Stepped Execution ── */}
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
                {/* Quick-fill: 25 / 50 / 75 / 100 % of total */}
                <div className="flex gap-1 mt-1.5">
                  {[25, 50, 75, 100].map((pct) => (
                    <button
                      key={pct}
                      type="button"
                      onClick={() => applyPortionPct(pct)}
                      disabled={!sizeValid}
                      className="flex-1 rounded-md bg-muted/60 py-0.5 text-[0.625rem] font-medium text-muted-foreground hover:bg-muted hover:text-foreground transition-colors disabled:opacity-40 disabled:pointer-events-none"
                    >
                      {pct}%
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Execution preview */}
          {isStepped && estimatedSteps && (
            <>
              <Separator />
              <div className="rounded-md border bg-muted/30 px-2.5 py-2">
                <SectionHeader className="mb-1">Preview</SectionHeader>
                <div className="grid grid-cols-2 gap-x-3 gap-y-0.5 text-[0.625rem]">
                  <span className="text-muted-foreground">Total</span>
                  <span className="font-mono tabular-nums text-right">
                    {sizeType === "usd" ? "$" : ""}
                    {activeSize}
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

          {/* Error / Success */}
          {createMutation.error && (
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-2.5 py-1.5 text-[0.625rem] text-destructive">
              {createMutation.error.message}
            </div>
          )}
          {createMutation.isSuccess && (
            <div className="rounded-md bg-emerald-500/10 border border-emerald-500/20 px-2.5 py-1.5 text-[0.625rem] text-emerald-600 dark:text-emerald-400">
              Position created successfully
            </div>
          )}

          <Separator />

          {/* ── Actions: Open Position (wide) + Simulate icon (narrow) ── */}
          <div className="flex gap-1.5">
            <Button
              size="sm"
              className="h-7 text-xs flex-1"
              disabled={!sizeValid || createMutation.isPending}
              onClick={handleSubmit}
            >
              {createMutation.isPending && (
                <Loader2 className="h-3 w-3 animate-spin mr-1" />
              )}
              {isStepped && estimatedSteps
                ? `Open (${estimatedSteps} steps)`
                : "Open Position"}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="h-7 w-7 shrink-0 p-0"
              title="Simulate"
              onClick={() => setView("simulation")}
            >
              <FlaskConical className="h-3.5 w-3.5" />
            </Button>
          </div>
      </div>
    </div>
  );
}
