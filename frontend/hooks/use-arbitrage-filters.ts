"use client";

import { useState, useCallback, useMemo, useEffect } from "react";
import type { FundingArbitrage } from "@/lib/api";

const STORAGE_KEY = "gembo:arbitrage-filters";

// --- Types ---

export interface ArbitrageFilters {
  symbols: string[];
  exchanges: string[];
  volumeMin: string;
  volumeMax: string;
  spreadMin: string;
  spreadMax: string;
  fundingSpreadMin: string;
  fundingSpreadMax: string;
  fundingLoopMin: string;
  fundingLoopMax: string;
}

export interface ArbitrageFiltersActions {
  setSymbols: (symbols: string[]) => void;
  setExchanges: (exchanges: string[]) => void;
  setVolumeMin: (value: string) => void;
  setVolumeMax: (value: string) => void;
  setSpreadMin: (value: string) => void;
  setSpreadMax: (value: string) => void;
  setFundingSpreadMin: (value: string) => void;
  setFundingSpreadMax: (value: string) => void;
  setFundingLoopMin: (value: string) => void;
  setFundingLoopMax: (value: string) => void;
  clearAll: () => void;
}

const EMPTY_FILTERS: ArbitrageFilters = {
  symbols: [],
  exchanges: [],
  volumeMin: "",
  volumeMax: "",
  spreadMin: "",
  spreadMax: "",
  fundingSpreadMin: "",
  fundingSpreadMax: "",
  fundingLoopMin: "",
  fundingLoopMax: "",
};

function loadFilters(): ArbitrageFilters {
  if (typeof window === "undefined") return EMPTY_FILTERS;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return EMPTY_FILTERS;
    const parsed = JSON.parse(raw) as Partial<ArbitrageFilters>;
    return {
      symbols: Array.isArray(parsed.symbols) ? parsed.symbols : [],
      exchanges: Array.isArray(parsed.exchanges) ? parsed.exchanges : [],
      volumeMin: typeof parsed.volumeMin === "string" ? parsed.volumeMin : "",
      volumeMax: typeof parsed.volumeMax === "string" ? parsed.volumeMax : "",
      spreadMin: typeof parsed.spreadMin === "string" ? parsed.spreadMin : "",
      spreadMax: typeof parsed.spreadMax === "string" ? parsed.spreadMax : "",
      fundingSpreadMin: typeof parsed.fundingSpreadMin === "string" ? parsed.fundingSpreadMin : "",
      fundingSpreadMax: typeof parsed.fundingSpreadMax === "string" ? parsed.fundingSpreadMax : "",
      fundingLoopMin: typeof parsed.fundingLoopMin === "string" ? parsed.fundingLoopMin : "",
      fundingLoopMax: typeof parsed.fundingLoopMax === "string" ? parsed.fundingLoopMax : "",
    };
  } catch {
    return EMPTY_FILTERS;
  }
}

function saveFilters(filters: ArbitrageFilters) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(filters));
  } catch {
    // Storage full or unavailable — silently ignore.
  }
}

// --- Hook ---

