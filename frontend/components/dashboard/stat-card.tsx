"use client";

import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import type { LucideIcon } from "lucide-react";

interface StatCardProps {
  label: string;
  value: string;
  sub?: string;
  trend?: "up" | "down" | "neutral";
  trendValue?: string;
  icon?: LucideIcon;
  isLoading?: boolean;
  className?: string;
  accent?: "default" | "emerald" | "red" | "amber" | "blue";
}

export function StatCard({
  label,
  value,
  sub,
  trend,
  trendValue,
  icon: Icon,
  isLoading,
  className,
  accent = "default",
}: StatCardProps) {
  if (isLoading) {
    return (
      <div className={cn("rounded-lg border bg-card/50 p-4", className)}>
        <Skeleton className="h-3 w-20 mb-3" />
        <Skeleton className="h-6 w-28 mb-1.5" />
        <Skeleton className="h-3 w-16" />
      </div>
    );
  }

  const accentMap = {
    default: "text-muted-foreground",
    emerald: "text-emerald-500",
    red: "text-red-500",
    amber: "text-amber-500",
    blue: "text-blue-500",
  };

  return (
    <div
      className={cn(
        "rounded-lg border bg-card/50 p-4 flex flex-col gap-2",
        className
      )}
    >
      <div className="flex items-center justify-between">
        <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          {label}
        </span>
        {Icon && (
          <Icon
            className={cn("h-3.5 w-3.5", accentMap[accent])}
            strokeWidth={1.5}
          />
        )}
      </div>

      <div className="flex items-baseline gap-2">
        <span className="text-xl font-semibold font-mono tabular-nums">
          {value}
        </span>
        {trendValue && trend && (
          <span
            className={cn(
              "text-[0.625rem] font-medium tabular-nums",
              trend === "up" && "text-emerald-600 dark:text-emerald-400",
              trend === "down" && "text-red-600 dark:text-red-400",
              trend === "neutral" && "text-muted-foreground"
            )}
          >
            {trendValue}
          </span>
        )}
      </div>

      {sub && (
        <span className="text-[0.625rem] text-muted-foreground">{sub}</span>
      )}
    </div>
  );
}
