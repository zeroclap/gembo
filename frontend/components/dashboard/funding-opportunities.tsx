"use client";

import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { TrendingUp } from "lucide-react";

// ---------------------------------------------------------------------------
// Mock funding opportunities — replace with real API hooks
// ---------------------------------------------------------------------------

interface FundingOpportunity {
  symbol: string;
  longExchange: string;
  shortExchange: string;
  netRate: string; // annualised
  nextFunding: string;
  spreadPct: string;
  confidence: "high" | "medium" | "low";
}

const MOCK_OPPORTUNITIES: FundingOpportunity[] = [
  {
    symbol: "BTC/USDT",
    longExchange: "bybit",
    shortExchange: "binance",
    netRate: "+14.20%",
    nextFunding: "3h 12m",
    spreadPct: "0.012%",
    confidence: "high",
  },
  {
    symbol: "ETH/USDT",
    longExchange: "bybit",
    shortExchange: "okx",
    netRate: "+9.80%",
    nextFunding: "3h 12m",
    spreadPct: "0.008%",
    confidence: "high",
  },
  {
    symbol: "SOL/USDT",
    longExchange: "binance",
    shortExchange: "okx",
    netRate: "+6.40%",
    nextFunding: "7h 32m",
    spreadPct: "0.025%",
    confidence: "medium",
  },
  {
    symbol: "ARB/USDT",
    longExchange: "binance",
    shortExchange: "bybit",
    netRate: "+4.10%",
    nextFunding: "1h 08m",
    spreadPct: "0.041%",
    confidence: "low",
  },
];

const CONFIDENCE_COLOR = {
  high: "text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border-emerald-500/20",
  medium: "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/20",
  low: "text-muted-foreground bg-muted border-border",
};

// ---------------------------------------------------------------------------
// FundingOpportunitiesSection
// ---------------------------------------------------------------------------

interface FundingOpportunitiesProps {
  isLoading?: boolean;
}

export function FundingOpportunities({ isLoading }: FundingOpportunitiesProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border bg-card/50 overflow-hidden">
        <div className="px-4 py-3 border-b">
          <Skeleton className="h-3 w-40" />
        </div>
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-2">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-12 ml-auto" />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b">
        <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Funding Opportunities
        </span>
        <div className="flex items-center gap-1">
          <TrendingUp className="h-3 w-3 text-primary" />
          <span className="text-[0.6rem] text-muted-foreground">
            {MOCK_OPPORTUNITIES.length} pairs tracked
          </span>
        </div>
      </div>

      {/* Column headers */}
      <div className="grid grid-cols-[1fr_auto_auto_auto] items-center gap-4 px-4 py-1.5 border-b bg-muted/20">
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Pair
        </div>
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground text-right w-16">
          APR
        </div>
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground text-right w-16 hidden sm:block">
          Next
        </div>
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground text-right w-16">
          Signal
        </div>
      </div>

      {/* Rows */}
      <div className="divide-y divide-border/40">
        {MOCK_OPPORTUNITIES.map((opp, i) => (
          <div
            key={i}
            className="grid grid-cols-[1fr_auto_auto_auto] items-center gap-4 px-4 py-2 hover:bg-muted/30 transition-colors"
          >
            {/* Symbol */}
            <div className="min-w-0">
              <div className="text-xs font-semibold truncate">{opp.symbol}</div>
              <div className="text-[0.6rem] text-muted-foreground truncate">
                {opp.longExchange} ↑ · {opp.shortExchange} ↓
              </div>
            </div>

            {/* APR */}
            <div className="text-right w-16">
              <span className="text-xs font-mono tabular-nums font-medium text-emerald-600 dark:text-emerald-400">
                {opp.netRate}
              </span>
            </div>

            {/* Next funding */}
            <div className="text-right w-16 hidden sm:block">
              <span className="text-xs font-mono tabular-nums text-muted-foreground">
                {opp.nextFunding}
              </span>
            </div>

            {/* Confidence */}
            <div className="text-right w-16">
              <span
                className={cn(
                  "inline-flex items-center px-1.5 py-0.5 rounded text-[0.55rem] font-medium uppercase tracking-wide border",
                  CONFIDENCE_COLOR[opp.confidence]
                )}
              >
                {opp.confidence}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
