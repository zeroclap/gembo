"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import type { PairSelection } from "@/components/pair-command";
import { fetchWorkspaces, saveWorkspaces } from "@/lib/api";
import type { RemoteWorkspace } from "@/lib/api";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ArbitrageWorkspace {
  id: string;
  symbol: string;
  longExchange: string;
  shortExchange: string;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeId() {
  return Math.random().toString(36).slice(2, 9);
}

function blankWorkspace(): ArbitrageWorkspace {
  return { id: makeId(), symbol: "", longExchange: "", shortExchange: "" };
}

const STORAGE_KEY = "arbitrage_workspaces_v1";
const ACTIVE_KEY = "arbitrage_active_workspace_v1";

function loadFromStorage(): ArbitrageWorkspace[] | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed) && parsed.length > 0) return parsed;
    return null;
  } catch {
    return null;
  }
}

function loadActiveFromStorage(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return localStorage.getItem(ACTIVE_KEY);
  } catch {
    return null;
  }
}

function saveToStorage(workspaces: ArbitrageWorkspace[], activeId: string | null) {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(workspaces));
    if (activeId) localStorage.setItem(ACTIVE_KEY, activeId);
    else localStorage.removeItem(ACTIVE_KEY);
  } catch {
    // ignore
  }
}

function toRemote(ws: ArbitrageWorkspace): RemoteWorkspace {
  return {
    id: ws.id,
    symbol: ws.symbol,
    long_exchange: ws.longExchange,
    short_exchange: ws.shortExchange,
  };
}

function fromRemote(ws: RemoteWorkspace): ArbitrageWorkspace {
  return {
    id: ws.id,
    symbol: ws.symbol,
    longExchange: ws.long_exchange,
    shortExchange: ws.short_exchange,
  };
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useArbitrageWorkspaces() {
  const router = useRouter();
  const searchParams = useSearchParams();

  // Whether we've hydrated from localStorage (avoid SSR mismatch)
  const [hydrated, setHydrated] = useState(false);

  // Initial blank state (SSR-safe)
  const [workspaces, setWorkspaces] = useState<ArbitrageWorkspace[]>([blankWorkspace()]);
  const [activeId, setActiveId] = useState<string | null>(null);

  // Skip the first sync-to-URL (we read FROM URL on first mount)
  const skipNextSync = useRef(true);

  // Debounce timer for remote save
  const remoteSaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Hydrate from backend (preferred) or localStorage + URL on client
  useEffect(() => {
    const urlSymbol = searchParams.get("symbol") || "";
    const urlLong = searchParams.get("long") || "";
    const urlShort = searchParams.get("short") || "";

    async function hydrate() {
      // Try fetching from backend first; fall back to localStorage on error.
      let stored: ArbitrageWorkspace[] | null = null;
      try {
        const remote = await fetchWorkspaces();
        if (remote.length > 0) {
          stored = remote.map(fromRemote);
        }
      } catch {
        // backend unavailable or unauthenticated — fall back to localStorage
      }

      if (!stored) {
        stored = loadFromStorage();
      }

      const storedActive = loadActiveFromStorage();

      if (stored) {
        setWorkspaces(stored);
        if (urlSymbol && urlLong && urlShort) {
          const match = stored.find(
            (w) =>
              w.symbol === urlSymbol &&
              w.longExchange === urlLong &&
              w.shortExchange === urlShort
          );
          if (match) {
            setActiveId(match.id);
          } else {
            const newWs: ArbitrageWorkspace = {
              id: makeId(),
              symbol: urlSymbol,
              longExchange: urlLong,
              shortExchange: urlShort,
            };
            setWorkspaces((prev) => [...prev, newWs]);
            setActiveId(newWs.id);
          }
        } else if (storedActive && stored.find((w) => w.id === storedActive)) {
          setActiveId(storedActive);
        } else {
          setActiveId(stored[0].id);
        }
      } else {
        if (urlSymbol && urlLong && urlShort) {
          const ws: ArbitrageWorkspace = {
            id: makeId(),
            symbol: urlSymbol,
            longExchange: urlLong,
            shortExchange: urlShort,
          };
          setWorkspaces([ws]);
          setActiveId(ws.id);
        } else {
          const ws = blankWorkspace();
          setWorkspaces([ws]);
          setActiveId(ws.id);
        }
      }

      setHydrated(true);
    }

    hydrate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Derived: active workspace
  const activeWorkspace = workspaces.find((w) => w.id === activeId) ?? null;

  // Sync URL whenever active workspace changes (but not on first render)
  useEffect(() => {
    if (!hydrated) return;
    if (skipNextSync.current) {
      skipNextSync.current = false;
      return;
    }
    const ws = workspaces.find((w) => w.id === activeId);
    if (ws && ws.symbol && ws.longExchange && ws.shortExchange) {
      const params = new URLSearchParams();
      params.set("symbol", ws.symbol);
      params.set("long", ws.longExchange);
      params.set("short", ws.shortExchange);
      router.replace(`/arbitrage?${params.toString()}`);
    } else {
      router.replace("/arbitrage");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId, workspaces, hydrated]);

  // Persist to localStorage + backend (debounced) whenever state changes
  useEffect(() => {
    if (!hydrated) return;
    saveToStorage(workspaces, activeId);

    // Debounce remote save to avoid hammering the API on rapid updates
    if (remoteSaveTimer.current) clearTimeout(remoteSaveTimer.current);
    remoteSaveTimer.current = setTimeout(() => {
      saveWorkspaces(workspaces.map(toRemote)).catch(() => {
        // ignore remote save errors silently
      });
    }, 500);
  }, [workspaces, activeId, hydrated]);

  // ---------------------------------------------------------------------------
  // Actions
  // ---------------------------------------------------------------------------

  const selectWorkspace = useCallback((id: string) => {
    setActiveId(id);
  }, []);

  const addWorkspace = useCallback(() => {
    const ws = blankWorkspace();
    setWorkspaces((prev) => [...prev, ws]);
    setActiveId(ws.id);
  }, []);

  const closeWorkspace = useCallback(
    (id: string) => {
      setWorkspaces((prev) => {
        const next = prev.filter((w) => w.id !== id);
        // Always keep at least one
        if (next.length === 0) {
          const fresh = blankWorkspace();
          if (id === activeId) setActiveId(fresh.id);
          return [fresh];
        }
        // If we're closing the active one, pick adjacent
        if (id === activeId) {
          const closedIdx = prev.findIndex((w) => w.id === id);
          const newActive = next[Math.min(closedIdx, next.length - 1)];
          setActiveId(newActive.id);
        }
        return next;
      });
    },
    [activeId]
  );

  const updateWorkspace = useCallback(
    (id: string, patch: Partial<Omit<ArbitrageWorkspace, "id">>) => {
      setWorkspaces((prev) =>
        prev.map((w) => (w.id === id ? { ...w, ...patch } : w))
      );
    },
    []
  );

  const applyPairSelection = useCallback(
    (id: string, selection: PairSelection) => {
      updateWorkspace(id, {
        symbol: selection.symbol,
        longExchange: selection.longExchange,
        shortExchange: selection.shortExchange,
      });
    },
    [updateWorkspace]
  );

  return {
    workspaces,
    activeId,
    activeWorkspace,
    hydrated,
    selectWorkspace,
    addWorkspace,
    closeWorkspace,
    updateWorkspace,
    applyPairSelection,
  };
}
