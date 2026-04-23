"use client";

import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import {
  fetchTicker,
  fetchFundingRate,
  fetchFundingArbitrage,
  fetchSpreadArbitrage,
  fetchPositions,
  fetchPositionOrders,
  fetchPositionFunding,
  type Ticker,
  type FundingRate,
  type FundingArbitrage,
  type SpreadArbitrage,
  type Position,
  type ExchangeOrder,
  type FundingPayment,
} from "@/lib/api";

/** Pair selection state from URL query params. */
export interface ArbitragePair {
  symbol: string;
  longExchange: string;
  shortExchange: string;
}

/** Aggregated ticker data for both legs. */
export interface ArbitrageTickers {
  long: Ticker | undefined;
  short: Ticker | undefined;
  isLoading: boolean;
  error: Error | null;
}

/** Aggregated funding rate data for both legs. */
export interface ArbitrageFundingRates {
  long: FundingRate | undefined;
  short: FundingRate | undefined;
  isLoading: boolean;
  error: Error | null;
}

/** Matched arbitrage data for this pair (both types). */
export interface ArbitrageData {
  funding: FundingArbitrage | null;
  spread: SpreadArbitrage | null;
  isLoading: boolean;
}

/** Positions for this pair, split by status. */
export interface ArbitragePositions {
  active: Position | null;
  history: Position[];
  all: Position[];
  isLoading: boolean;
  error: Error | null;
}

// Active position statuses
const ACTIVE_STATUSES = new Set([
  "pending",
  "opening",
  "open",
  "partially_open",
  "closing",
]);

// ---------------------------------------------------------------------------
// Individual hooks
// ---------------------------------------------------------------------------

export function useArbitrageTickers(pair: ArbitragePair | null): ArbitrageTickers {
  const longQuery = useQuery({
    queryKey: ["ticker", pair?.longExchange, pair?.symbol],
    queryFn: () => fetchTicker(pair!.longExchange, pair!.symbol),
    enabled: !!pair,
  });

  const shortQuery = useQuery({
    queryKey: ["ticker", pair?.shortExchange, pair?.symbol],
    queryFn: () => fetchTicker(pair!.shortExchange, pair!.symbol),
    enabled: !!pair,
  });

  return {
    long: longQuery.data,
    short: shortQuery.data,
    isLoading: longQuery.isLoading || shortQuery.isLoading,
    error: longQuery.error || shortQuery.error,
  };
}

export function useArbitrageFundingRates(
  pair: ArbitragePair | null
): ArbitrageFundingRates {
  const longQuery = useQuery({
    queryKey: ["funding-rate", pair?.longExchange, pair?.symbol],
    queryFn: () => fetchFundingRate(pair!.longExchange, pair!.symbol),
    enabled: !!pair,
  });

  const shortQuery = useQuery({
    queryKey: ["funding-rate", pair?.shortExchange, pair?.symbol],
    queryFn: () => fetchFundingRate(pair!.shortExchange, pair!.symbol),
    enabled: !!pair,
  });

  return {
    long: longQuery.data,
    short: shortQuery.data,
    isLoading: longQuery.isLoading || shortQuery.isLoading,
    error: longQuery.error || shortQuery.error,
  };
}

export function useArbitrageData(
  pair: ArbitragePair | null
): ArbitrageData {
  const fundingQuery = useQuery({
    queryKey: ["funding-arbitrage"],
    queryFn: fetchFundingArbitrage,
    enabled: !!pair,
  });

  const spreadQuery = useQuery({
    queryKey: ["spread-arbitrage"],
    queryFn: fetchSpreadArbitrage,
    enabled: !!pair,
  });

  const funding = useMemo(() => {
    if (!pair || !fundingQuery.data) return null;
    return (
      fundingQuery.data.find(
        (a) =>
          a.symbol === pair.symbol &&
          ((a.long.exchange_id === pair.longExchange &&
            a.short.exchange_id === pair.shortExchange) ||
           (a.long.exchange_id === pair.shortExchange &&
            a.short.exchange_id === pair.longExchange))
      ) ?? null
    );
  }, [pair, fundingQuery.data]);

  const spread = useMemo(() => {
    if (!pair || !spreadQuery.data) return null;
    return (
      spreadQuery.data.find(
        (a) =>
          a.symbol === pair.symbol &&
          ((a.buy_exchange_id === pair.longExchange &&
            a.sell_exchange_id === pair.shortExchange) ||
            (a.buy_exchange_id === pair.shortExchange &&
              a.sell_exchange_id === pair.longExchange))
      ) ?? null
    );
  }, [pair, spreadQuery.data]);

  return {
    funding,
    spread,
    isLoading: fundingQuery.isLoading || spreadQuery.isLoading,
  };
}

export function useArbitragePositions(
  pair: ArbitragePair | null
): ArbitragePositions {
  const query = useQuery({
    queryKey: ["positions", pair?.symbol],
    queryFn: () => fetchPositions({ symbol: pair!.symbol }),
    enabled: !!pair,
  });

  const { active, history, all } = useMemo(() => {
    if (!pair || !query.data) {
      return { active: null, history: [] as Position[], all: [] as Position[] };
    }

    // Filter to matching exchange pair
    const matched = query.data.filter(
      (p) =>
        p.long_exchange_id === pair.longExchange &&
        p.short_exchange_id === pair.shortExchange
    );

    const activePos =
      matched.find((p) => ACTIVE_STATUSES.has(p.status)) ?? null;

    const historyPos = matched
      .filter((p) => !ACTIVE_STATUSES.has(p.status))
      .sort(
        (a, b) =>
          new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      );

    return { active: activePos, history: historyPos, all: matched };
  }, [pair, query.data]);

  return {
    active,
    history,
    all,
    isLoading: query.isLoading,
    error: query.error,
  };
}

export function usePositionOrders(positionId: string | null) {
  return useQuery<ExchangeOrder[]>({
    queryKey: ["position-orders", positionId],
    queryFn: () => fetchPositionOrders(positionId!),
    enabled: !!positionId,
  });
}

export function usePositionFunding(positionId: string | null) {
  return useQuery<FundingPayment[]>({
    queryKey: ["position-funding", positionId],
    queryFn: () => fetchPositionFunding(positionId!),
    enabled: !!positionId,
  });
}
