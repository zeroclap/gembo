"use client";

import { useCallback, useState, useSyncExternalStore } from "react";
import { useTheme } from "next-themes";
import { useQueryClient } from "@tanstack/react-query";
import { cn } from "@/lib/utils";
import { Plus, X, TrendingUp, Sun, Moon, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ExchangeBadge } from "@/components/arbitrage/shared";
import type { ArbitrageWorkspace } from "@/hooks/use-arbitrage-workspaces";

const emptySubscribe = () => () => {};
const getTrue = () => true;
const getFalse = () => false;

// ---------------------------------------------------------------------------
// WorkspaceNav — workspace switcher, vertical (sidebar) or horizontal (top bar)
// ---------------------------------------------------------------------------

interface WorkspaceNavProps {
  workspaces: ArbitrageWorkspace[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onAdd: () => void;
  onClose: (id: string) => void;
  /** Layout orientation. Defaults to "vertical" (left sidebar). */
  orientation?: "vertical" | "horizontal";
}

export function WorkspaceNav({
  workspaces,
  activeId,
  onSelect,
  onAdd,
  onClose,
  orientation = "vertical",
}: WorkspaceNavProps) {
  const handleClose = useCallback(
    (e: React.MouseEvent, id: string) => {
      e.stopPropagation();
      onClose(id);
    },
    [onClose]
  );

  const { theme, setTheme } = useTheme();
  const queryClient = useQueryClient();
  const mounted = useSyncExternalStore(emptySubscribe, getTrue, getFalse);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const isHorizontal = orientation === "horizontal";

  if (isHorizontal) {
    return (
      <nav className="flex flex-row items-center border-b bg-card/30">
        {/* Scrollable workspace list */}
        <div className="flex flex-row items-center gap-1.5 px-3 py-1.5 overflow-x-auto scrollbar-none flex-1 min-w-0">
          {workspaces.map((ws, idx) => (
            <WorkspaceButton
              key={ws.id}
              workspace={ws}
              index={idx}
              isActive={ws.id === activeId}
              canClose={true}
              orientation={orientation}
              onSelect={() => onSelect(ws.id)}
              onClose={(e) => handleClose(e, ws.id)}
            />
          ))}

          {/* Add new workspace */}
          <button
            type="button"
            onClick={onAdd}
            title="Add new workspace"
            className={cn(
              "group relative flex items-center justify-center rounded-xl transition-all duration-200 shrink-0",
              "border border-dashed border-border/60 text-muted-foreground",
              "hover:border-primary/40 hover:text-primary hover:bg-primary/5",
              "w-8 h-8 ml-1"
            )}
          >
            <Plus className="h-3.5 w-3.5" />
          </button>
        </div>

        {/* Pinned action buttons */}
        <div className="flex items-center gap-0.5 px-2 py-1.5 border-l shrink-0">
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => {
              setIsRefreshing(true);
              queryClient.invalidateQueries().then(() => {
                setIsRefreshing(false);
              });
            }}
            aria-label="Refresh data"
          >
            <RefreshCw
              className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")}
            />
          </Button>
          {mounted && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              aria-label="Toggle theme"
            >
              {theme === "dark" ? (
                <Sun className="h-3.5 w-3.5" />
              ) : (
                <Moon className="h-3.5 w-3.5" />
              )}
            </Button>
          )}
        </div>
      </nav>
    );
  }

  return (
    <nav className="flex flex-col items-center gap-1.5 py-3 px-1.5 h-full border-r bg-card/30">
      {workspaces.map((ws, idx) => (
        <WorkspaceButton
          key={ws.id}
          workspace={ws}
          index={idx}
          isActive={ws.id === activeId}
          canClose={true}
          orientation={orientation}
          onSelect={() => onSelect(ws.id)}
          onClose={(e) => handleClose(e, ws.id)}
        />
      ))}

      {/* Add new workspace */}
      <button
        type="button"
        onClick={onAdd}
        title="Add new workspace"
        className={cn(
          "group relative flex items-center justify-center rounded-xl transition-all duration-200 shrink-0",
          "border border-dashed border-border/60 text-muted-foreground",
          "hover:border-primary/40 hover:text-primary hover:bg-primary/5",
          "w-9 h-9 mt-1"
        )}
      >
        <Plus className="h-3.5 w-3.5" />
      </button>
    </nav>
  );
}

// ---------------------------------------------------------------------------
// WorkspaceButton — individual workspace slot
// ---------------------------------------------------------------------------

