"use client";

import { useState, useCallback, useEffect } from "react";
import { cn } from "@/lib/utils";
import { PairCommand, type PairSelection } from "@/components/pair-command";
import { ExchangeStatusRow } from "@/components/arbitrage/metric-cards";
import { ChevronDown, BarChart2, SlidersHorizontal } from "lucide-react";
import type { ArbitrageWorkspace } from "@/hooks/use-arbitrage-workspaces";
import type { ArbitrageTickers, ArbitrageFundingRates, ArbitrageData } from "@/hooks/use-arbitrage-data";

// ---------------------------------------------------------------------------
// SymbolButton — click to open symbol picker
// ---------------------------------------------------------------------------

function SymbolButton({
  symbol,
  onClick,
}: {
  symbol: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "group flex items-center gap-1.5 px-2 py-1.5 rounded-md transition-colors min-w-0",
        "hover:bg-muted/60",
        symbol ? "text-foreground" : "text-muted-foreground"
      )}
    >
      <span className="text-base font-mono font-semibold tabular-nums leading-none">
        {symbol || "Select symbol"}
      </span>
      <ChevronDown className="h-3 w-3 text-muted-foreground group-hover:text-foreground transition-colors shrink-0" />
    </button>
  );
}

// ---------------------------------------------------------------------------
// SpreadMiniBar — compact funding + price spread summary below symbol button
// ---------------------------------------------------------------------------

function SpreadMiniBar({
  tickers,
  fundingRates,
  arbitrage,
}: {
  tickers?: ArbitrageTickers;
  fundingRates?: ArbitrageFundingRates;
  arbitrage?: ArbitrageData;
}) {
  // Funding spread: short rate − long rate, normalised to 8h
  const longRate = parseFloat(fundingRates?.long?.rate ?? arbitrage?.funding?.long.funding_rate ?? "0");
  const shortRate = parseFloat(fundingRates?.short?.rate ?? arbitrage?.funding?.short.funding_rate ?? "0");
  const longInterval = fundingRates?.long?.interval_hours ?? 8;
  const shortInterval = fundingRates?.short?.interval_hours ?? 8;
  const longRate8h = longRate * (8 / longInterval);
  const shortRate8h = shortRate * (8 / shortInterval);
  const hasRates = fundingRates?.long || fundingRates?.short || arbitrage?.funding;
  const netRate8h = hasRates ? shortRate8h - longRate8h : null;
  const fundingStr = netRate8h !== null
    ? `${netRate8h >= 0 ? "+" : ""}${(netRate8h * 100).toFixed(4)}%`
    : "—";

  // Price spread
  const longAsk = parseFloat(tickers?.long?.ask ?? "0");
  const shortBid = parseFloat(tickers?.short?.bid ?? "0");
  const midPrice = longAsk && shortBid ? (longAsk + shortBid) / 2 : 0;
  const rawSpreadPct = midPrice > 0 ? ((shortBid - longAsk) / midPrice) * 100 : null;
  const spreadPct = arbitrage?.spread ? parseFloat(arbitrage.spread.spread_pct) : rawSpreadPct;
  const spreadStr = spreadPct !== null
    ? `${spreadPct >= 0 ? "+" : ""}${spreadPct.toFixed(4)}%`
    : "—";

  return (
    <div className="flex flex-col md:flex-row md:items-center md:gap-2 md:px-2 md:pb-1 px-2 leading-none">
      <span className="flex items-baseline gap-1">
        <span className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">F.Spread</span>
        <span className={cn(
          "text-[0.7rem] font-mono tabular-nums font-medium",
          netRate8h === null ? "text-muted-foreground" :
          netRate8h > 0 ? "text-emerald-600 dark:text-emerald-400" :
          netRate8h < 0 ? "text-red-600 dark:text-red-400" : "text-foreground"
        )}>{fundingStr}</span>
      </span>
      <span className="hidden md:inline text-border/60 text-xs select-none">·</span>
      <span className="flex items-baseline gap-1">
        <span className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">P.Spread</span>
        <span className={cn(
          "text-[0.7rem] font-mono tabular-nums font-medium",
          spreadPct === null ? "text-muted-foreground" :
          spreadPct > 0 ? "text-emerald-600 dark:text-emerald-400" :
          spreadPct < 0 ? "text-red-600 dark:text-red-400" : "text-foreground"
        )}>{spreadStr}</span>
      </span>
    </div>
  );
}



