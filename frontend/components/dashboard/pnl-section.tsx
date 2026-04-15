"use client";

import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { TrendingUp, TrendingDown } from "lucide-react";

// ---------------------------------------------------------------------------
// Mock P&L data — replace with real API hooks when backend ready
// ---------------------------------------------------------------------------

interface PnlEntry {
  label: string;
  today: string;
  week: string;
  month: string;
  total: string;
  pair?: string;
  type: "arbitrage" | "funding" | "fees";
}

const MOCK_PNL: PnlEntry[] = [
  {
    label: "BTC/USDT",
    pair: "bybit / binance",
    type: "arbitrage",
    today: "+142.50",
    week: "+830.20",
    month: "+2,450.00",
    total: "+8,100.00",
  },
  {
    label: "ETH/USDT",
    pair: "bybit / okx",
    type: "arbitrage",
    today: "+38.00",
    week: "+290.10",
    month: "+910.50",
    total: "+3,200.00",
  },
  {
    label: "SOL/USDT",
    pair: "binance / deribit",
    type: "arbitrage",
    today: "-12.80",
    week: "+55.00",
    month: "+180.00",
    total: "+620.00",
  },
  {
    label: "Funding",
    type: "funding",
    today: "+24.10",
    week: "+168.70",
    month: "+512.30",
    total: "+1,840.00",
  },
  {
    label: "Fees",
    type: "fees",
    today: "-18.50",
    week: "-130.00",
    month: "-420.00",
    total: "-1,520.00",
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function PnlCell({ value }: { value: string }) {
  const num = parseFloat(value.replace(/[^0-9+\-.]/g, ""));
  const isPos = num > 0;
  const isNeg = num < 0;
  return (
    <span
      className={cn(
        "text-xs font-mono tabular-nums font-medium",
        isPos && "text-emerald-600 dark:text-emerald-400",
        isNeg && "text-red-600 dark:text-red-400",
        !isPos && !isNeg && "text-muted-foreground"
      )}
    >
      {isPos ? "+" : ""}${Math.abs(num).toLocaleString(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })}
    </span>
  );
}

function TypeBadge({ type }: { type: PnlEntry["type"] }) {
  const map: Record<PnlEntry["type"], string> = {
    arbitrage: "bg-primary/10 text-primary border-primary/20",
    funding: "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20",
    fees: "bg-muted text-muted-foreground border-border",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center px-1.5 py-0.5 rounded text-[0.55rem] font-medium uppercase tracking-wide border",
        map[type]
      )}
    >
      {type}
    </span>
  );
}

// ---------------------------------------------------------------------------
// PnlSection
// ---------------------------------------------------------------------------

interface PnlSectionProps {
  isLoading?: boolean;
}

export function PnlSection({ isLoading }: PnlSectionProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border bg-card/50 overflow-hidden">
        <div className="px-4 py-3 border-b">
          <Skeleton className="h-3 w-24" />
        </div>
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-2.5">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-16" />
          </div>
        ))}
      </div>
    );
  }

  const totalToday = MOCK_PNL.reduce(
    (sum, e) => sum + parseFloat(e.today.replace(/[^0-9+\-.]/g, "")),
    0
  );

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b">
        <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          P&amp;L Breakdown
        </span>
        <div className="flex items-center gap-1.5">
          {totalToday >= 0 ? (
            <TrendingUp className="h-3 w-3 text-emerald-500" />
          ) : (
            <TrendingDown className="h-3 w-3 text-red-500" />
          )}
          <span
            className={cn(
              "text-xs font-mono tabular-nums font-semibold",
              totalToday >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {totalToday >= 0 ? "+" : ""}$
            {Math.abs(totalToday).toFixed(2)}
          </span>
          <span className="text-[0.6rem] text-muted-foreground">today</span>
        </div>
      </div>

      {/* Column headers */}
      <div className="grid grid-cols-[1fr_auto_auto_auto_auto] items-center gap-4 px-4 py-1.5 border-b bg-muted/20">
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Pair / Type
        </div>
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground text-right w-16">
          Today
        </div>
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground text-right w-16 hidden sm:block">
          7d
        </div>
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground text-right w-16 hidden md:block">
          30d
        </div>
        <div className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground text-right w-16">
          All-time
        </div>
      </div>

      {/* Rows */}
      <div className="divide-y divide-border/40">
        {MOCK_PNL.map((entry, i) => (
          <div
            key={i}
            className="grid grid-cols-[1fr_auto_auto_auto_auto] items-center gap-4 px-4 py-2 hover:bg-muted/30 transition-colors"
          >
            {/* Label */}
            <div className="min-w-0">
              <div className="flex items-center gap-1.5">
                <span className="text-xs font-semibold truncate">
                  {entry.label}
                </span>
                <TypeBadge type={entry.type} />
              </div>
              {entry.pair && (
                <div className="text-[0.6rem] text-muted-foreground truncate mt-0.5">
                  {entry.pair}
                </div>
              )}
            </div>

            {/* Values */}
            <div className="text-right w-16">
              <PnlCell value={entry.today} />
            </div>
            <div className="text-right w-16 hidden sm:block">
              <PnlCell value={entry.week} />
            </div>
            <div className="text-right w-16 hidden md:block">
              <PnlCell value={entry.month} />
            </div>
            <div className="text-right w-16">
              <PnlCell value={entry.total} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
