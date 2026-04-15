"use client";

import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ArrowUpRight,
  ArrowDownRight,
  Zap,
  RotateCcw,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Mock activity data — replace with real API hooks when backend ready
// ---------------------------------------------------------------------------

type ActivityStatus = "running" | "pending" | "completed" | "failed" | "closing";
type ActivityType = "open" | "close" | "rebalance" | "funding_collect";

interface Activity {
  id: string;
  type: ActivityType;
  symbol: string;
  longExchange: string;
  shortExchange: string;
  status: ActivityStatus;
  size: string;
  pnl?: string;
  duration: string;
  ts: string;
}

const MOCK_ACTIVITIES: Activity[] = [
  {
    id: "act_001",
    type: "open",
    symbol: "BTC/USDT",
    longExchange: "bybit",
    shortExchange: "binance",
    status: "running",
    size: "$5,000",
    pnl: "+$142.50",
    duration: "2h 14m",
    ts: "Active",
  },
  {
    id: "act_002",
    type: "open",
    symbol: "ETH/USDT",
    longExchange: "bybit",
    shortExchange: "okx",
    status: "running",
    size: "$2,000",
    pnl: "+$38.00",
    duration: "45m",
    ts: "Active",
  },
  {
    id: "act_003",
    type: "close",
    symbol: "SOL/USDT",
    longExchange: "binance",
    shortExchange: "deribit",
    status: "closing",
    size: "$1,500",
    pnl: "-$12.80",
    duration: "3h 02m",
    ts: "1m ago",
  },
  {
    id: "act_004",
    type: "rebalance",
    symbol: "ETH/USDT",
    longExchange: "okx",
    shortExchange: "binance",
    status: "completed",
    size: "$800",
    pnl: "+$4.20",
    duration: "8m",
    ts: "12m ago",
  },
  {
    id: "act_005",
    type: "open",
    symbol: "ARB/USDT",
    longExchange: "binance",
    shortExchange: "bybit",
    status: "failed",
    size: "$1,200",
    duration: "2m",
    ts: "38m ago",
  },
  {
    id: "act_006",
    type: "funding_collect",
    symbol: "BTC/USDT",
    longExchange: "bybit",
    shortExchange: "binance",
    status: "completed",
    size: "-",
    pnl: "+$24.10",
    duration: "-",
    ts: "1h ago",
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const STATUS_CONFIG: Record<ActivityStatus, { label: string; color: string; dot: string }> = {
  running: {
    label: "Running",
    color: "text-emerald-600 dark:text-emerald-400",
    dot: "bg-emerald-500 animate-pulse",
  },
  pending: {
    label: "Pending",
    color: "text-amber-600 dark:text-amber-400",
    dot: "bg-amber-500",
  },
  completed: {
    label: "Done",
    color: "text-muted-foreground",
    dot: "bg-muted-foreground/50",
  },
  failed: {
    label: "Failed",
    color: "text-red-600 dark:text-red-400",
    dot: "bg-red-500",
  },
  closing: {
    label: "Closing",
    color: "text-amber-600 dark:text-amber-400",
    dot: "bg-amber-500 animate-pulse",
  },
};

const TYPE_CONFIG: Record<ActivityType, { label: string; icon: typeof Zap }> = {
  open: { label: "Open", icon: ArrowUpRight },
  close: { label: "Close", icon: ArrowDownRight },
  rebalance: { label: "Rebalance", icon: RotateCcw },
  funding_collect: { label: "Funding", icon: Zap },
};

function ActivityRow({ activity }: { activity: Activity }) {
  const status = STATUS_CONFIG[activity.status];
  const typeConfig = TYPE_CONFIG[activity.type];
  const TypeIcon = typeConfig.icon;

  const pnlNum = activity.pnl
    ? parseFloat(activity.pnl.replace(/[^0-9+\-.]/g, ""))
    : null;

  return (
    <div className="flex items-center gap-3 px-4 py-2 hover:bg-muted/30 transition-colors">
      {/* Type icon */}
      <div className="flex items-center justify-center h-6 w-6 rounded-md bg-muted/60 shrink-0">
        <TypeIcon className="h-3 w-3 text-muted-foreground" strokeWidth={2} />
      </div>

      {/* Symbol + exchanges */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <span className="text-xs font-semibold">{activity.symbol}</span>
          <span className="text-[0.6rem] text-muted-foreground uppercase tracking-wide">
            {typeConfig.label}
          </span>
        </div>
        <div className="text-[0.6rem] text-muted-foreground truncate">
          {activity.longExchange} ↑ · {activity.shortExchange} ↓
        </div>
      </div>

      {/* Size */}
      <div className="hidden sm:block shrink-0 text-right">
        <div className="text-xs font-mono tabular-nums text-muted-foreground">
          {activity.size}
        </div>
        <div className="text-[0.6rem] text-muted-foreground">size</div>
      </div>

      {/* P&L */}
      <div className="shrink-0 text-right w-16">
        {pnlNum !== null ? (
          <span
            className={cn(
              "text-xs font-mono tabular-nums font-medium",
              pnlNum > 0 && "text-emerald-600 dark:text-emerald-400",
              pnlNum < 0 && "text-red-600 dark:text-red-400",
              pnlNum === 0 && "text-muted-foreground"
            )}
          >
            {pnlNum > 0 ? "+" : ""}${Math.abs(pnlNum).toFixed(2)}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        )}
      </div>

      {/* Duration */}
      <div className="hidden md:block shrink-0 text-right w-14">
        <span className="text-xs font-mono tabular-nums text-muted-foreground">
          {activity.duration}
        </span>
      </div>

      {/* Status */}
      <div className="shrink-0 flex items-center gap-1.5 w-20 justify-end">
        <span className={cn("h-1.5 w-1.5 rounded-full shrink-0", status.dot)} />
        <span className={cn("text-xs font-medium", status.color)}>
          {status.label}
        </span>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// OngoingActivities
// ---------------------------------------------------------------------------

interface OngoingActivitiesProps {
  isLoading?: boolean;
}

export function OngoingActivities({ isLoading }: OngoingActivitiesProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border bg-card/50 overflow-hidden">
        <div className="px-4 py-3 border-b">
          <Skeleton className="h-3 w-36" />
        </div>
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-2.5">
            <Skeleton className="h-6 w-6 rounded-md" />
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-3 w-16 ml-auto" />
          </div>
        ))}
      </div>
    );
  }

  const running = MOCK_ACTIVITIES.filter(
    (a) => a.status === "running" || a.status === "closing"
  ).length;

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b">
        <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Activity Log
        </span>
        <div className="flex items-center gap-1.5">
          {running > 0 && (
            <>
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
              <span className="text-xs font-medium text-emerald-600 dark:text-emerald-400">
                {running} active
              </span>
            </>
          )}
        </div>
      </div>

      {/* Column headers */}
      <div className="flex items-center gap-3 px-4 py-1.5 border-b bg-muted/20">
        <div className="w-6 shrink-0" />
        <div className="flex-1 text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Pair
        </div>
        <div className="hidden sm:block shrink-0 text-right text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground w-14">
          Size
        </div>
        <div className="shrink-0 text-right text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground w-16">
          PnL
        </div>
        <div className="hidden md:block shrink-0 text-right text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground w-14">
          Duration
        </div>
        <div className="shrink-0 text-right text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground w-20">
          Status
        </div>
      </div>

      {/* Rows */}
      <div className="divide-y divide-border/40">
        {MOCK_ACTIVITIES.map((a) => (
          <ActivityRow key={a.id} activity={a} />
        ))}
      </div>
    </div>
  );
}