export type MobileTab = "metrics" | "controls";

function MobilePanelSwitch({
  value,
  onChange,
}: {
  value: MobileTab;
  onChange: (v: MobileTab) => void;
}) {
  return (
    <div className="flex items-center rounded-md border bg-muted/40 p-0.5 gap-0.5 shrink-0">
      <button
        type="button"
        onClick={() => onChange("metrics")}
        title="Metrics"
        className={cn(
          "flex items-center justify-center w-7 h-7 rounded-sm transition-colors",
          value === "metrics"
            ? "bg-background text-foreground shadow-sm border border-border/50"
            : "text-muted-foreground hover:text-foreground"
        )}
      >
        <BarChart2 className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        onClick={() => onChange("controls")}
        title="Controls"
        className={cn(
          "flex items-center justify-center w-7 h-7 rounded-sm transition-colors",
          value === "controls"
            ? "bg-background text-foreground shadow-sm border border-border/50"
            : "text-muted-foreground hover:text-foreground"
        )}
      >
        <SlidersHorizontal className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ArbitrageTopBar
// ---------------------------------------------------------------------------

interface ArbitrageTopBarProps {
  workspace: ArbitrageWorkspace | null;
  onPairSelect: (selection: PairSelection) => void;
  tickers?: ArbitrageTickers;
  fundingRates?: ArbitrageFundingRates;
  arbitrage?: ArbitrageData;
  /** Mobile panel tab state — passed in from page so it can control content */
  mobileTab?: MobileTab;
  onMobileTabChange?: (tab: MobileTab) => void;
}

export function ArbitrageTopBar({
  workspace,
  onPairSelect,
  tickers,
  fundingRates,
  arbitrage,
  mobileTab,
  onMobileTabChange,
}: ArbitrageTopBarProps) {
  const [commandOpen, setCommandOpen] = useState(false);

  const handlePairSelect = useCallback(
    (selection: PairSelection) => {
      onPairSelect(selection);
      setCommandOpen(false);
    },
    [onPairSelect]
  );

  // Keyboard shortcut ⌘K
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setCommandOpen(true);
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, []);

  const symbol = workspace?.symbol ?? "";
  const longExchange = workspace?.longExchange ?? "";
  const shortExchange = workspace?.shortExchange ?? "";

  const hasPair = Boolean(symbol && longExchange && shortExchange);
  const showMobileSwitch = hasPair && mobileTab !== undefined && onMobileTabChange !== undefined;

  return (
    <>
      <PairCommand
        open={commandOpen}
        onOpenChange={setCommandOpen}
        onSelect={handlePairSelect}
        currentPair={hasPair ? { symbol, longExchange, shortExchange } : undefined}
      />

      <div className="flex items-center gap-2 px-3 border-b bg-card/20 md:py-0 py-2">
        {/* Symbol button + spread values: side-by-side on mobile, stacked on desktop */}
        <div className="flex items-center gap-2 md:flex-col md:items-start md:gap-0">
          <SymbolButton
            symbol={symbol}
            onClick={() => setCommandOpen(true)}
          />
          {hasPair && (
            <SpreadMiniBar
              tickers={tickers}
              fundingRates={fundingRates}
              arbitrage={arbitrage}
            />
          )}
        </div>

        {/* Desktop: exchange status rows stacked, right of symbol button */}
        {hasPair && (
          <div className="hidden md:flex flex-col divide-y divide-border/40 ml-1 border-l border-border/40">
            <ExchangeStatusRow
              exchangeId={longExchange}
              side="long"
              ticker={tickers?.long}
              fundingRate={fundingRates?.long}
            />
            <ExchangeStatusRow
              exchangeId={shortExchange}
              side="short"
              ticker={tickers?.short}
              fundingRate={fundingRates?.short}
            />
          </div>
        )}

        {/* Mobile panel switcher — right-aligned, only on mobile when pair selected */}
        {showMobileSwitch && (
          <div className="ml-auto md:hidden">
            <MobilePanelSwitch
              value={mobileTab!}
              onChange={onMobileTabChange!}
            />
          </div>
        )}
      </div>
    </>
  );
}
