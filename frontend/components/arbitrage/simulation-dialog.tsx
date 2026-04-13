"use client";

import { useState, useCallback, useEffect, useRef, useMemo } from "react";
import {
  simulateArbitrage,
  type SimulationResult,
  type SimulationRow,
  type SimulationEvent,
  type LoopSimulation,
  type SimSizeType,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ExchangeBadge, SectionHeader } from "@/components/arbitrage/shared";
import { formatUsd, formatUsdPlain, formatPct } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Loader2, ChevronRight } from "lucide-react";

// ---------------------------------------------------------------------------
// Hold presets
// ---------------------------------------------------------------------------

const HOLD_PRESETS = [
  { label: "8h", value: "8" },
  { label: "24h", value: "24" },
  { label: "48h", value: "48" },
  { label: "7d", value: "168" },
  { label: "30d", value: "720" },
] as const;

// ---------------------------------------------------------------------------
// SimulationPanelView — inline simulation view
// ---------------------------------------------------------------------------

interface SimulationPanelViewProps {
  symbol: string;
  longExchange: string;
  shortExchange: string;
  /** "usd" = position_size (USDT), "qty" = quantity (contracts) */
  sizeType: SimSizeType;
  size: string;
  slippagePct: string;
  autoRun?: boolean;
}

export function SimulationPanelView({
  symbol,
  longExchange,
  shortExchange,
  sizeType,
  size,
  slippagePct,
  autoRun = false,
}: SimulationPanelViewProps) {
  const [holdHours, setHoldHours] = useState("24");
  const [simLoading, setSimLoading] = useState(false);
  const [simError, setSimError] = useState<string | null>(null);
  const [simResult, setSimResult] = useState<SimulationResult | null>(null);

  const runSimulation = useCallback(async () => {
    setSimLoading(true);
    setSimError(null);
    try {
      const result = await simulateArbitrage({
        symbol,
        long_exchange_id: longExchange,
        short_exchange_id: shortExchange,
        size_type: sizeType,
        ...(sizeType === "usd"
          ? { position_size: size }
          : { quantity: size }),
        slippage_pct: slippagePct || "0",
        hold_hours: holdHours,
      });
      setSimResult(result);
    } catch (err) {
      setSimError(err instanceof Error ? err.message : "Simulation failed");
    } finally {
      setSimLoading(false);
    }
  }, [symbol, longExchange, shortExchange, sizeType, size, slippagePct, holdHours]);

  // Auto-run once on mount
  const didAutoRun = useRef(false);
  useEffect(() => {
    if (autoRun && !didAutoRun.current) {
      didAutoRun.current = true;
      runSimulation();
    }
  }, [autoRun, runSimulation]);

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Toolbar: hold-hours presets + Calculate button */}
      <div className="flex items-center gap-2 px-3 py-2 border-b shrink-0">
        <span className="text-[0.625rem] text-muted-foreground uppercase tracking-wider shrink-0">
          Hold
        </span>
        <div className="grid grid-cols-5 gap-1 flex-1">
          {HOLD_PRESETS.map((preset) => (
            <button
              key={preset.value}
              type="button"
              onClick={() => setHoldHours(preset.value)}
              className={cn(
                "px-1 py-0.5 text-[0.625rem] font-medium rounded-md transition-colors",
                holdHours === preset.value
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted text-muted-foreground hover:text-foreground"
              )}
            >
              {preset.label}
            </button>
          ))}
        </div>
        <Button
          variant="default"
          size="sm"
          className="h-7 px-3 text-xs shrink-0"
          onClick={() => runSimulation()}
          disabled={simLoading}
        >
          {simLoading ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            "Calculate"
          )}
        </Button>
      </div>

      {/* Content */}
      <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
        {simLoading ? (
          <div className="flex items-center justify-center gap-2 py-12">
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            <span className="text-xs text-muted-foreground">Running...</span>
          </div>
        ) : simError ? (
          <div className="m-3 rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive">
            {simError}
          </div>
        ) : simResult ? (
          <SimulationContent result={simResult} />
        ) : (
          <div className="flex flex-col items-center justify-center gap-3 py-12 text-center px-4">
            <span className="text-xs text-muted-foreground">
              Press Calculate to simulate position cost &amp; P&L projection.
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SimulationContent — Summary + Breakdown tabs
// ---------------------------------------------------------------------------

function SimulationContent({ result }: { result: SimulationResult }) {
  const loop = result.hold_simulation;
  const rows = loop?.rows ?? [];

  return (
    <Tabs
      defaultValue="summary"
      className="flex-1 min-h-0 flex flex-col h-full"
    >
      <div className="flex justify-end px-3 pt-2 shrink-0">
        <TabsList className="h-6 p-0.5 text-[0.625rem]">
          <TabsTrigger value="summary" className="h-5 px-2 text-[0.625rem]">
            Summary
          </TabsTrigger>
          <TabsTrigger value="breakdown" className="h-5 px-2 text-[0.625rem]">
            Breakdown
          </TabsTrigger>
        </TabsList>
      </div>

      <TabsContent
        value="summary"
        className="flex-1 min-h-0 overflow-y-auto px-3 pb-4"
      >
        <SimulationSummary result={result} />
      </TabsContent>

      <TabsContent
        value="breakdown"
        className="flex-1 min-h-0 flex flex-col overflow-hidden pb-2"
      >
        {rows.length > 0 && loop ? (
          <SimulationBreakdown loop={loop} rows={rows} />
        ) : (
          <div className="px-3 py-3 text-xs text-muted-foreground">
            No breakdown data available.
          </div>
        )}
      </TabsContent>
    </Tabs>
  );
}

// ---------------------------------------------------------------------------
// SimulationSummary
// ---------------------------------------------------------------------------

function SimulationSummary({ result }: { result: SimulationResult }) {
  const holdHoursNum = parseFloat(result.hold_hours);
  const holdLabel =
    holdHoursNum >= 24
      ? `${Math.round(holdHoursNum / 24)}d`
      : `${holdHoursNum}h`;

  return (
    <div className="space-y-3 pt-2">
      {/* Position sizing */}
      <div>
        <SectionHeader className="mb-1.5">Position</SectionHeader>
        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
          <span className="text-muted-foreground">Quantity</span>
          <span className="font-mono tabular-nums text-right">
            {result.quantity} contracts
          </span>
          <span className="text-muted-foreground">Position size</span>
          <span className="font-mono tabular-nums text-right">
            {formatUsdPlain(result.actual_position_size || result.position_size)}
          </span>
          <span className="text-muted-foreground">Capital required</span>
          <span className="font-mono tabular-nums text-right">
            {formatUsdPlain(result.capital)}
          </span>
        </div>
      </div>

      <Separator />

      {/* Fees */}
      <div>
        <SectionHeader className="mb-1.5">Fees</SectionHeader>
        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
          <span className="text-muted-foreground">Entry cost</span>
          <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
            {formatUsdPlain(result.entry_cost)}
          </span>
          <span className="text-muted-foreground">Exit cost</span>
          <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
            {formatUsdPlain(result.exit_cost)}
          </span>
          <span className="text-muted-foreground">Total fees</span>
          <span className="font-mono tabular-nums text-right font-medium text-red-600 dark:text-red-400">
            {formatUsdPlain(result.total_cost)}
          </span>
        </div>
      </div>

      <Separator />

      {/* P&L projections */}
      <div>
        <SectionHeader className="mb-1.5">P&L Projection</SectionHeader>
        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
          <span className="text-muted-foreground">Funding (8h)</span>
          <PnlSpan value={result.funding_8h} />
          <span className="text-muted-foreground">Hold ({holdLabel})</span>
          <PnlSpan value={result.funding_hold} />
          <span className="text-muted-foreground">Breakeven</span>
          <span className="font-mono tabular-nums text-right">
            {parseFloat(result.breakeven_days).toFixed(1)} days
          </span>

          <span className="col-span-2 border-t border-border/30 my-1" />

          <span className="text-muted-foreground">Next P&L</span>
          <PnlSpan value={result.next_pnl} bold />
          <span className="text-muted-foreground">LCM P&L</span>
          <PnlSpan value={result.lcm_pnl} bold />
          <span className="text-muted-foreground">{holdLabel} P&L</span>
          <PnlSpan value={result.pnl_hold} bold />
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SimulationBreakdown — payment schedule with expandable event rows
// ---------------------------------------------------------------------------

function SimulationBreakdown({
  loop,
  rows,
}: {
  loop: LoopSimulation;
  rows: SimulationRow[];
}) {
  const fundingCount = rows.filter((r) => r.kind === "funding").length;
  const netPnlPct = parseFloat(loop.net_pnl_pct) * 100;

  return (
    <>
      {/* Header summary */}
      <div className="px-3 py-2 shrink-0">
        <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Hold ({loop.duration_hours}h) &mdash; {fundingCount} payments &mdash;
          Net:{" "}
          <span
            className={cn(
              "font-mono",
              netPnlPct >= 0
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-red-600 dark:text-red-400"
            )}
          >
            {formatPct(String(netPnlPct))}
          </span>
          <span className="ml-1 text-muted-foreground font-mono">
            ({formatUsd(loop.net_pnl_usd)})
          </span>
        </div>
      </div>

      {/* Sticky table header */}
      <div className="px-3 shrink-0">
        <table className="text-xs w-full table-fixed">
          <ScheduleColGroup />
          <thead>
            <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
              <th className="pr-2 pb-1 text-left font-medium">Time</th>
              <th className="pr-2 pb-1 text-left font-medium">Event</th>
              <th className="pr-2 pb-1 text-right font-medium">$ P&L</th>
              <th className="pr-2 pb-1 text-right font-medium">$ Cum.</th>
            </tr>
          </thead>
        </table>
      </div>

      {/* Scrollable schedule */}
      <ScheduleList key={loop.net_pnl_usd} rows={rows} />
    </>
  );
}

// ---------------------------------------------------------------------------
// ScheduleList — expandable rows
// ---------------------------------------------------------------------------

function ScheduleColGroup() {
  return (
    <colgroup>
      <col className="w-[18%]" />
      <col className="w-[34%]" />
      <col className="w-[24%]" />
      <col className="w-[24%]" />
    </colgroup>
  );
}

type FlatItem =
  | { type: "parent"; row: SimulationRow; index: number }
  | { type: "event"; event: SimulationEvent; parentIndex: number };

function ScheduleList({ rows }: { rows: SimulationRow[] }) {
  const [expandedSet, setExpandedSet] = useState<Set<number>>(() => new Set());

  const toggleExpanded = useCallback((index: number) => {
    setExpandedSet((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }, []);

  const flatItems = useMemo<FlatItem[]>(() => {
    const items: FlatItem[] = [];
    for (let i = 0; i < rows.length; i++) {
      items.push({ type: "parent", row: rows[i], index: i });
      if (expandedSet.has(i)) {
        for (const event of rows[i].events) {
          items.push({ type: "event", event, parentIndex: i });
        }
      }
    }
    return items;
  }, [rows, expandedSet]);

  return (
    <div className="flex-1 min-h-0 overflow-y-auto px-3">
      <table className="text-xs w-full table-fixed">
        <ScheduleColGroup />
        <tbody>
          {flatItems.map((item, idx) => {
            if (item.type === "event") {
              return (
                <EventRow
                  key={`event-${item.parentIndex}-${idx}`}
                  event={item.event}
                />
              );
            }
            const row = item.row;
            const pnlUsd = parseFloat(row.pnl_usd);
            const cumUsd = parseFloat(row.cumulative_usd);
            const isFee = row.kind === "open" || row.kind === "close";
            const isExpanded = expandedSet.has(item.index);
            return (
              <ParentRow
                key={`parent-${item.index}`}
                row={row}
                pnlUsd={pnlUsd}
                cumUsd={cumUsd}
                isFee={isFee}
                isExpanded={isExpanded}
                onToggle={() => toggleExpanded(item.index)}
              />
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ParentRow
// ---------------------------------------------------------------------------

function EventKindBadge({ kind }: { kind: SimulationRow["kind"] }) {
  if (kind === "open") {
    return (
      <span className="inline-flex items-center rounded-md px-1.5 py-0.5 text-[0.5625rem] font-medium bg-muted text-muted-foreground">
        Open fee
      </span>
    );
  }
  if (kind === "close") {
    return (
      <span className="inline-flex items-center rounded-md px-1.5 py-0.5 text-[0.5625rem] font-medium bg-muted text-muted-foreground">
        Close fee
      </span>
    );
  }
  // funding
  return (
    <span className="inline-flex items-center rounded-md px-1.5 py-0.5 text-[0.5625rem] font-medium bg-violet-500/10 text-violet-600 dark:text-violet-400">
      Funding
    </span>
  );
}

function ParentRow({
  row,
  pnlUsd,
  cumUsd,
  isFee,
  isExpanded,
  onToggle,
}: {
  row: SimulationRow;
  pnlUsd: number;
  cumUsd: number;
  isFee: boolean;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  return (
    <tr
      className={cn(
        "border-t border-border/30 h-[34px] align-middle cursor-pointer hover:bg-muted/20",
        isFee && "bg-muted/30"
      )}
      onClick={onToggle}
    >
      <td className="pr-2 py-0.5 font-mono tabular-nums text-muted-foreground text-[0.625rem] align-middle">
        <span className="flex items-center gap-0.5">
          <ChevronRight
            className={cn(
              "size-3 shrink-0 text-muted-foreground/60 transition-transform duration-150",
              isExpanded && "rotate-90"
            )}
          />
          <span>
            {row.at
              ? new Date(row.at).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                  hour12: false,
                })
              : "-"}
          </span>
        </span>
      </td>
      <td className="pr-2 py-0.5 align-middle">
        <EventKindBadge kind={row.kind} />
      </td>
      <td
        className={cn(
          "pr-2 py-0.5 text-right font-mono tabular-nums text-[0.625rem] align-middle",
          pnlUsd >= 0
            ? "text-emerald-600 dark:text-emerald-400"
            : "text-red-600 dark:text-red-400"
        )}
      >
        {formatUsd(row.pnl_usd)}
      </td>
      <td
        className={cn(
          "pr-2 py-0.5 text-right font-mono tabular-nums text-[0.625rem] font-medium align-middle",
          cumUsd >= 0
            ? "text-emerald-600 dark:text-emerald-400"
            : "text-red-600 dark:text-red-400"
        )}
      >
        {formatUsd(row.cumulative_usd)}
      </td>
    </tr>
  );
}

// ---------------------------------------------------------------------------
// EventRow
// ---------------------------------------------------------------------------

function EventRow({ event }: { event: SimulationEvent }) {
  const pnlUsd = parseFloat(event.pnl_usd);
  return (
    <tr className="h-[30px] align-middle bg-muted/10">
      <td className="pr-2 py-0.5 align-middle" />
      <td className="pr-2 py-0.5 align-middle">
        <ExchangeBadge exchangeId={event.exchange_id} />
      </td>
      <td
        className={cn(
          "pr-2 py-0.5 text-right font-mono tabular-nums text-[0.5625rem] align-middle",
          pnlUsd >= 0
            ? "text-emerald-600/80 dark:text-emerald-400/80"
            : "text-red-600/80 dark:text-red-400/80"
        )}
      >
        {formatUsd(event.pnl_usd)}
      </td>
      <td className="pr-2 py-0.5 align-middle" />
    </tr>
  );
}

// ---------------------------------------------------------------------------
// PnlSpan
// ---------------------------------------------------------------------------

function PnlSpan({ value, bold }: { value: string; bold?: boolean }) {
  const num = parseFloat(value);
  return (
    <span
      className={cn(
        "font-mono tabular-nums text-right",
        bold && "font-medium",
        num >= 0
          ? "text-emerald-600 dark:text-emerald-400"
          : "text-red-600 dark:text-red-400"
      )}
    >
      {formatUsd(value)}
    </span>
  );
}