export function useArbitrageFilters(): ArbitrageFilters & ArbitrageFiltersActions & {
  activeFilterCount: number;
  filterData: (data: FundingArbitrage[]) => FundingArbitrage[];
} {
  const [filters, setFilters] = useState<ArbitrageFilters>(loadFilters);

  // Persist to localStorage on every change.
  useEffect(() => {
    saveFilters(filters);
  }, [filters]);

  const activeFilterCount = useMemo(() => {
    let count = 0;
    if (filters.symbols.length > 0) count++;
    if (filters.exchanges.length > 0) count++;
    if (filters.volumeMin || filters.volumeMax) count++;
    if (filters.spreadMin || filters.spreadMax) count++;
    if (filters.fundingSpreadMin || filters.fundingSpreadMax) count++;
    if (filters.fundingLoopMin || filters.fundingLoopMax) count++;
    return count;
  }, [filters]);

  // --- Setters (each produces a new filters object -> triggers persist) ---

  const update = useCallback(
    (patch: Partial<ArbitrageFilters>) => setFilters((prev) => ({ ...prev, ...patch })),
    []
  );

  const setSymbols = useCallback((v: string[]) => update({ symbols: v }), [update]);
  const setExchanges = useCallback((v: string[]) => update({ exchanges: v }), [update]);
  const setVolumeMin = useCallback((v: string) => update({ volumeMin: v }), [update]);
  const setVolumeMax = useCallback((v: string) => update({ volumeMax: v }), [update]);
  const setSpreadMin = useCallback((v: string) => update({ spreadMin: v }), [update]);
  const setSpreadMax = useCallback((v: string) => update({ spreadMax: v }), [update]);
  const setFundingSpreadMin = useCallback((v: string) => update({ fundingSpreadMin: v }), [update]);
  const setFundingSpreadMax = useCallback((v: string) => update({ fundingSpreadMax: v }), [update]);
  const setFundingLoopMin = useCallback((v: string) => update({ fundingLoopMin: v }), [update]);
  const setFundingLoopMax = useCallback((v: string) => update({ fundingLoopMax: v }), [update]);
  const clearAll = useCallback(() => setFilters(EMPTY_FILTERS), []);

  // --- Filtering logic ---

  const filterData = useCallback(
    (data: FundingArbitrage[]): FundingArbitrage[] => {
      return data.filter((arb) => {
        // Symbol filter (case-insensitive substring match against tags)
        if (filters.symbols.length > 0) {
          const sym = arb.symbol.toLowerCase();
          if (!filters.symbols.some((tag) => sym.includes(tag.toLowerCase()))) {
            return false;
          }
        }

        // Exchange filter (either leg matches any selected exchange)
        if (filters.exchanges.length > 0) {
          const exLower = filters.exchanges.map((e) => e.toLowerCase());
          const longEx = arb.long.exchange_id.toLowerCase();
          const shortEx = arb.short.exchange_id.toLowerCase();
          if (!exLower.some((e) => longEx === e || shortEx === e)) {
            return false;
          }
        }

        // Volume: min volume of the two legs (the bottleneck)
        // Filter input is in millions (1 = 1M)
        const minVolume = Math.min(
          parseFloat(arb.long.volume_24h) || 0,
          parseFloat(arb.short.volume_24h) || 0
        );
        if (filters.volumeMin) {
          const min = parseFloat(filters.volumeMin);
          if (!isNaN(min) && minVolume < min * 1_000_000) return false;
        }
        if (filters.volumeMax) {
          const max = parseFloat(filters.volumeMax);
          if (!isNaN(max) && minVolume > max * 1_000_000) return false;
        }

        // Open spread
        const spread = parseFloat(arb.open_spread) || 0;
        if (filters.spreadMin) {
          const min = parseFloat(filters.spreadMin);
          if (!isNaN(min) && spread < min) return false;
        }
        if (filters.spreadMax) {
          const max = parseFloat(filters.spreadMax);
          if (!isNaN(max) && spread > max) return false;
        }

        // Funding spread (net_rate_8h * 100)
        const fundingSpread = (parseFloat(arb.net_rate_8h) || 0) * 100;
        if (filters.fundingSpreadMin) {
          const min = parseFloat(filters.fundingSpreadMin);
          if (!isNaN(min) && fundingSpread < min) return false;
        }
        if (filters.fundingSpreadMax) {
          const max = parseFloat(filters.fundingSpreadMax);
          if (!isNaN(max) && fundingSpread > max) return false;
        }

        // Full loop net (net_pct_per_loop * 100)
        const loopNet = (parseFloat(arb.full_loop.net_pct_per_loop) || 0) * 100;
        if (filters.fundingLoopMin) {
          const min = parseFloat(filters.fundingLoopMin);
          if (!isNaN(min) && loopNet < min) return false;
        }
        if (filters.fundingLoopMax) {
          const max = parseFloat(filters.fundingLoopMax);
          if (!isNaN(max) && loopNet > max) return false;
        }

        return true;
      });
    },
    [filters]
  );

  return {
    ...filters,
    activeFilterCount,
    setSymbols,
    setExchanges,
    setVolumeMin,
    setVolumeMax,
    setSpreadMin,
    setSpreadMax,
    setFundingSpreadMin,
    setFundingSpreadMax,
    setFundingLoopMin,
    setFundingLoopMax,
    clearAll,
    filterData,
  };
}
