"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import {
  SectionHeader,
} from "@/components/monitor/shared";
import { formatTimeRelative, formatUsd } from "@/lib/format";
import type { Position } from "@/lib/api";
import {
  PlayCircle,
  XCircle,
  CircleDot,
  AlertTriangle,
  CheckCircle2,
  Timer,
  ArrowRightLeft,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface TimelineEvent {
  id: string;
  time: string;
  type:
    | "created"
    | "opened"
    | "closed"
    | "failed"
    | "error";
  label: string;
  detail?: string;
  positionId: string;
  icon: React.ReactNode;
  color: string;
}

// ---------------------------------------------------------------------------
// ActivityTimeline
// ---------------------------------------------------------------------------

interface ActivityTimelineProps {
  positions: Position[];
  isLoading: boolean;
}

export function ActivityTimeline({
  positions,
  isLoading,
}: ActivityTimelineProps) {
  const events = useMemo(() => buildTimeline(positions), [positions]);

  if (isLoading) {
    return (
      <div className="rounded-lg border bg-card/50 p-4 space-y-3">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <div className="rounded-lg border border-dashed bg-card/30 p-6 text-center">
        <Timer className="h-5 w-5 text-muted-foreground mx-auto mb-2" />
        <div className="text-sm text-muted-foreground">
          No activity yet
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      <div className="flex items-center justify-between border-b px-4 py-2.5">
        <SectionHeader>
          <ArrowRightLeft className="inline h-3 w-3 mr-0.5 align-text-top" />
          Activity
        </SectionHeader>
        <span className="text-[0.625rem] text-muted-foreground">
          {events.length} event{events.length !== 1 ? "s" : ""}
        </span>
      </div>

      <div className="relative px-4 py-3">
        {/* Vertical line */}
        <div className="absolute left-[1.65rem] top-3 bottom-3 w-px bg-border" />

        <div className="space-y-3">
          {events.map((event) => (
            <div key={event.id} className="relative flex gap-3 items-start">
              {/* Icon dot */}
              <div
                className={cn(
                  "relative z-10 flex items-center justify-center h-5 w-5 rounded-full border bg-background shrink-0",
                  event.color
                )}
              >
                {event.icon}
              </div>

              {/* Content */}
              <div className="flex-1 min-w-0 -mt-0.5">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium">{event.label}</span>
                  <span className="text-[0.625rem] text-muted-foreground font-mono tabular-nums">
                    {formatTimeRelative(event.time)}
                  </span>
                </div>
                {event.detail && (
                  <p className="text-[0.625rem] text-muted-foreground mt-0.5 truncate">
                    {event.detail}
                  </p>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Build timeline events from positions
// ---------------------------------------------------------------------------

function buildTimeline(positions: Position[]): TimelineEvent[] {
  const events: TimelineEvent[] = [];

  for (const pos of positions) {
    // Created
    events.push({
      id: `${pos.id}-created`,
      time: pos.created_at,
      type: "created",
      label: "Position created",
      detail: `${pos.size_type === "usd" ? "$" : ""}${pos.requested_size} ${pos.arbitrage_type || "funding"}`,
      positionId: pos.id,
      icon: <CircleDot className="h-3 w-3" />,
      color: "border-blue-500/30 text-blue-500",
    });

    // Opened
    if (pos.opened_at) {
      events.push({
        id: `${pos.id}-opened`,
        time: pos.opened_at,
        type: "opened",
        label: "Position opened",
        detail: `${pos.long_exchange_id} / ${pos.short_exchange_id}`,
        positionId: pos.id,
        icon: <PlayCircle className="h-3 w-3" />,
        color: "border-emerald-500/30 text-emerald-500",
      });
    }

    // Closed
    if (pos.closed_at) {
      const unrealized =
        (parseFloat(pos.long_unrealized_pnl) || 0) +
        (parseFloat(pos.short_unrealized_pnl) || 0);
      const fundingPnl = parseFloat(pos.total_funding_pnl) || 0;
      const totalPnl = unrealized + fundingPnl;

      events.push({
        id: `${pos.id}-closed`,
        time: pos.closed_at,
        type: "closed",
        label: `Position closed${pos.close_reason ? ` (${pos.close_reason.replace(/_/g, " ")})` : ""}`,
        detail: `P&L: ${formatUsd(String(totalPnl))}`,
        positionId: pos.id,
        icon: <CheckCircle2 className="h-3 w-3" />,
        color:
          totalPnl >= 0
            ? "border-emerald-500/30 text-emerald-500"
            : "border-red-500/30 text-red-500",
      });
    }

    // Failed
    if (pos.status === "failed") {
      events.push({
        id: `${pos.id}-failed`,
        time: pos.updated_at,
        type: "failed",
        label: "Position failed",
        detail: pos.error_message || undefined,
        positionId: pos.id,
        icon: <XCircle className="h-3 w-3" />,
        color: "border-red-500/30 text-red-500",
      });
    }

    // Error (but not failed status)
    if (pos.error_message && pos.status !== "failed") {
      events.push({
        id: `${pos.id}-error`,
        time: pos.updated_at,
        type: "error",
        label: "Error",
        detail: pos.error_message,
        positionId: pos.id,
        icon: <AlertTriangle className="h-3 w-3" />,
        color: "border-amber-500/30 text-amber-500",
      });
    }
  }

  // Sort by time descending (most recent first)
  events.sort(
    (a, b) => new Date(b.time).getTime() - new Date(a.time).getTime()
  );

  return events;
}
