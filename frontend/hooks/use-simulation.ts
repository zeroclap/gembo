"use client";

import { useState, useCallback, useMemo, useEffect } from "react";
import type { FundingArbitrage } from "@/lib/api";

const STORAGE_KEY = "gembo:simulation";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface SimulationConfig {
  enabled: boolean;
  positionSize: string; // USD, e.g. "1000"
  slippagePct: string; // e.g. "0.05" (percent)
}

export interface SimulationActions {
  setEnabled: (enabled: boolean) => void;
  setPositionSize: (value: string) => void;
  setSlippagePct: (value: string) => void;
  reset: () => void;
}

export interface SimulationResult {
  positionSize: number; // full notional value in USD
  capital: number; // actual capital required
  entryCost: number; // $ cost to open (fees + spread loss + slippage)
  exitCost: number; // $ cost to close (fees + slippage)
  totalCost: number; // entry + exit
  funding8h: number; // $ funding P&L per 8h period
  funding24h: number; // $ funding P&L per day
  breakevenDays: number; // days to recoup entry+exit cost from funding
  /** P&L for one full LCM loop (funding only, no cost amortization) */
  lcmPnL: number;
  /** P&L for the next funding cycle — sums all legs that fire at the nearest payment time */
  nextPnL: number;
  /** P&L for 24h of funding (net_rate_8h * 3 * positionSize) */
  pnl24h: number;
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

const DEFAULT_CONFIG: SimulationConfig = {
  enabled: false,
  positionSize: "1000",
  slippagePct: "0.05",
};

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

function loadConfig(): SimulationConfig {
  if (typeof window === "undefined") return DEFAULT_CONFIG;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_CONFIG;
    const parsed = JSON.parse(raw) as Partial<SimulationConfig>;
    return {
      enabled: typeof parsed.enabled === "boolean" ? parsed.enabled : false,
      positionSize:
        typeof parsed.positionSize === "string"
          ? parsed.positionSize
          : DEFAULT_CONFIG.positionSize,
      slippagePct:
        typeof parsed.slippagePct === "string"
          ? parsed.slippagePct
          : DEFAULT_CONFIG.slippagePct,
    };
  } catch {
    return DEFAULT_CONFIG;
  }
}

function saveConfig(config: SimulationConfig) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(config));
  } catch {
    // Storage full or unavailable — silently ignore.
  }
}

// ---------------------------------------------------------------------------
// Next payment helper
// ---------------------------------------------------------------------------

/**
 * Compute the net P&L percentage for the next funding cycle.
 *
 * The "next" cycle is defined by the soonest `next_funding_at` timestamp across
 * both legs. If both legs fire at the same time they are summed.
 *
 * For the long leg:  P&L = −fundingRate  (positive rate → you pay → negative P&L)
 * For the short leg: P&L = +fundingRate  (positive rate → you receive)
 */
function computeNextPaymentPct(arb: FundingArbitrage): number {
  const longAt = new Date(arb.long.next_funding_at).getTime();
  const shortAt = new Date(arb.short.next_funding_at).getTime();

  const longValid = !isNaN(longAt);
  const shortValid = !isNaN(shortAt);

  if (!longValid && !shortValid) return 0;

  const longPnl = -(parseFloat(arb.long.funding_rate) || 0); // long pays opposite
  const shortPnl = parseFloat(arb.short.funding_rate) || 0; // short receives as-is

  if (!shortValid) return longPnl;
  if (!longValid) return shortPnl;

  // Both valid — check if they coincide (within 60s tolerance)
  const diff = Math.abs(longAt - shortAt);
  if (diff <= 60_000) {
    // Same time — sum both
    return longPnl + shortPnl;
  }

  // Different times — pick the earlier one
  return longAt < shortAt ? longPnl : shortPnl;
}

// ---------------------------------------------------------------------------
// Computation (pure, per-row)
// ---------------------------------------------------------------------------

export function computeSimulation(
  arb: FundingArbitrage,
  config: SimulationConfig
): SimulationResult {
  const size = parseFloat(config.positionSize) || 0;
  const slippage = parseFloat(config.slippagePct) || 0;

  const capital = size;

  // Entry cost: open spread loss + open fees + slippage on entry
  const openSpreadPct = parseFloat(arb.open_spread) || 0;
  const spreadCostPct = Math.max(0, -openSpreadPct);
  const spreadCost = (spreadCostPct / 100) * size;

  const openFeesPct = parseFloat(arb.fees.open_fees_pct) || 0;
  const openFees = (openFeesPct / 100) * size;

  const closeFeesPct = parseFloat(arb.fees.close_fees_pct) || 0;
  const closeFees = (closeFeesPct / 100) * size;

  const slippageCostPerSide = (slippage / 100) * size;

  const entryCost = spreadCost + openFees + slippageCostPerSide;
  const exitCost = closeFees + slippageCostPerSide;
  const totalCost = entryCost + exitCost;

  // Funding P&L per 8h period (net_rate_8h is a ratio, e.g. 0.0001)
  const netRate8h = parseFloat(arb.net_rate_8h) || 0;
  const funding8h = netRate8h * size;
  const funding24h = funding8h * 3;

  // Breakeven
  const breakevenDays = funding24h > 0 ? totalCost / funding24h : Infinity;

  // LCM P&L: funding earned over one full LCM loop (no cost amortization)
  const netPctPerLoop = parseFloat(arb.full_loop.net_pct_per_loop) || 0;
  const lcmPnL = netPctPerLoop * size;

  // Next payment P&L: sum of all legs firing at the nearest payment time
  const nextPaymentPct = computeNextPaymentPct(arb);
  const nextPnL = nextPaymentPct * size;

  // 24h P&L: same as funding24h
  const pnl24h = funding24h;

  return {
    positionSize: size,
    capital,
    entryCost,
    exitCost,
    totalCost,
    funding8h,
    funding24h,
    breakevenDays,
    lcmPnL,
    nextPnL,
    pnl24h,
  };
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useSimulation(): SimulationConfig &
  SimulationActions & {
    summary: string;
  } {
  const [config, setConfig] = useState<SimulationConfig>(loadConfig);

  useEffect(() => {
    saveConfig(config);
  }, [config]);

  const update = useCallback(
    (patch: Partial<SimulationConfig>) =>
      setConfig((prev) => ({ ...prev, ...patch })),
    []
  );

  const setEnabled = useCallback(
    (v: boolean) => update({ enabled: v }),
    [update]
  );
  const setPositionSize = useCallback(
    (v: string) => update({ positionSize: v }),
    [update]
  );
  const setSlippagePct = useCallback(
    (v: string) => update({ slippagePct: v }),
    [update]
  );
  const reset = useCallback(() => setConfig(DEFAULT_CONFIG), []);

  const summary = useMemo(() => {
    const size = parseFloat(config.positionSize) || 0;
    const formatted =
      size >= 1000
        ? `$${(size / 1000).toLocaleString(undefined, { maximumFractionDigits: 1 })}K`
        : `$${size.toLocaleString()}`;
    return formatted;
  }, [config.positionSize]);

  return {
    ...config,
    summary,
    setEnabled,
    setPositionSize,
    setSlippagePct,
    reset,
  };
}
