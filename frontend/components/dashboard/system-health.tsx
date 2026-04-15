"use client";

import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import {
  CheckCircle2,
  XCircle,
  AlertCircle,
  Database,
  Wifi,
  Server,
  Activity,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Mock system health — replace with real API hooks when backend ready
// ---------------------------------------------------------------------------

type HealthStatus = "healthy" | "degraded" | "down";

interface ServiceHealth {
  name: string;
  status: HealthStatus;
  latencyMs?: number;
  detail?: string;
  icon: typeof Server;
}

const MOCK_SERVICES: ServiceHealth[] = [
  {
    name: "Collector",
    status: "healthy",
    latencyMs: 12,
    detail: "4 exchanges active",
    icon: Activity,
  },
  {
    name: "API",
    status: "healthy",
    latencyMs: 8,
    detail: "p99: 42ms",
    icon: Server,
  },
  {
    name: "Redis",
    status: "healthy",
    latencyMs: 1,
    detail: "used: 124 MB",
    icon: Database,
  },
  {
    name: "Bybit WS",
    status: "healthy",
    latencyMs: 38,
    detail: "connected",
    icon: Wifi,
  },
  {
    name: "Binance WS",
    status: "healthy",
    latencyMs: 42,
    detail: "connected",
    icon: Wifi,
  },
  {
    name: "OKX WS",
    status: "healthy",
    latencyMs: 55,
    detail: "connected",
    icon: Wifi,
  },
  {
    name: "Deribit WS",
    status: "degraded",
    latencyMs: 380,
    detail: "high latency",
    icon: Wifi,
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const STATUS_CONFIG: Record<HealthStatus, { icon: typeof CheckCircle2; color: string; bg: string }> = {
  healthy: {
    icon: CheckCircle2,
    color: "text-emerald-600 dark:text-emerald-400",
    bg: "bg-emerald-500",
  },
  degraded: {
    icon: AlertCircle,
    color: "text-amber-600 dark:text-amber-400",
    bg: "bg-amber-500",
  },
  down: {
    icon: XCircle,
    color: "text-red-600 dark:text-red-400",
    bg: "bg-red-500",
  },
};

function ServiceRow({ svc }: { svc: ServiceHealth }) {
  const cfg = STATUS_CONFIG[svc.status];
  const Icon = svc.icon;

  return (
    <div className="flex items-center gap-3 px-4 py-2 hover:bg-muted/30 transition-colors">
      <div className="flex items-center justify-center h-6 w-6 rounded-md bg-muted/60 shrink-0">
        <Icon className="h-3 w-3 text-muted-foreground" strokeWidth={1.5} />
      </div>

      <div className="flex-1 min-w-0">
        <span className="text-xs font-medium">{svc.name}</span>
        {svc.detail && (
          <div className="text-[0.6rem] text-muted-foreground">{svc.detail}</div>
        )}
      </div>

      {svc.latencyMs !== undefined && (
        <span className="text-xs font-mono tabular-nums text-muted-foreground hidden sm:block">
          {svc.latencyMs}ms
        </span>
      )}

      <div className="flex items-center gap-1.5 shrink-0">
        <span className={cn("h-1.5 w-1.5 rounded-full shrink-0", cfg.bg)} />
        <span className={cn("text-xs font-medium capitalize", cfg.color)}>
          {svc.status}
        </span>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SystemHealth
// ---------------------------------------------------------------------------

interface SystemHealthProps {
  isLoading?: boolean;
}

export function SystemHealth({ isLoading }: SystemHealthProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border bg-card/50 overflow-hidden">
        <div className="px-4 py-3 border-b">
          <Skeleton className="h-3 w-24" />
        </div>
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-2">
            <Skeleton className="h-6 w-6 rounded-md" />
            <Skeleton className="h-3 w-20" />
            <Skeleton className="h-3 w-12 ml-auto" />
          </div>
        ))}
      </div>
    );
  }

  const issues = MOCK_SERVICES.filter((s) => s.status !== "healthy").length;

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b">
        <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          System Health
        </span>
        {issues === 0 ? (
          <div className="flex items-center gap-1">
            <CheckCircle2 className="h-3 w-3 text-emerald-500" />
            <span className="text-xs text-emerald-600 dark:text-emerald-400 font-medium">
              All systems nominal
            </span>
          </div>
        ) : (
          <div className="flex items-center gap-1">
            <AlertCircle className="h-3 w-3 text-amber-500" />
            <span className="text-xs text-amber-600 dark:text-amber-400 font-medium">
              {issues} issue{issues > 1 ? "s" : ""}
            </span>
          </div>
        )}
      </div>

      {/* Rows */}
      <div className="divide-y divide-border/40">
        {MOCK_SERVICES.map((svc, i) => (
          <ServiceRow key={i} svc={svc} />
        ))}
      </div>
    </div>
  );
}
