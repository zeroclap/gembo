"use client";

import { useState } from "react";
import {
  Wallet,
  TrendingUp,
  Activity,
  DollarSign,
  Zap,
  RefreshCw,
  LayoutDashboard,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { StatCard } from "@/components/dashboard/stat-card";
import { AccountBalances } from "@/components/dashboard/account-balances";
import { PnlSection } from "@/components/dashboard/pnl-section";
import { OngoingActivities } from "@/components/dashboard/ongoing-activities";
import { SystemHealth } from "@/components/dashboard/system-health";
import { FundingOpportunities } from "@/components/dashboard/funding-opportunities";

// ---------------------------------------------------------------------------
// Dashboard summary stats — mock data, replace with real API hooks
// ---------------------------------------------------------------------------

const SUMMARY_STATS = [
  {
    label: "Total Portfolio",
    value: "$67,549.87",
    trendValue: "+2.4%",
    trend: "up" as const,
    sub: "Across 4 exchange accounts",
    icon: Wallet,
    accent: "default" as const,
  },
  {
    label: "Today's PnL",
    value: "+$173.30",
    trendValue: "+$142.50 arb · +$24.10 fund",
    trend: "up" as const,
    sub: "Net of fees (−$18.50)",
    icon: TrendingUp,
    accent: "emerald" as const,
  },
  {
    label: "Active Positions",
    value: "2",
    trendValue: "1 closing",
    trend: "neutral" as const,
    sub: "$7,000 deployed",
    icon: Activity,
    accent: "default" as const,
  },
  {
    label: "Open P&L",
    value: "+$169.70",
    trendValue: "+$142.50 · +$38.00",
    trend: "up" as const,
    sub: "Unrealized across all positions",
    icon: DollarSign,
    accent: "emerald" as const,
  },
  {
    label: "Best APR",
    value: "14.2%",
    trendValue: "BTC/USDT bybit↑/binance↓",
    trend: "up" as const,
    sub: "Annualized funding rate",
    icon: Zap,
    accent: "blue" as const,
  },
];

// ---------------------------------------------------------------------------
// DashboardPage
// ---------------------------------------------------------------------------

export default function DashboardPage() {
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefresh = () => {
    setIsRefreshing(true);
    setTimeout(() => setIsRefreshing(false), 1200);
  };

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="mx-auto w-full max-w-screen-xl px-4 py-6 flex flex-col gap-6">
        {/* Page header */}
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-2">
              <LayoutDashboard className="h-4 w-4 text-muted-foreground" />
              <h1 className="text-sm font-semibold">Dashboard</h1>
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              Portfolio overview, open positions, and system status.
            </p>
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground"
            onClick={handleRefresh}
          >
            <RefreshCw
              className={cn("h-3 w-3 mr-1.5", isRefreshing && "animate-spin")}
            />
            Refresh
          </Button>
        </div>

        {/* Summary stat cards */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
          {SUMMARY_STATS.map((stat) => (
            <StatCard key={stat.label} {...stat} />
          ))}
        </div>

        {/* Main two-column grid */}
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-4">
          {/* Left column — data tables */}
          <div className="flex flex-col gap-4 min-w-0">
            <AccountBalances />
            <PnlSection />
            <OngoingActivities />
          </div>

          {/* Right column — system + opportunities */}
          <div className="flex flex-col gap-4">
            <SystemHealth />
            <FundingOpportunities />
          </div>
        </div>
      </div>
    </div>
  );
}
