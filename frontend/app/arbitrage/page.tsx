"use client";

import { useState, useCallback, Suspense } from "react";
import { useArbitrageTickers, useArbitrageFundingRates, useArbitrageData, useArbitragePositions, type ArbitragePair } from "@/hooks/use-arbitrage-data";
import { useArbitrageWorkspaces } from "@/hooks/use-arbitrage-workspaces";
import { useArbitrageWebSocket } from "@/hooks/use-arbitrage-websocket";
import { WorkspaceNav } from "@/components/arbitrage/workspace-nav";
import { ArbitrageTopBar, type MobileTab } from "@/components/arbitrage/arbitrage-top-bar";
import { MetricCards } from "@/components/arbitrage/metric-cards";
import { ActivePosition } from "@/components/arbitrage/active-position";
import { OpenPositionPanel } from "@/components/arbitrage/open-position-panel";
import { ArbitrageBottomTabs } from "@/components/arbitrage/arbitrage-bottom-tabs";
import { ArbitrageCharts } from "@/components/arbitrage/arbitrage-charts";
import { PairCommand, type PairSelection } from "@/components/pair-command";
import { Search, TrendingUp, ChevronRight, ChevronLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// ---------------------------------------------------------------------------
// ArbitrageContent — the actual page content
// ---------------------------------------------------------------------------

function ArbitrageContent() {
  const {
    workspaces,
    activeId,
    activeWorkspace,
    hydrated,
    selectWorkspace,
    addWorkspace,
    closeWorkspace,
    closeAndActivate,
    applyPairSelection,
  } = useArbitrageWorkspaces();

  const [initCommandOpen, setInitCommandOpen] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const [mobileTab, setMobileTab] = useState<MobileTab>("metrics");

  // Derived pair for data hooks
  const pair: ArbitragePair | null =
    activeWorkspace?.symbol &&
    activeWorkspace.longExchange &&
    activeWorkspace.shortExchange
      ? {
          symbol: activeWorkspace.symbol,
          longExchange: activeWorkspace.longExchange,
          shortExchange: activeWorkspace.shortExchange,
        }
      : null;

  // Data hooks
  const tickers = useArbitrageTickers(pair);
  const fundingRates = useArbitrageFundingRates(pair);
  const arbitrage = useArbitrageData(pair);
  const positions = useArbitragePositions(pair);

  // WebSocket for high-frequency updates — pushes data into React Query cache.
  const { status: wsStatus } = useArbitrageWebSocket(pair);

  const handlePairSelect = useCallback(
    (selection: PairSelection) => {
      if (!activeId) return;
      // If another workspace already has this exact pair, close the current
      // (newly-created blank) workspace and switch to the existing one.
      const existing = workspaces.find(
        (w) =>
          w.id !== activeId &&
          w.symbol === selection.symbol &&
          w.longExchange === selection.longExchange &&
          w.shortExchange === selection.shortExchange
      );
      if (existing) {
        closeAndActivate(activeId, existing.id);
      } else {
        applyPairSelection(activeId, selection);
      }
    },
    [activeId, workspaces, applyPairSelection, closeAndActivate]
  );

  const handleSwap = useCallback(() => {
    if (!activeId || !activeWorkspace?.symbol) return;
    applyPairSelection(activeId, {
      symbol: activeWorkspace.symbol,
      longExchange: activeWorkspace.shortExchange,
      shortExchange: activeWorkspace.longExchange,
    });
  }, [activeId, activeWorkspace, applyPairSelection]);

  const handleAddAndOpen = useCallback(() => {
    addWorkspace();
    setInitCommandOpen(true);
  }, [addWorkspace]);

  const hasPair = Boolean(pair);

  if (!hydrated) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="w-4 h-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0 overflow-hidden">
      {/* ------------------------------------------------------------------ */}
      {/* Left sidebar — workspace switcher (desktop only)                    */}
      {/* ------------------------------------------------------------------ */}
      <div className="hidden md:flex">
        <WorkspaceNav
          workspaces={workspaces}
          activeId={activeId}
          onSelect={selectWorkspace}
          onAdd={handleAddAndOpen}
          onClose={closeWorkspace}
          orientation="vertical"
        />
      </div>

      {/* ------------------------------------------------------------------ */}
      {/* Center — main content                                               */}
      {/* ------------------------------------------------------------------ */}
      <div className="flex-1 flex flex-col min-w-0 min-h-0 overflow-hidden">
        {/* Horizontal workspace bar — mobile only, sticky so vertical scroll can't push it away */}
        <div className="md:hidden sticky top-0 z-10">
          <WorkspaceNav
            workspaces={workspaces}
            activeId={activeId}
            onSelect={selectWorkspace}
            onAdd={handleAddAndOpen}
            onClose={closeWorkspace}
            orientation="horizontal"
          />
        </div>

        {/* Top bar — symbol + exchange display; mobile panel switcher inline */}
        <ArbitrageTopBar
          workspace={activeWorkspace}
          onPairSelect={handlePairSelect}
          onSwap={handleSwap}
          tickers={tickers}
          fundingRates={fundingRates}
          arbitrage={arbitrage}
          mobileTab={mobileTab}
          onMobileTabChange={setMobileTab}
        />

        {/* Scrollable center body */}
        <div className="flex-1 overflow-y-auto">
          {!hasPair ? (
            /* Empty state */
            <div className="flex flex-col items-center justify-center h-full gap-4 py-20">
              <div className="flex items-center justify-center w-12 h-12 rounded-2xl bg-muted/60 border">
                <TrendingUp className="h-5 w-5 text-muted-foreground" />
              </div>
              <div className="text-center space-y-1.5">
                <p className="text-sm font-medium">No pair selected</p>
                <p className="text-xs text-muted-foreground">
                  Press{" "}
                  <kbd className="inline-flex items-center gap-0.5 rounded border bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground">
                    ⌘K
                  </kbd>{" "}
                  to search for a trading pair
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setInitCommandOpen(true)}
              >
                <Search className="h-3.5 w-3.5 mr-1.5" />
                Select Pair
              </Button>
            </div>
          ) : (
            /* Main arbitrage content */
            <div className="p-3 md:p-4 space-y-4">
              {/* Mobile: Controls panel */}
              {mobileTab === "controls" && (
                <div className="md:hidden">
                  <OpenPositionPanel
                    symbol={pair!.symbol}
                    longExchange={pair!.longExchange}
                    shortExchange={pair!.shortExchange}
                  />
                </div>
              )}

              {/* Metrics content — hidden on mobile when Controls is active */}
              <div className={cn(mobileTab === "controls" ? "hidden md:block" : undefined)}>
                <MetricCards
                  tickers={tickers}
                  fundingRates={fundingRates}
                  longExchange={pair!.longExchange}
                  shortExchange={pair!.shortExchange}
                />

                <div className="mt-4">
                  <ArbitrageCharts pair={pair} />
                </div>

                <div className="mt-4">
                   <ActivePosition
                     position={positions.active}
                     isLoading={positions.isLoading}
                     funding={arbitrage.funding}
                   />
                </div>

                <div className="mt-4">
                  <ArbitrageBottomTabs
                    positions={positions.history}
                    allPositions={positions.all}
                    isLoading={positions.isLoading}
                  />
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* ------------------------------------------------------------------ */}
      {/* Right panel — position controls (desktop only, collapsible)         */}
      {/* ------------------------------------------------------------------ */}
      {hasPair && (
        <div className="relative hidden md:flex shrink-0">
          <button
            type="button"
            onClick={() => setRightCollapsed((v) => !v)}
            title={rightCollapsed ? "Expand panel" : "Collapse panel"}
            className={cn(
              "absolute left-0 top-1/2 -translate-x-full -translate-y-1/2 z-10",
              "flex items-center justify-center w-4 h-8 rounded-l-md",
              "border border-r-0 bg-card/80 hover:bg-muted text-muted-foreground hover:text-foreground",
              "transition-colors"
            )}
          >
            {rightCollapsed ? (
              <ChevronLeft className="h-3 w-3" />
            ) : (
              <ChevronRight className="h-3 w-3" />
            )}
          </button>

          {!rightCollapsed && (
            <div className="w-96 border-l overflow-y-auto bg-card/20 flex flex-col">
              <OpenPositionPanel
                symbol={pair!.symbol}
                longExchange={pair!.longExchange}
                shortExchange={pair!.shortExchange}
                embedded
              />
            </div>
          )}
        </div>
      )}

      {/* Initial pair picker for new blank workspaces */}
      <PairCommand
        open={initCommandOpen}
        onOpenChange={setInitCommandOpen}
        onSelect={(sel) => {
          handlePairSelect(sel);
          setInitCommandOpen(false);
        }}
        currentPair={
          activeWorkspace?.symbol && activeWorkspace?.longExchange && activeWorkspace?.shortExchange
            ? {
                symbol: activeWorkspace.symbol,
                longExchange: activeWorkspace.longExchange,
                shortExchange: activeWorkspace.shortExchange,
              }
            : undefined
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page — Suspense required because useArbitrageWorkspaces uses useSearchParams
// ---------------------------------------------------------------------------

export default function ArbitragePage() {
  return (
    <Suspense>
      <ArbitrageContent />
    </Suspense>
  );
}
