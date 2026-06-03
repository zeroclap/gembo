"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { ArbitragePair } from "@/hooks/use-arbitrage-data";
import type { FundingArbitrage, SpreadArbitrage, TickerSnapshot, TickerSnapshotHistory } from "@/lib/api";

/** Connection status for UI indicators. */
export type WSStatus = "connecting" | "connected" | "disconnected";

interface WSMessage {
  type: string;
  data: unknown;
}

interface ArbitrageUpdate {
  funding: FundingArbitrage | null;
  spread: SpreadArbitrage | null;
}

function getWSUrl(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  // Connect directly to Go backend (Next.js rewrites don't support WS).
  // In development, the API runs on port 8080.
  const host =
    process.env.NEXT_PUBLIC_WS_HOST ?? `${window.location.hostname}:8080`;
  return `${proto}//${host}/v1/arbitrage/ws`;
}

/**
 * useArbitrageWebSocket establishes a WebSocket connection to receive
 * real-time arbitrage updates for a pair. It syncs incoming data into
 * React Query cache so all existing consumers stay up to date.
 *
 * New message types handled:
 *   - "ticker_snapshot_history" → seeded into ["ticker-snapshot-history", symbol, exA, exB]
 *   - "ticker_snapshot"         → latest per-exchange snapshot in ["ticker-snapshot-live", exchangeId, symbol]
 */
export function useArbitrageWebSocket(pair: ArbitragePair | null) {
  const queryClient = useQueryClient();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [status, setStatus] = useState<WSStatus>("disconnected");

  // Stable ref to current pair for use inside callbacks.
  const pairRef = useRef(pair);
  pairRef.current = pair;

  // Stable ref for the connect function so onclose can reference it.
  const connectRef = useRef<() => void>(undefined);

  const sendSubscribe = useCallback((ws: WebSocket, p: ArbitragePair) => {
    const msg = JSON.stringify({
      type: "subscribe",
      data: {
        symbol: p.symbol,
        exchange_a: p.longExchange,
        exchange_b: p.shortExchange,
      },
    });
    ws.send(msg);
  }, []);

  const handleMessage = useCallback(
    (event: MessageEvent) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        const p = pairRef.current;
        if (!p) return;

        if (msg.type === "arbitrage") {
          const data = msg.data as ArbitrageUpdate;
          if (data.funding) {
            queryClient.setQueryData<FundingArbitrage[]>(
              ["funding-arbitrage"],
              (old) => {
                if (!old) return [data.funding!];
                const idx = old.findIndex(
                  (a) =>
                    a.symbol === p.symbol &&
                    a.long.exchange_id === p.longExchange &&
                    a.short.exchange_id === p.shortExchange
                );
                if (idx >= 0) {
                  const next = [...old];
                  next[idx] = data.funding!;
                  return next;
                }
                return [...old, data.funding!];
              }
            );
          }
          if (data.spread) {
            queryClient.setQueryData<SpreadArbitrage[]>(
              ["spread-arbitrage"],
              (old) => {
                if (!old) return [data.spread!];
                const idx = old.findIndex(
                  (a) =>
                    a.symbol === p.symbol &&
                    ((a.buy_exchange_id === p.longExchange &&
                      a.sell_exchange_id === p.shortExchange) ||
                      (a.buy_exchange_id === p.shortExchange &&
                        a.sell_exchange_id === p.longExchange))
                );
                if (idx >= 0) {
                  const next = [...old];
                  next[idx] = data.spread!;
                  return next;
                }
                return [...old, data.spread!];
              }
            );
          }
          return;
        }

        if (msg.type === "ticker_snapshot_history") {
          const history = msg.data as TickerSnapshotHistory;
          queryClient.setQueryData(
            ["ticker-snapshot-history", p.symbol, p.longExchange, p.shortExchange],
            history
          );
          return;
        }

        if (msg.type === "ticker_snapshot") {
          const snap = msg.data as TickerSnapshot;
          // Store latest snapshot per exchange so chart hooks can read it.
          queryClient.setQueryData(
            ["ticker-snapshot-live", snap.exchange_id, snap.symbol],
            snap
          );
          return;
        }
      } catch {
        // Ignore malformed messages.
      }
    },
    [queryClient]
  );

  connectRef.current = () => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const p = pairRef.current;
    if (!p) return;

    setStatus("connecting");
    const ws = new WebSocket(getWSUrl());
    wsRef.current = ws;

    ws.onopen = () => {
      setStatus("connected");
      sendSubscribe(ws, p);
    };

    ws.onmessage = handleMessage;

    ws.onclose = () => {
      setStatus("disconnected");
      wsRef.current = null;
      if (pairRef.current) {
        reconnectTimer.current = setTimeout(() => connectRef.current?.(), 2000);
      }
    };

    ws.onerror = () => {
      ws.close();
    };
  };

  // Connect/reconnect when pair changes.
  useEffect(() => {
    if (!pair) {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      clearTimeout(reconnectTimer.current);
      setStatus("disconnected");
      return;
    }

    // If already connected, just re-subscribe.
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      sendSubscribe(wsRef.current, pair);
      return;
    }

    connectRef.current?.();

    return () => {
      clearTimeout(reconnectTimer.current);
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pair?.symbol, pair?.longExchange, pair?.shortExchange]);

  return { status };
}
