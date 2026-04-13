"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { PositionHistory } from "@/components/arbitrage/position-history";
import { ActivityTimeline } from "@/components/arbitrage/activity-timeline";
import type { Position } from "@/lib/api";
import { History, ArrowRightLeft } from "lucide-react";

// ---------------------------------------------------------------------------
// ArbitrageBottomTabs — tabs for position history and activity
// ---------------------------------------------------------------------------

interface ArbitrageBottomTabsProps {
  positions: Position[];
  allPositions: Position[];
  isLoading: boolean;
}

type Tab = "history" | "activity";

export function ArbitrageBottomTabs({
  positions,
  allPositions,
  isLoading,
}: ArbitrageBottomTabsProps) {
  const [tab, setTab] = useState<Tab>("history");

  const tabs: { key: Tab; label: string; icon: React.ReactNode; count?: number }[] = [
    {
      key: "history",
      label: "Position History",
      icon: <History className="h-3 w-3" />,
      count: positions.length > 0 ? positions.length : undefined,
    },
    {
      key: "activity",
      label: "Activity",
      icon: <ArrowRightLeft className="h-3 w-3" />,
    },
  ];

  return (
    <div className="rounded-lg border bg-card/30 overflow-hidden flex flex-col">
      {/* Tab header */}
      <div className="flex items-center border-b bg-card/20 px-1 pt-1 gap-0.5">
        {tabs.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setTab(t.key)}
            className={cn(
              "flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-t-md transition-colors relative",
              tab === t.key
                ? "bg-background border border-b-background text-foreground -mb-px z-10"
                : "text-muted-foreground hover:text-foreground hover:bg-muted/40"
            )}
          >
            {t.icon}
            {t.label}
            {t.count !== undefined && (
              <span className="ml-0.5 inline-flex items-center justify-center rounded-full bg-muted px-1.5 min-w-[1.1rem] h-4 text-[0.5625rem] font-medium text-muted-foreground">
                {t.count}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 min-h-0">
        {tab === "history" && (
          <PositionHistory positions={positions} isLoading={isLoading} />
        )}
        {tab === "activity" && (
          <ActivityTimeline positions={allPositions} isLoading={isLoading} />
        )}
      </div>
    </div>
  );
}
