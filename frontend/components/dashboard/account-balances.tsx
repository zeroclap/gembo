"use client";

import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowUp, ArrowDown, Minus } from "lucide-react";

// ---------------------------------------------------------------------------
// Mock data shapes (replace with real API calls when backend is ready)
// ---------------------------------------------------------------------------

export interface AccountBalance {
  account: string;
  exchange: string;
  totalUsd: string;
  available: string;
  margin: string;
  unrealizedPnl: string;
  status: "connected" | "error" | "degraded";
}

const MOCK_BALANCES: AccountBalance[] = [
  {
    account: "Main",
    exchange: "bybit",
    totalUsd: "24,835.12",
    available: "12,400.00",
    margin: "10,200.00",
    unrealizedPnl: "+235.12",
    status: "connected",
  },
  {
    account: "Main",
    exchange: "binance",
    totalUsd: "31,500.00",
    available: "9,800.00",
    margin: "19,600.00",
    unrealizedPnl: "+1,100.00",
    status: "connected",
  },
  {
    account: "Alt",
    exchange: "okx",
    totalUsd: "8,210.55",
    available: "8,210.55",
    margin: "0.00",
    unrealizedPnl: "0.00",
    status: "connected",
  },
  {
    account: "Alt",
    exchange: "deribit",
    totalUsd: "3,004.20",
    available: "1,200.00",
    margin: "1,700.00",
    unrealizedPnl: "+104.20",
    status: "degraded",
  },
];

// ---------------------------------------------------------------------------
// AccountBalancesSection
// ---------------------------------------------------------------------------

interface BalanceRowProps {
  balance: AccountBalance;
}

function BalanceRow({ balance }: BalanceRowProps) {
  const pnl = parseFloat(balance.unrealizedPnl.replace(/[^0-9+\-.]/g, ""));
  const isPositive = pnl > 0;
  const isNegative = pnl < 0;

  const statusDot = {
    connected: "bg-emerald-500",
    error: "bg-red-500",
    degraded: "bg-amber-500",
  }[balance.status];

  return (
    <div className="flex items-center gap-3 px-4 py-2.5 hover:bg-muted/30 transition-colors">
      {/* Exchange + account */}
      <div className="flex items-center gap-2 w-36 shrink-0">
        <span className={cn("h-1.5 w-1.5 rounded-full shrink-0", statusDot)} />
        <div className="min-w-0">
          <div className="text-xs font-semibold capitalize truncate">
            {balance.exchange}
          </div>
          <div className="text-[0.6rem] text-muted-foreground truncate">
            {balance.account}
          </div>
        </div>
      </div>

      {/* Total */}
      <div className="flex-1 min-w-0">
        <div className="text-xs font-mono tabular-nums font-medium">
          ${balance.totalUsd}
        </div>
        <div className="text-[0.6rem] text-muted-foreground">total</div>
      </div>

      {/* Available */}
      <div className="flex-1 min-w-0 hidden sm:block">
        <div className="text-xs font-mono tabular-nums text-muted-foreground">
          ${balance.available}
        </div>
        <div className="text-[0.6rem] text-muted-foreground">available</div>
      </div>

      {/* Margin used */}
      <div className="flex-1 min-w-0 hidden md:block">
        <div className="text-xs font-mono tabular-nums text-muted-foreground">
          ${balance.margin}
        </div>
        <div className="text-[0.6rem] text-muted-foreground">margin</div>
      </div>

      {/* Unrealized P&L */}
      <div className="shrink-0 flex items-center gap-1">
        {isPositive && (
          <ArrowUp className="h-3 w-3 text-emerald-500" strokeWidth={2.5} />
        )}
        {isNegative && (
          <ArrowDown className="h-3 w-3 text-red-500" strokeWidth={2.5} />
        )}
        {!isPositive && !isNegative && (
          <Minus className="h-3 w-3 text-muted-foreground" strokeWidth={2} />
        )}
        <span
          className={cn(
            "text-xs font-mono tabular-nums font-medium",
            isPositive && "text-emerald-600 dark:text-emerald-400",
            isNegative && "text-red-600 dark:text-red-400",
            !isPositive && !isNegative && "text-muted-foreground"
          )}
        >
          ${Math.abs(pnl).toFixed(2)}
        </span>
      </div>
    </div>
  );
}

interface AccountBalancesProps {
  isLoading?: boolean;
}

export function AccountBalances({ isLoading }: AccountBalancesProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border bg-card/50 overflow-hidden">
        <div className="px-4 py-3 border-b">
          <Skeleton className="h-3 w-32" />
        </div>
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-2.5">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-3 w-20" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-12" />
          </div>
        ))}
      </div>
    );
  }

  const totalUsd = MOCK_BALANCES.reduce(
    (sum, b) => sum + parseFloat(b.totalUsd.replace(/,/g, "")),
    0
  );

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b">
        <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Cross-Exchange Balances
        </span>
        <div className="flex items-center gap-2">
          <span className="text-xs font-mono tabular-nums font-semibold">
            ${totalUsd.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          </span>
          <span className="text-[0.6rem] text-muted-foreground">total</span>
        </div>
      </div>

      {/* Column headers */}
      <div className="flex items-center gap-3 px-4 py-1.5 border-b bg-muted/20">
        <div className="w-36 shrink-0 text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Account
        </div>
        <div className="flex-1 text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Total
        </div>
        <div className="flex-1 hidden sm:block text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Available
        </div>
        <div className="flex-1 hidden md:block text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Margin
        </div>
        <div className="shrink-0 text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Unr. PnL
        </div>
      </div>

      {/* Rows */}
      <div className="divide-y divide-border/40">
        {MOCK_BALANCES.map((b, i) => (
          <BalanceRow key={i} balance={b} />
        ))}
      </div>
    </div>
  );
}
