"use client";

import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown } from "lucide-react";
import type { PositionStatus } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { formatUsd } from "@/lib/format";

// ---------------------------------------------------------------------------
// ExchangeBadge
// ---------------------------------------------------------------------------

const exchangeColors: Record<string, string> = {
  bybit: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
  okx: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
  gate: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  binance: "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400",
};

export function ExchangeBadge({ exchangeId }: { exchangeId: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium uppercase tracking-wider",
        exchangeColors[exchangeId] ?? "bg-muted text-muted-foreground"
      )}
    >
      {exchangeId}
    </span>
  );
}

// ---------------------------------------------------------------------------
// LegLabel — "Long"/"Short" indicator with colored icon
// ---------------------------------------------------------------------------

export function LegLabel({
  side,
  exchangeId,
}: {
  side: "long" | "short";
  exchangeId: string;
}) {
  const isLong = side === "long";
  return (
    <span className="inline-flex items-center gap-1">
      <span
        className={cn(
          "inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm shrink-0",
          isLong
            ? "bg-emerald-600 dark:bg-emerald-500"
            : "bg-red-600 dark:bg-red-500"
        )}
      >
        {isLong ? (
          <ArrowUp className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
        ) : (
          <ArrowDown className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
        )}
      </span>
      <ExchangeBadge exchangeId={exchangeId} />
    </span>
  );
}

// ---------------------------------------------------------------------------
// StatusBadge
// ---------------------------------------------------------------------------

const statusConfig: Record<
  PositionStatus,
  { label: string; className: string }
> = {
  pending: {
    label: "Pending",
    className:
      "bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/20",
  },
  opening: {
    label: "Opening",
    className:
      "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20",
  },
  open: {
    label: "Open",
    className:
      "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  },
  partially_open: {
    label: "Partial",
    className:
      "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
  },
  closing: {
    label: "Closing",
    className:
      "bg-orange-500/10 text-orange-600 dark:text-orange-400 border-orange-500/20",
  },
  closed: {
    label: "Closed",
    className: "bg-muted text-muted-foreground border-border",
  },
  failed: {
    label: "Failed",
    className:
      "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20",
  },
};

export function StatusBadge({ status }: { status: PositionStatus }) {
  const cfg = statusConfig[status] ?? statusConfig.pending;
  return (
    <Badge variant="outline" className={cn("text-[0.625rem]", cfg.className)}>
      {cfg.label}
    </Badge>
  );
}

// ---------------------------------------------------------------------------
// PnlValue
// ---------------------------------------------------------------------------

export function PnlValue({
  value,
  className,
}: {
  value: string;
  className?: string;
}) {
  const num = parseFloat(value);
  const isPositive = num > 0;
  const isNegative = num < 0;
  return (
    <span
      className={cn(
        "font-mono tabular-nums",
        isPositive && "text-emerald-600 dark:text-emerald-400",
        isNegative && "text-red-600 dark:text-red-400",
        !isPositive && !isNegative && "text-muted-foreground",
        className
      )}
    >
      {formatUsd(value)}
    </span>
  );
}

// ---------------------------------------------------------------------------
// SectionHeader
// ---------------------------------------------------------------------------

export function SectionHeader({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground",
        className
      )}
    >
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// MetricRow — for key-value pairs
// ---------------------------------------------------------------------------

export function MetricRow({
  label,
  children,
  className,
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <>
      <span className={cn("text-muted-foreground", className)}>{label}</span>
      <span className="font-mono tabular-nums text-right">{children}</span>
    </>
  );
}
