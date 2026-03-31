"use client";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Power } from "lucide-react";
import type {
  SimulationConfig,
  SimulationActions,
} from "@/hooks/use-simulation";

// ---------------------------------------------------------------------------
// Preset buttons for position size
// ---------------------------------------------------------------------------

const SIZE_PRESETS = [
  { label: "$100", value: "100" },
  { label: "$1K", value: "1000" },
  { label: "$10K", value: "10000" },
  { label: "$100K", value: "100000" },
];

const LEVERAGE_OPTIONS = ["1", "2"];

// ---------------------------------------------------------------------------
// Simulation Panel (content only, toggle is in parent)
// ---------------------------------------------------------------------------

interface SimulationPanelProps {
  config: SimulationConfig;
  actions: SimulationActions;
}

export function SimulationPanel({
  config,
  actions,
}: SimulationPanelProps) {
  return (
    <div className="rounded-lg border bg-card/50 p-3 space-y-3">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {/* Enable/Disable */}
        <div className="space-y-1">
          <label className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            Status
          </label>
          <Button
            variant={config.enabled ? "default" : "outline"}
            size="sm"
            className={cn(
              "h-7 w-full gap-1.5 text-xs",
              config.enabled &&
                "bg-violet-600 text-white hover:bg-violet-700 dark:bg-violet-500 dark:hover:bg-violet-600"
            )}
            onClick={() => actions.setEnabled(!config.enabled)}
          >
            <Power className="h-3 w-3" />
            {config.enabled ? "Enabled" : "Disabled"}
          </Button>
        </div>

        {/* Position size */}
        <div className="space-y-1">
          <label
            className={cn(
              "text-[0.625rem] font-medium uppercase tracking-wider",
              config.enabled
                ? "text-foreground"
                : "text-muted-foreground"
            )}
          >
            Position Size, $
          </label>
          <div className="flex items-center gap-1.5">
            <input
              type="text"
              inputMode="decimal"
              value={config.positionSize}
              onChange={(e) => actions.setPositionSize(e.target.value)}
              disabled={!config.enabled}
              placeholder="1000"
              className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50"
            />
          </div>
          <div className="flex items-center gap-1">
            {SIZE_PRESETS.map((preset) => (
              <button
                key={preset.value}
                type="button"
                disabled={!config.enabled}
                className={cn(
                  "rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium transition-colors",
                  config.positionSize === preset.value
                    ? "bg-violet-500/15 text-violet-600 dark:text-violet-400"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                  "disabled:cursor-not-allowed disabled:opacity-50"
                )}
                onClick={() => actions.setPositionSize(preset.value)}
              >
                {preset.label}
              </button>
            ))}
          </div>
        </div>

        {/* Leverage */}
        <div className="space-y-1">
          <label
            className={cn(
              "text-[0.625rem] font-medium uppercase tracking-wider",
              config.enabled
                ? "text-foreground"
                : "text-muted-foreground"
            )}
          >
            Leverage
          </label>
          <div className="flex items-center gap-1">
            {LEVERAGE_OPTIONS.map((lev) => (
              <button
                key={lev}
                type="button"
                disabled={!config.enabled}
                className={cn(
                  "h-7 flex-1 rounded-md border text-xs font-medium tabular-nums transition-colors",
                  config.leverage === lev
                    ? "border-violet-500/40 bg-violet-500/10 text-violet-600 dark:text-violet-400"
                    : "border-transparent bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground",
                  "disabled:cursor-not-allowed disabled:opacity-50"
                )}
                onClick={() => actions.setLeverage(lev)}
              >
                {lev}x
              </button>
            ))}
          </div>
        </div>

        {/* Slippage */}
        <div className="space-y-1">
          <label
            className={cn(
              "text-[0.625rem] font-medium uppercase tracking-wider",
              config.enabled
                ? "text-foreground"
                : "text-muted-foreground"
            )}
          >
            Slippage, %
          </label>
          <input
            type="text"
            inputMode="decimal"
            value={config.slippagePct}
            onChange={(e) => actions.setSlippagePct(e.target.value)}
            disabled={!config.enabled}
            placeholder="0.05"
            className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50"
          />
        </div>
      </div>
    </div>
  );
}
