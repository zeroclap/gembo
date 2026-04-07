"use client";

import { useState } from "react";
import { ArbitrageTable } from "./arbitrage-table";
import { SpreadTable } from "./spread-table";
import { cn } from "@/lib/utils";

type ArbitrageMode = "funding" | "spread";

export default function ArbitragePage() {
  const [mode, setMode] = useState<ArbitrageMode>("funding");

  return (
    <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
      {/* Mode switcher */}
      <div className="shrink-0 border-b px-4 sm:px-6 lg:px-8">
        <div className="flex items-center gap-1 -mb-px">
          <button
            className={cn(
              "px-3 py-2 text-xs font-medium border-b-2 transition-colors",
              mode === "funding"
                ? "border-foreground text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
            onClick={() => setMode("funding")}
          >
            Funding Rate
          </button>
          <button
            className={cn(
              "px-3 py-2 text-xs font-medium border-b-2 transition-colors",
              mode === "spread"
                ? "border-foreground text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
            onClick={() => setMode("spread")}
          >
            Spread
          </button>
        </div>
      </div>

      {/* Content */}
      {mode === "funding" ? <ArbitrageTable /> : <SpreadTable />}
    </div>
  );
}