function WorkspaceButton({
  workspace,
  index,
  isActive,
  canClose,
  orientation,
  onSelect,
  onClose,
}: {
  workspace: ArbitrageWorkspace;
  index: number;
  isActive: boolean;
  canClose: boolean;
  orientation: "vertical" | "horizontal";
  onSelect: () => void;
  onClose: (e: React.MouseEvent) => void;
}) {
  const { symbol, longExchange, shortExchange } = workspace;

  // Derive short label: base asset (e.g. "BTC" from "BTC/USDT:USDT")
  const label = symbol
    ? symbol.split("/")[0] ?? String(index + 1)
    : String(index + 1);

  // Derive exchange color for the dot indicator
  const exchangeColorMap: Record<string, string> = {
    bybit: "bg-amber-500",
    okx: "bg-blue-500",
    gate: "bg-emerald-500",
    binance: "bg-yellow-500",
  };
  const longColor = exchangeColorMap[longExchange ?? ""] ?? "bg-muted-foreground";

  // Mock "active position" status — workspace is "live" when it's the active one
  // and has a fully configured pair. In production this would come from a real hook.
  const hasActiveTrade = isActive && Boolean(symbol && longExchange && shortExchange);

  const isHorizontal = orientation === "horizontal";

  // Horizontal buttons are slightly wider to show text label inline
  const btnSizeClass = isHorizontal ? "h-8 px-2.5 min-w-[3.5rem]" : "w-9 h-9";

  // Tooltip positioning: below for horizontal, right for vertical
  const tooltipClass = isHorizontal
    ? "top-full mt-2 left-1/2 -translate-x-1/2"
    : "left-full ml-2.5 top-1/2 -translate-y-1/2";

  // Active indicator: bottom bar for horizontal, left bar for vertical
  const activeIndicatorClass = isHorizontal
    ? "absolute bottom-0 left-1/2 -translate-x-1/2 h-0.5 w-4 rounded-t-full bg-primary-foreground/60"
    : "absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-4 rounded-r-full bg-primary-foreground/60";

  return (
    <div className="relative group/ws shrink-0">
      <button
        type="button"
        onClick={onSelect}
        title={symbol ? `${symbol} — ${longExchange} / ${shortExchange}` : `Workspace ${index + 1}`}
        className={cn(
          "relative flex items-center justify-center rounded-xl transition-all duration-200 overflow-hidden",
          isHorizontal ? "flex-row gap-1.5" : "flex-col",
          btnSizeClass,
          isActive
            ? "bg-primary text-primary-foreground shadow-sm"
            : "bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground border border-transparent hover:border-border/50"
        )}
      >
        {symbol ? (
          <>
            <span className="text-[0.5rem] font-bold leading-none tracking-tight uppercase">
              {label}
            </span>
            {/* Exchange indicator dots */}
            {longExchange && shortExchange && (
              <div className="flex items-center gap-0.5">
                <span
                  className={cn(
                    "h-1 w-1 rounded-full opacity-80",
                    isActive ? "bg-primary-foreground/70" : longColor
                  )}
                />
                <span
                  className={cn(
                    "h-1 w-1 rounded-full opacity-80",
                    isActive
                      ? "bg-primary-foreground/50"
                      : (exchangeColorMap[shortExchange ?? ""] ?? "bg-muted-foreground")
                  )}
                />
              </div>
            )}
          </>
        ) : (
          <TrendingUp className="h-3.5 w-3.5" />
        )}

        {/* Active indicator bar */}
        {isActive && <span className={activeIndicatorClass} />}
      </button>

      {/* "Active position" green pulse dot — bottom-right corner */}
      {hasActiveTrade && (
        <span
          title="Position active"
          className="absolute bottom-0 right-0 z-10 flex h-2.5 w-2.5 items-center justify-center"
        >
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" />
          <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-emerald-500" />
        </span>
      )}

      {/* Close button — hover on desktop, always visible on mobile (horizontal) */}
      {canClose && (
        <button
          type="button"
          onClick={onClose}
          title="Close workspace"
          className={cn(
            "absolute -top-1 -right-1 z-10 flex items-center justify-center",
            "w-3.5 h-3.5 rounded-full bg-background border border-border/60",
            "text-muted-foreground hover:text-destructive hover:border-destructive/40",
            isHorizontal
              ? "opacity-100"
              : "opacity-0 group-hover/ws:opacity-100 transition-opacity duration-150",
            "shadow-sm"
          )}
        >
          <X className="h-2 w-2" />
        </button>
      )}

      {/* Tooltip with full name */}
      {symbol && (
        <div
          className={cn(
            "absolute z-50 pointer-events-none",
            "opacity-0 group-hover/ws:opacity-100 transition-opacity duration-150",
            "whitespace-nowrap rounded-md border bg-popover px-2 py-1 shadow-md",
            tooltipClass
          )}
        >
          <div className="flex items-center gap-1.5">
            <span className="text-xs font-medium">{symbol}</span>
            {hasActiveTrade && (
              <span className="inline-flex items-center gap-0.5 rounded-full bg-emerald-500/15 px-1.5 py-0.5 text-[0.5rem] font-medium text-emerald-600 dark:text-emerald-400 uppercase tracking-wider">
                Live
              </span>
            )}
          </div>
          {longExchange && shortExchange && (
            <div className="flex items-center gap-1 mt-0.5">
              <ExchangeBadge exchangeId={longExchange} />
              <span className="text-[0.5rem] text-muted-foreground">/</span>
              <ExchangeBadge exchangeId={shortExchange} />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

