"use client";

import { useState, useCallback, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { PairCommand, type PairSelection } from "@/components/pair-command";
import { MetricCards } from "@/components/monitor/metric-cards";
import { ActivePosition } from "@/components/monitor/active-position";
import { OpenPositionPanel } from "@/components/monitor/open-position-panel";
import { PositionHistory } from "@/components/monitor/position-history";
import { ActivityTimeline } from "@/components/monitor/activity-timeline";
import { LegLabel } from "@/components/monitor/shared";
import {
  useMonitorTickers,
  useMonitorFundingRates,
  useMonitorArbitrage,
  useMonitorPositions,
  type MonitorPair,
} from "@/hooks/use-monitor-data";
import { Search } from "lucide-react";
import { Button } from "@/components/ui/button";

// ---------------------------------------------------------------------------
// MonitorContent — the actual page content (needs useSearchParams)
// ---------------------------------------------------------------------------

function MonitorContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  // Parse pair from URL
  const symbol = searchParams.get("symbol") || "";
  const longExchange = searchParams.get("long") || "";
  const shortExchange = searchParams.get("short") || "";

  const pair: MonitorPair | null =
    symbol && longExchange && shortExchange
      ? { symbol, longExchange, shortExchange }
      : null;

  // Command palette state — open by default when no pair selected
  const [commandOpen, setCommandOpen] = useState(!pair);

  // Keyboard shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setCommandOpen(true);
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Handle pair selection
  const handlePairSelect = useCallback(
    (selection: PairSelection) => {
      const params = new URLSearchParams();
      params.set("symbol", selection.symbol);
      params.set("long", selection.longExchange);
      params.set("short", selection.shortExchange);
      router.push(`/monitor?${params.toString()}`);
    },
    [router]
  );

  // Data hooks
  const tickers = useMonitorTickers(pair);
  const fundingRates = useMonitorFundingRates(pair);
  const arbitrage = useMonitorArbitrage(pair);
  const positions = useMonitorPositions(pair);

  return (
    <>
      <PairCommand
        open={commandOpen}
        onOpenChange={setCommandOpen}
        onSelect={handlePairSelect}
        currentSymbol={pair?.symbol}
      />

      <main className="mx-auto max-w-screen-xl flex-1 px-4 py-4 sm:px-6 lg:px-8 space-y-4">
        {/* Page header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {pair ? (
              <>
                <h1 className="text-sm font-heading font-semibold">
                  {pair.symbol}
                </h1>
                <div className="flex items-center gap-2">
                  <LegLabel side="long" exchangeId={pair.longExchange} />
                  <span className="text-[0.625rem] text-muted-foreground">
                    /
                  </span>
                  <LegLabel side="short" exchangeId={pair.shortExchange} />
                </div>
              </>
            ) : (
              <h1 className="text-sm font-heading font-semibold text-muted-foreground">
                Select a pair to monitor
              </h1>
            )}
          </div>

          <Button
            variant="outline"
            size="sm"
            className="h-7 gap-1.5 text-xs text-muted-foreground"
            onClick={() => setCommandOpen(true)}
          >
            <Search className="h-3 w-3" />
            <span className="hidden sm:inline">Search pair</span>
            <kbd className="hidden sm:inline-flex h-4 items-center gap-0.5 rounded border bg-muted px-1 font-mono text-[0.625rem] text-muted-foreground">
              <span className="text-[0.625rem]">&#8984;</span>K
            </kbd>
          </Button>
        </div>

        {/* Empty state */}
        {!pair && (
          <div className="flex flex-col items-center justify-center py-20 gap-4">
            <div className="text-center space-y-2">
              <p className="text-sm text-muted-foreground">
                Press{" "}
                <kbd className="inline-flex items-center gap-0.5 rounded border bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground">
                  <span>&#8984;</span>K
                </kbd>{" "}
                to search for a trading pair
              </p>
              <p className="text-xs text-muted-foreground">
                Select a symbol, then choose a long/short exchange pair
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCommandOpen(true)}
            >
              <Search className="h-3.5 w-3.5 mr-1.5" />
              Select Pair
            </Button>
          </div>
        )}

        {/* Monitoring content — 2-column layout */}
        {pair && (
          <div className="grid grid-cols-1 lg:grid-cols-[1fr_280px] gap-4">
            {/* Left / center — metrics, active position, history, timeline */}
            <div className="space-y-4">
              {/* Metric cards */}
              <MetricCards
                tickers={tickers}
                fundingRates={fundingRates}
                arbitrage={arbitrage}
                longExchange={pair.longExchange}
                shortExchange={pair.shortExchange}
              />

              {/* Active position */}
              <ActivePosition
                position={positions.active}
                isLoading={positions.isLoading}
              />

              {/* Two-column: history + timeline */}
              <div className="grid grid-cols-1 xl:grid-cols-[1fr_280px] gap-4">
                <PositionHistory
                  positions={positions.history}
                  isLoading={positions.isLoading}
                />
                <ActivityTimeline
                  positions={positions.all}
                  isLoading={positions.isLoading}
                />
              </div>
            </div>

            {/* Right sidebar — open position form */}
            <div className="lg:sticky lg:top-4 lg:self-start">
              <OpenPositionPanel
                symbol={pair.symbol}
                longExchange={pair.longExchange}
                shortExchange={pair.shortExchange}
              />
            </div>
          </div>
        )}
      </main>
    </>
  );
}

// ---------------------------------------------------------------------------
// Page — wraps content in Suspense for useSearchParams
// ---------------------------------------------------------------------------

export default function MonitorPage() {
  return (
    <Suspense>
      <MonitorContent />
    </Suspense>
  );
}
