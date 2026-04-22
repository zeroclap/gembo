"use client";

import { useState, useCallback, useMemo, useEffect, useRef } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  createPosition,
  fetchExchangeCredentials,
  preflightOpenPosition,
  simulateArbitrage,
  type CreatePositionRequest,
  type ExchangeCredential,
  type OpenPreflight,
  type OpenPreflightIssue,
  type OpenPreflightLeg,
  type SizeType,
  type SimulationResult,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { SectionHeader } from "@/components/arbitrage/shared";
import { SimulationPanelView } from "@/components/arbitrage/simulation-dialog";
import { formatUsd, formatUsdPlain, formatPct, formatCountdown, formatQty, formatPrice } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Loader2, FlaskConical, ArrowLeft, CheckCircle2, AlertCircle, AlertTriangle, ChevronDown, ChevronRight } from "lucide-react";

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

  const [credentials, setCredentials] = useState<ExchangeCredential[]>([]);
  const [credentialsLoading, setCredentialsLoading] = useState(true);
  const [credentialsError, setCredentialsError] = useState<string | null>(null);

  // --- Size ---
  // sizeType "price" → position_size (denominated in quote, e.g. USDT)
  // sizeType "quantity" → base quantity (underlying asset amount)
  const [sizeType, setSizeType] = useState<SizeType>("quantity");
  const [size, setSize] = useState("1000");
  const [quantity, setQuantity] = useState("1000");

  // --- Stepped execution ---
  const [portionEnabled, setPortionEnabled] = useState(false);
  const [portionSize, setPortionSize] = useState("");
  const [slippagePct, setSlippagePct] = useState("");
  const [slippageEnabled, setSlippageEnabled] = useState(false);

  // --- Leverage: per-exchange, 1–3 only ---
  const [longLeverage, setLongLeverage] = useState(1);
  const [shortLeverage, setShortLeverage] = useState(1);

  // --- Margin mode: per-exchange ---
  const [longMarginMode, setLongMarginMode] = useState<"isolated" | "cross">("isolated");
  const [shortMarginMode, setShortMarginMode] = useState<"isolated" | "cross">("isolated");

  // --- Margin capital (optional) ---
  const [marginCapitalEnabled, setMarginCapitalEnabled] = useState(false);
  const [marginCapital, setMarginCapital] = useState("");

  const [longCredentialID, setLongCredentialID] = useState("");
  const [shortCredentialID, setShortCredentialID] = useState("");

  // --- Strategy: derived from priceSpreadEnabled ---
  const [priceSpreadEnabled, setPriceSpreadEnabled] = useState(false);
  const strategy: Strategy = priceSpreadEnabled ? "price_spread" : "funding_rate";

  // Price-spread strategy fields
  const [targetSpreadPct, setTargetSpreadPct] = useState("");
  const [stopLossSpreadPct, setStopLossSpreadPct] = useState("");

  // --- Max Hold (optional, shared across strategies) ---
  const [maxHoldEnabled, setMaxHoldEnabled] = useState(false);
  const [maxHoldDuration, setMaxHoldDuration] = useState(0);

  // --- View toggle ---
  const [view, setView] = useState<"form" | "simulation">("form");

  // --- Derived ---
  const isStepped =
    portionEnabled && portionSize.trim() !== "" && parseFloat(portionSize) > 0;

  const activeSize = sizeType === "price" ? size : quantity;

  const estimatedSteps = useMemo(() => {
    if (!isStepped) return null;
    const total = parseFloat(activeSize);
    const portion = parseFloat(portionSize);
    if (!total || !portion || total <= 0 || portion <= 0) return null;
    return Math.ceil(total / portion);
  }, [isStepped, activeSize, portionSize]);

  const sizeValid = parseFloat(activeSize) > 0;
  const credentialsReady = longCredentialID !== "" && shortCredentialID !== "";

  // --- Inline simulation (auto-run with debounce) ---
  const [simResult, setSimResult] = useState<SimulationResult | null>(null);
  const [simLoading, setSimLoading] = useState(false);
  const [openPreflight, setOpenPreflight] = useState<OpenPreflight | null>(null);
  const [preflightLoading, setPreflightLoading] = useState(false);
  const simTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const preflightTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Stable min requirements — accumulate the highest minimums seen across preflight calls.
  // Reset when the exchange pair changes so stale values don't carry over.
  const [stableMinNotional, setStableMinNotional] = useState(0);
  const [stableMinBaseQty, setStableMinBaseQty] = useState(0);
  const prevPairRef = useRef("");

  const simSizeType = sizeType;
  const simSize = sizeType === "price" ? size : quantity;
  const simSlippage = slippageEnabled ? slippagePct : "";

  useEffect(() => {
    clearTimeout(simTimerRef.current);
    const val = parseFloat(simSize);
    if (!val || val <= 0 || !symbol || !longExchange || !shortExchange) {
      setSimResult(null);
      return;
    }
    simTimerRef.current = setTimeout(async () => {
      setSimLoading(true);
      try {
        const result = await simulateArbitrage({
          symbol,
          long_exchange_id: longExchange,
          short_exchange_id: shortExchange,
          size_type: simSizeType,
          ...(simSizeType === "price"
            ? { position_size: simSize }
            : { quantity: simSize }),
          slippage_pct: simSlippage || "0",
          hold_hours: "24",
        });
        setSimResult(result);
      } catch {
        setSimResult(null);
      } finally {
        setSimLoading(false);
      }
    }, 600);
    return () => clearTimeout(simTimerRef.current);
  }, [symbol, longExchange, shortExchange, simSizeType, simSize, simSlippage]);

  useEffect(() => {
    clearTimeout(preflightTimerRef.current);

    const val = parseFloat(activeSize);
    if (!val || val <= 0 || !symbol || !longExchange || !shortExchange) {
      setOpenPreflight(null);
      return;
    }

    preflightTimerRef.current = setTimeout(async () => {
      setPreflightLoading(true);
      try {
        const result = await preflightOpenPosition({
          symbol,
          long_exchange_id: longExchange,
          short_exchange_id: shortExchange,
          size_type: sizeType,
          requested_size: activeSize,
          ...(portionEnabled && portionSize.trim() ? { portion_size: portionSize } : {}),
          ...(longLeverage > 1 || shortLeverage > 1 ? { leverage: Math.max(longLeverage, shortLeverage) } : {}),
          ...(marginCapital.trim() ? { margin_capital: marginCapital } : {}),
          ...((longMarginMode === "isolated" || shortMarginMode === "isolated") ? { margin_mode: "isolated" } : { margin_mode: "cross" }),
        });
        setOpenPreflight(result);

        // Reset stable mins if the exchange pair changed.
        const pairKey = `${symbol}|${longExchange}|${shortExchange}`;
        const isNewPair = prevPairRef.current !== pairKey;
        prevPairRef.current = pairKey;

        const legs = [result.long, result.short];
        const newMinNotional = Math.max(...legs.map(l => parseFloat(l.min_notional) || 0));
        const newMinBaseQty = Math.max(...legs.map(l => parseFloat(l.min_contracts) || 0));
        if (isNewPair) {
          setStableMinNotional(newMinNotional);
          setStableMinBaseQty(newMinBaseQty);
        } else {
          setStableMinNotional(prev => Math.max(prev, newMinNotional));
          setStableMinBaseQty(prev => Math.max(prev, newMinBaseQty));
        }
      } catch {
        setOpenPreflight(null);
      } finally {
        setPreflightLoading(false);
      }
    }, 400);

    return () => clearTimeout(preflightTimerRef.current);
  }, [symbol, longExchange, shortExchange, sizeType, activeSize, portionEnabled, portionSize, longLeverage, shortLeverage, marginCapital, longMarginMode, shortMarginMode]);

  useEffect(() => {
    let cancelled = false;

    setCredentialsLoading(true);
    setCredentialsError(null);

    fetchExchangeCredentials()
      .then((items) => {
        if (cancelled) return;
        setCredentials(items);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setCredentialsError(err instanceof Error ? err.message : "Failed to load credentials");
      })
      .finally(() => {
        if (cancelled) return;
        setCredentialsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const longCredentials = useMemo(
    () => credentials.filter((c) => c.exchange_id === longExchange),
    [credentials, longExchange]
  );

  const shortCredentials = useMemo(
    () => credentials.filter((c) => c.exchange_id === shortExchange),
    [credentials, shortExchange]
  );

  useEffect(() => {
    const selectedStillValid = longCredentials.some((c) => c.id === longCredentialID);
    if (selectedStillValid) return;
    const preferred = longCredentials.find((c) => c.is_default) ?? longCredentials[0];
    setLongCredentialID(preferred?.id ?? "");
  }, [longCredentials, longCredentialID]);

  useEffect(() => {
    const selectedStillValid = shortCredentials.some((c) => c.id === shortCredentialID);
    if (selectedStillValid) return;
    const preferred = shortCredentials.find((c) => c.is_default) ?? shortCredentials[0];
    setShortCredentialID(preferred?.id ?? "");
  }, [shortCredentials, shortCredentialID]);

  // Converted size hint from simulation result (qty↔price).
  const convertedSize = useMemo(() => {
    if (!simResult) return null;
    if (sizeType === "price") {
      const qty = parseFloat(simResult.quantity);
      return qty > 0 ? `≈ ${Math.floor(qty)} base` : null;
    }
    const usd = parseFloat(simResult.actual_position_size || simResult.position_size);
    return usd > 0 ? `≈ $${usd.toLocaleString(undefined, { maximumFractionDigits: 2 })}` : null;
  }, [sizeType, simResult]);

  const blockedByPreflight = !openPreflight || !openPreflight.can_open;

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
      long_credential_id: longCredentialID,
      short_exchange_id: shortExchange,
      short_credential_id: shortCredentialID,
      size_type: sizeType,
      requested_size: sizeType === "price" ? size : quantity,
    };

    if (portionEnabled && portionSize.trim()) req.portion_size = portionSize;
    if (slippageEnabled && slippagePct.trim()) req.slippage_pct = slippagePct;

    if (longLeverage > 1) req.leverage = longLeverage;
    req.long_margin_mode = longMarginMode;
    req.short_margin_mode = shortMarginMode;
    if (marginCapital.trim()) req.margin_capital = marginCapital;

    if (strategy === "price_spread") {
      if (targetSpreadPct.trim()) req.target_spread_pct = targetSpreadPct;
      if (stopLossSpreadPct.trim()) req.stop_loss_spread_pct = stopLossSpreadPct;
    } else {
      // funding_rate: hold as long as profitable — no target spread
    }
    if (maxHoldEnabled && maxHoldDuration > 0) req.max_hold_duration = maxHoldDuration;

    createMutation.mutate(req);
  }, [
    symbol,
    longExchange,
    longCredentialID,
    shortExchange,
    shortCredentialID,
    sizeType,
    size,
    quantity,
    portionEnabled,
    portionSize,
    slippageEnabled,
    slippagePct,
    longLeverage,
    shortLeverage,
    longMarginMode,
    shortMarginMode,
    marginCapital,
    priceSpreadEnabled,
    strategy,
    targetSpreadPct,
    stopLossSpreadPct,
    maxHoldEnabled,
    maxHoldDuration,
    createMutation,
  ]);

  const simProps = {
    symbol,
    longExchange,
    shortExchange,
    sizeType: sizeType,
    size: sizeType === "price" ? size : quantity,
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
                {sizeType === "quantity" ? "Quantity (Base)" : "Price (USDT)"}
              </Label>
              <div className="flex h-6 items-center rounded-md border bg-muted/40 p-0.5">
                {(["quantity", "price"] as const).map((t) => (
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
                    {t === "quantity" ? "Quantity" : "Price"}
                  </button>
                ))}
              </div>
            </div>
            <div className="relative">
              <Input
                type="text"
                inputMode="decimal"
                value={sizeType === "price" ? size : quantity}
                onChange={(e) =>
                  sizeType === "price"
                    ? setSize(e.target.value)
                    : setQuantity(e.target.value)
                }
                placeholder="1000"
                className="font-mono tabular-nums text-xs h-7 pr-[var(--hint-width,4rem)]"
              />
              {convertedSize && (
                <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[0.625rem] text-muted-foreground/60 font-mono tabular-nums pointer-events-none select-none">
                  {convertedSize}
                </span>
              )}
            </div>
            {/* Min requirement hint — always shown once minimums are known */}
            {(sizeType === "price" ? stableMinNotional > 0 : stableMinBaseQty > 0) && (
              <p className={cn(
                "text-[0.625rem] mt-0.5",
                openPreflight && !openPreflight.can_open
                  ? "text-red-600 dark:text-red-400"
                  : "text-muted-foreground/60"
              )}>
                {sizeType === "price"
                  ? `min ${formatUsdPlain(String(stableMinNotional))}`
                  : `min ${formatQty(String(stableMinBaseQty))} base qty`}
              </p>
            )}
          </div>

          {/* ── Portion Size ── */}
          <div>
            <label className="flex items-center gap-1.5 cursor-pointer select-none mb-1.5">
              <input
                type="checkbox"
                checked={portionEnabled}
                onChange={(e) => setPortionEnabled(e.target.checked)}
                className="h-3 w-3 rounded-sm accent-primary"
              />
              <span className="text-xs text-muted-foreground">
                Portion Size
                {portionEnabled && isStepped && estimatedSteps && (
                  <span className="ml-1 text-violet-600 dark:text-violet-400">
                    ({estimatedSteps} steps)
                  </span>
                )}
              </span>
            </label>
            {portionEnabled && (
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  {portionSize && (
                    <button
                      type="button"
                      onClick={() => setPortionSize("")}
                      className="flex items-center gap-0.5 text-[0.625rem] text-muted-foreground/60 hover:text-foreground transition-colors ml-auto"
                    >
                      <span>×</span> Clear
                    </button>
                  )}
                </div>
                <Input
                  type="text"
                  inputMode="decimal"
                  value={portionSize}
                  onChange={(e) => setPortionSize(e.target.value)}
                  placeholder="e.g. 250"
                  className="font-mono tabular-nums text-xs h-7"
                />
                <div className="flex gap-1">
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
            )}
          </div>

          {/* ── Margin Capital ── */}
          <div>
            <label className="flex items-center gap-1.5 cursor-pointer select-none mb-1.5">
              <input
                type="checkbox"
                checked={marginCapitalEnabled}
                onChange={(e) => {
                  setMarginCapitalEnabled(e.target.checked);
                  if (!e.target.checked) setMarginCapital("");
                }}
                className="h-3 w-3 rounded-sm accent-primary"
              />
              <span className="text-xs text-muted-foreground">Margin Capital (USDT)</span>
            </label>
            {marginCapitalEnabled && (
              <>
                <Input
                  type="text"
                  inputMode="decimal"
                  value={marginCapital}
                  onChange={(e) => setMarginCapital(e.target.value)}
                  placeholder="e.g. 500"
                  className="font-mono tabular-nums text-xs h-7"
                />
                {(() => {
                  const mcVal = parseFloat(marginCapital);
                  if (!mcVal || mcVal <= 0 || !openPreflight) return null;
                  const legsData = [
                    { leg: openPreflight.long, lev: longLeverage },
                    { leg: openPreflight.short, lev: shortLeverage },
                  ];
                  const maxReq = Math.max(...legsData.map(({ leg, lev }) => {
                    const price = parseFloat(leg.price) || 0;
                    const qty = parseFloat(leg.required_base_qty) || 0;
                    return (price * qty) / lev;
                  }));
                  if (maxReq > 0 && mcVal < maxReq) {
                    return (
                      <p className="text-[0.625rem] mt-0.5 text-amber-600 dark:text-amber-400">
                        May be too small — est. required ≈ {formatUsdPlain(String(maxReq))}
                      </p>
                    );
                  }
                  return null;
                })()}
              </>
            )}
          </div>

          {/* ── Slippage ── */}
          <div>
            <label className="flex items-center gap-1.5 cursor-pointer select-none mb-1.5">
              <input
                type="checkbox"
                checked={slippageEnabled}
                onChange={(e) => setSlippageEnabled(e.target.checked)}
                className="h-3 w-3 rounded-sm accent-primary"
              />
              <span className="text-xs text-muted-foreground">Max Slippage %</span>
            </label>
            {slippageEnabled && (
              <>
                <Input
                  type="text"
                  inputMode="decimal"
                  value={slippagePct}
                  onChange={(e) => setSlippagePct(e.target.value)}
                  placeholder="No limit"
                  className="font-mono tabular-nums text-xs h-7"
                />
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
              </>
            )}
          </div>

          {/* ── Price Spread TP/SL ── */}
          <div>
            <label className="flex items-center gap-1.5 cursor-pointer select-none mb-1.5">
              <input
                type="checkbox"
                checked={priceSpreadEnabled}
                onChange={(e) => setPriceSpreadEnabled(e.target.checked)}
                className="h-3 w-3 rounded-sm accent-primary"
              />
              <span className="text-xs text-muted-foreground">Price Spread TP/SL</span>
            </label>
            {priceSpreadEnabled && (
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <Label className="text-xs text-muted-foreground">TP Spread %</Label>
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
                  <Label className="text-xs text-muted-foreground">SL Spread %</Label>
                  <Input
                    type="text"
                    inputMode="decimal"
                    value={stopLossSpreadPct}
                    onChange={(e) => setStopLossSpreadPct(e.target.value)}
                    placeholder="None"
                    className="font-mono tabular-nums text-xs h-7 mt-0.5"
                  />
                </div>
              </div>
            )}
          </div>

          {/* ── Max Hold ── */}
          <div>
            <label className="flex items-center gap-1.5 cursor-pointer select-none mb-1.5">
              <input
                type="checkbox"
                checked={maxHoldEnabled}
                onChange={(e) => setMaxHoldEnabled(e.target.checked)}
                className="h-3 w-3 rounded-sm accent-primary"
              />
              <span className="text-xs text-muted-foreground">Max Hold</span>
            </label>
            {maxHoldEnabled && (
              <div className="grid grid-cols-7 gap-1">
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
            )}
          </div>

          <Separator />

          {/* ── Exchange legs: credentials + leverage + margin mode + preflight status ── */}
          <div className="space-y-3">
            {(["long", "short"] as const).map((side) => {
              const exchange = side === "long" ? longExchange : shortExchange;
              const credentialID = side === "long" ? longCredentialID : shortCredentialID;
              const setCredentialID = side === "long" ? setLongCredentialID : setShortCredentialID;
              const creds = side === "long" ? longCredentials : shortCredentials;
              const leverage = side === "long" ? longLeverage : shortLeverage;
              const setLeverage = side === "long" ? setLongLeverage : setShortLeverage;
              const marginMode = side === "long" ? longMarginMode : shortMarginMode;
              const setMarginMode = side === "long" ? setLongMarginMode : setShortMarginMode;
              const leg = openPreflight?.[side];
              const issues = leg ? (openPreflight?.errors ?? []).filter(e => e.exchange === leg.exchange_id) : [];
              const warnings = leg ? (openPreflight?.warnings ?? []).filter(w => w.code !== "isolated_no_capital" && w.code !== "missing_margin_capital") : [];
              const passed = leg ? issues.length === 0 : null;

              return (
                <ExchangeLegBlock
                  key={side}
                  side={side}
                  exchange={exchange}
                  credentialID={credentialID}
                  setCredentialID={setCredentialID}
                  creds={creds}
                  credentialsLoading={credentialsLoading}
                  leverage={leverage}
                  setLeverage={setLeverage}
                  marginMode={marginMode}
                  setMarginMode={setMarginMode}
                  leg={leg ?? null}
                  issues={issues}
                  warnings={warnings}
                  passed={passed}
                  preflightLoading={preflightLoading}
                />
              );
            })}

            {credentialsError && (
              <div className="rounded-md bg-destructive/10 border border-destructive/20 px-2.5 py-1.5 text-[0.625rem] text-destructive">
                {credentialsError}
              </div>
            )}
            {(openPreflight?.warnings ?? []).filter(w => w.code !== "isolated_no_capital" && w.code !== "missing_margin_capital").length > 0 && (
              <div className="rounded-md border border-amber-500/20 bg-amber-500/10 px-2.5 py-1.5 space-y-0.5">
                {(openPreflight!.warnings).filter(w => w.code !== "isolated_no_capital" && w.code !== "missing_margin_capital").map((w) => (
                  <div key={w.code} className="flex items-start gap-1.5 text-[0.625rem] text-amber-700 dark:text-amber-300">
                    <AlertTriangle className="h-3 w-3 shrink-0 mt-px" />
                    {w.message}
                  </div>
                ))}
              </div>
            )}
          </div>

          <Separator />

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

          {/* ── Simulation Summary ── */}
          {simLoading && !simResult && (
            <div className="flex items-center gap-1.5 text-[0.625rem] text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              Estimating...
            </div>
          )}
          {simResult && (
            <SimSummary
              result={simResult}
              estimatedSteps={estimatedSteps}
            />
          )}

          {/* ── Actions: Open Position (wide) + Simulate icon (narrow) ── */}
          <div className="flex gap-1.5">
            <Button
              size="sm"
              className="h-7 text-xs flex-1"
              disabled={!sizeValid || !credentialsReady || createMutation.isPending || blockedByPreflight}
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
          {blockedByPreflight && (
            <div className="rounded-md border border-amber-500/20 bg-amber-500/10 px-2.5 py-1.5 text-[0.625rem] text-amber-700 dark:text-amber-300">
              Open is blocked by preflight rules. Increase total size or increase portion size (fewer steps).
            </div>
          )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ExchangeLegBlock — credentials + leverage + margin + preflight status
// ---------------------------------------------------------------------------

function ExchangeLegBlock({
  side,
  exchange,
  credentialID,
  setCredentialID,
  creds,
  credentialsLoading,
  leverage,
  setLeverage,
  marginMode,
  setMarginMode,
  leg,
  issues,
  warnings,
  passed,
  preflightLoading,
}: {
  side: "long" | "short";
  exchange: string;
  credentialID: string;
  setCredentialID: (v: string) => void;
  creds: ExchangeCredential[];
  credentialsLoading: boolean;
  leverage: number;
  setLeverage: (v: number) => void;
  marginMode: "isolated" | "cross";
  setMarginMode: (v: "isolated" | "cross") => void;
  leg: OpenPreflightLeg | null;
  issues: OpenPreflightIssue[];
  warnings: OpenPreflightIssue[];
  passed: boolean | null;
  preflightLoading: boolean;
}) {
  const [detailOpen, setDetailOpen] = useState(false);
  const isLong = side === "long";

  return (
    <div className="rounded-md border border-border/60 bg-muted/20 px-2.5 py-2 space-y-2">
      {/* Header row: side label + exchange + preflight status badge */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1.5">
          <span className={cn(
            "text-[0.625rem] font-medium uppercase tracking-wider",
            isLong ? "text-emerald-600 dark:text-emerald-400" : "text-red-500 dark:text-red-400"
          )}>
            {isLong ? "↑ Long" : "↓ Short"}
          </span>
          <span className="text-[0.625rem] text-muted-foreground">{exchange}</span>
        </div>

        {/* Preflight status — clickable to expand */}
        {leg && (
          <button
            type="button"
            onClick={() => setDetailOpen(o => !o)}
            className={cn(
              "inline-flex items-center gap-1 text-[0.625rem] rounded px-1.5 py-0.5 transition-colors",
              passed
                ? "text-emerald-600 dark:text-emerald-400 hover:bg-emerald-500/10"
                : "text-red-600 dark:text-red-400 hover:bg-red-500/10"
            )}
            title={detailOpen ? "Hide details" : "Show details"}
          >
            {passed
              ? <CheckCircle2 className="h-3 w-3" />
              : <AlertCircle className="h-3 w-3" />}
            {leg ? formatPrice(leg.price) : null}
            {detailOpen
              ? <ChevronDown className="h-3 w-3 opacity-60" />
              : <ChevronRight className="h-3 w-3 opacity-60" />}
          </button>
        )}
        {!leg && preflightLoading && (
          <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
        )}
      </div>

      {/* Preflight detail panel */}
      {detailOpen && leg && (
        <div className="rounded-md border border-border/50 bg-background/60 px-2 py-1.5 space-y-0.5">
          <div className="grid grid-cols-2 gap-x-3 gap-y-0.5 text-[0.625rem]">
            <span className="text-muted-foreground">Min notional</span>
            <span className="font-mono tabular-nums text-right">{formatUsdPlain(leg.min_notional)}</span>
            <span className="text-muted-foreground">Min base qty</span>
            <span className="font-mono tabular-nums text-right">{formatQty(leg.min_contracts)}</span>
            <span className="text-muted-foreground">Qty step</span>
            <span className="font-mono tabular-nums text-right">{formatQty(leg.contracts_step)}</span>
            <span className="text-muted-foreground">Required base qty</span>
            <span className="font-mono tabular-nums text-right">{formatQty(leg.required_base_qty)}</span>
          </div>
          {!passed && issues.length > 0 && (
            <div className="pt-1 space-y-0.5 text-[0.625rem] text-red-600 dark:text-red-400 border-t border-border/40">
              {issues.map((issue) => (
                <div key={issue.code + issue.message}>— {issue.message}</div>
              ))}
            </div>
          )}
          {warnings.length > 0 && (
            <div className="pt-1 space-y-0.5 text-[0.625rem] text-amber-600 dark:text-amber-400 border-t border-border/40">
              {warnings.map((w) => (
                <div key={w.code + w.message} className="flex items-start gap-1">
                  <AlertTriangle className="h-2.5 w-2.5 shrink-0 mt-px" />
                  {w.message}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Errors inline when detail is collapsed */}
      {!detailOpen && !passed && issues.length > 0 && (
        <div className="space-y-0.5 text-[0.625rem] text-red-600 dark:text-red-400">
          {issues.map((issue) => (
            <div key={issue.code + issue.message}>— {issue.message}</div>
          ))}
        </div>
      )}

      {/* Credential */}
      <div>
        <Label className="text-[0.625rem] text-muted-foreground block mb-1">Credential</Label>
        <select
          value={credentialID}
          onChange={(e) => setCredentialID(e.target.value)}
          className="w-full h-7 rounded-md border bg-background px-2 text-xs"
          disabled={credentialsLoading || creds.length === 0}
        >
          {creds.length === 0 ? (
            <option value="">No credentials found</option>
          ) : (
            creds.map((c) => (
              <option key={c.id} value={c.id}>{c.title}</option>
            ))
          )}
        </select>
      </div>

      {/* Leverage + Margin side by side */}
      <div className="grid grid-cols-2 gap-2">
        <div>
          <Label className="text-[0.625rem] text-muted-foreground block mb-1">Leverage</Label>
          <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5">
            {[1, 2, 3].map((lv) => (
              <button
                key={lv}
                type="button"
                onClick={() => setLeverage(lv)}
                className={cn(
                  "flex-1 rounded-sm h-5 text-[0.625rem] font-medium transition-colors",
                  leverage === lv
                    ? "bg-background text-foreground shadow-sm border border-border/50"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {lv}x
              </button>
            ))}
          </div>
        </div>
        <div>
          <Label className="text-[0.625rem] text-muted-foreground block mb-1">Margin</Label>
          <div className="flex h-7 items-center rounded-md border bg-muted/40 p-0.5 gap-0.5">
            {(["isolated", "cross"] as const).map((m) => (
              <button
                key={m}
                type="button"
                onClick={() => setMarginMode(m)}
                className={cn(
                  "flex-1 rounded-sm h-5 text-[0.625rem] font-medium transition-colors capitalize",
                  marginMode === m
                    ? "bg-background text-foreground shadow-sm border border-border/50"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {m}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SimSummary — fees, spread context, funding P&L with countdowns
// ---------------------------------------------------------------------------

function SimSummary({ result, estimatedSteps }: { result: SimulationResult; estimatedSteps: number | null }) {
  // Live countdown ticker (re-renders every second).
  const [, setTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setTick((t) => t + 1), 1000);
    return () => clearInterval(id);
  }, []);

  const breakevenHours = parseFloat(result.breakeven_hours);
  const spreadPct = parseFloat(result.open_spread_pct);

  const actualNotional = parseFloat(result.actual_position_size || result.position_size) || 0;
  const contracts = parseFloat(result.contracts) || 0;
  const steps = estimatedSteps && estimatedSteps > 0 ? estimatedSteps : 1;
  const perStepNotional = actualNotional / steps;
  const perStepContracts = contracts / steps;

  return (
    <div className="rounded-md border bg-muted/30 px-2.5 py-2 space-y-1.5">
      <div className="grid grid-cols-2 gap-x-3 gap-y-0.5 text-[0.625rem]">
        {/* Execution plan */}
        <span className="text-muted-foreground">Notional</span>
        <span className="font-mono tabular-nums text-right">{formatUsdPlain(String(actualNotional))}</span>
        <span className="text-muted-foreground">Base qty</span>
        <span className="font-mono tabular-nums text-right">{formatQty(String(contracts))}</span>
        {steps > 1 && (
          <>
            <span className="text-muted-foreground">Per step</span>
            <span className="font-mono tabular-nums text-right">
              {formatUsdPlain(String(perStepNotional))} · {formatQty(String(perStepContracts))} base
            </span>
          </>
        )}

        <span className="col-span-2 border-t border-border/30 my-0.5" />

        {/* Fees — single line */}
        <span className="text-muted-foreground">Total fees</span>
        <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
          {formatUsdPlain(result.total_cost)}
        </span>

        {/* Spread */}
        <span className="text-muted-foreground">Current spread</span>
        <span className={cn(
          "font-mono tabular-nums text-right",
          spreadPct >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400"
        )}>
          {formatPct(result.open_spread_pct)}
        </span>
        <span className="text-muted-foreground">Spread 24h range</span>
        <span className="font-mono tabular-nums text-right text-muted-foreground">
          {formatPct(result.spread_range_24h.min)} – {formatPct(result.spread_range_24h.max)}
        </span>

        <span className="col-span-2 border-t border-border/30 my-0.5" />

        {/* Funding */}
        <span className="text-muted-foreground">Funding / 8h</span>
        <SimPnl value={result.funding_8h} />
        <span className="text-muted-foreground">Breakeven</span>
        <span className="font-mono tabular-nums text-right">
          {breakevenHours < 0 ? "never" : `${breakevenHours.toFixed(1)}h`}
        </span>
        <span className="text-muted-foreground">
          Next funding
          {result.next_funding_at && (
            <span className="ml-1 text-muted-foreground/60">{formatCountdown(result.next_funding_at)}</span>
          )}
        </span>
        <SimPnl value={result.next_pnl} bold />
        <span className="text-muted-foreground">24h P&L</span>
        <SimPnl value={result.pnl_hold} bold />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SimPnl — inline P&L value with color
// ---------------------------------------------------------------------------

function SimPnl({ value, bold }: { value: string; bold?: boolean }) {
  const num = parseFloat(value);
  return (
    <span
      className={cn(
        "font-mono tabular-nums text-right",
        bold && "font-medium",
        num >= 0
          ? "text-emerald-600 dark:text-emerald-400"
          : "text-red-600 dark:text-red-400"
      )}
    >
      {formatUsd(value)}
    </span>
  );
}
