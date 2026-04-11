"use client";

import { useState, useCallback, useMemo } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  simulateArbitrage,
  type SimulationResult,
  type SimulationRow,
  type SimulationEvent,
  type LoopSimulation,
  type SimSizeType,
} from "@/lib/api";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ExchangeBadge, SectionHeader } from "@/components/monitor/shared";
import { formatUsd, formatUsdPlain, formatRate, formatPct } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Loader2, ChevronRight } from "lucide-react";

// ---------------------------------------------------------------------------
// SimulationDialog — triggered from the open position panel
// ---------------------------------------------------------------------------

interface SimulationDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  symbol: string;
  longExchange: string;
  shortExchange: string;
  sizeType: "usd" | "base";
  size: string;
  slippagePct: string;
}

export function SimulationDialog({
  open,
  onOpenChange,
  symbol,
  longExchange,
  shortExchange,
  sizeType,
  size,
  slippagePct,
}: SimulationDialogProps) {
  const simulateMutation = useMutation({
    mutationFn: simulateArbitrage,
  });

  const runSimulation = useCallback(() => {
    const simSizeType: SimSizeType = sizeType === "usd" ? "usd" : "qty";
    simulateMutation.mutate({
      symbol,
      long_exchange_id: longExchange,
      short_exchange_id: shortExchange,
      size_type: simSizeType,
      position_size: sizeType === "usd" ? size : undefined,
      quantity: sizeType === "base" ? size : undefined,
      slippage_pct: slippagePct || "0",
    });
  }, [symbol, longExchange, shortExchange, sizeType, size, slippagePct, simulateMutation]);

  // Auto-run simulation when dialog opens
  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen && !simulateMutation.data && !simulateMutation.isPending) {
      runSimulation();
    }
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[80vh] flex flex-col overflow-hidden p-0">
        <DialogHeader className="px-4 pt-4 pb-0 shrink-0">
          <div className="flex items-center justify-between">
            <DialogTitle className="text-sm font-medium">
              Simulation — {symbol}
            </DialogTitle>
            <Button
              variant="outline"
              size="sm"
              className="h-6 text-[0.625rem] px-2"
              onClick={runSimulation}
              disabled={simulateMutation.isPending}
            >
              {simulateMutation.isPending ? (
                <Loader2 className="h-3 w-3 animate-spin mr-1" />
              ) : null}
              Re-run
            </Button>
          </div>
        </DialogHeader>

        <div className="flex-1 min-h-0 overflow-hidden">
          {simulateMutation.isPending ? (
            <div className="flex items-center justify-center gap-2 py-12">
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
              <span className="text-xs text-muted-foreground">
                Running simulation...
              </span>
            </div>
          ) : simulateMutation.error ? (
            <div className="mx-4 my-4 rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive">
              {simulateMutation.error.message}
            </div>
          ) : simulateMutation.data ? (
            <SimulationContent result={simulateMutation.data} />
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// SimulationContent — tabs: Summary + Breakdown
// ---------------------------------------------------------------------------

function SimulationContent({ result }: { result: SimulationResult }) {
  const loop = result.hold_simulation;
  const rows = loop?.rows ?? [];

  return (
    <Tabs
      defaultValue="summary"
      className="flex-1 min-h-0 flex flex-col h-full"
    >
      <div className="flex justify-end px-4 pt-2 shrink-0">
        <TabsList className="h-6 p-0.5 text-[0.625rem]">
          <TabsTrigger value="summary" className="h-5 px-2 text-[0.625rem]">
            Summary
          </TabsTrigger>
          <TabsTrigger value="breakdown" className="h-5 px-2 text-[0.625rem]">
            Breakdown
          </TabsTrigger>
        </TabsList>
      </div>

      {/* Summary tab */}
      <TabsContent
        value="summary"
        className="flex-1 min-h-0 overflow-y-auto px-4 pb-4"
      >
        <SimulationSummary result={result} />
      </TabsContent>

      {/* Breakdown tab */}
      <TabsContent
        value="breakdown"
        className="flex-1 min-h-0 flex flex-col overflow-hidden pb-2"
      >
        {rows.length > 0 && loop ? (
          <SimulationBreakdown loop={loop} rows={rows} />
        ) : (
          <div className="px-4 py-3 text-xs text-muted-foreground">
            No breakdown data available.
          </div>
        )}
      </TabsContent>
    </Tabs>
  );
}

// ---------------------------------------------------------------------------
// SimulationSummary — scalar KPI grid
// ---------------------------------------------------------------------------

function SimulationSummary({ result }: { result: SimulationResult }) {
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
            {formatUsdPlain(
              result.actual_position_size || result.position_size
            )}
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
          <span className="text-muted-foreground">
            Hold ({result.hold_hours}h)
          </span>
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
          <span className="text-muted-foreground">
            {result.hold_hours}h P&L
          </span>
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
      <div className="px-4 py-2 shrink-0">
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

      {/* Table header */}
      <div className="px-4 shrink-0">
        <table className="text-xs w-full table-fixed">
          <ScheduleColGroup />
          <thead>
            <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
              <th className="pr-3 pb-1 text-left font-medium">Time</th>
              <th className="pr-3 pb-1 text-left font-medium">Hour</th>
              <th className="pr-3 pb-1 text-left font-medium">Event</th>
              <th className="pr-3 pb-1 text-right font-medium">Rate</th>
              <th className="pr-3 pb-1 text-right font-medium">$ P&L</th>
              <th className="pr-3 pb-1 text-right font-medium">$ Cum.</th>
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
// ScheduleList — expandable payment rows (no virtualization needed in dialog)
// ---------------------------------------------------------------------------

function ScheduleColGroup() {
  return (
    <colgroup>
      <col className="w-[12%]" />
      <col className="w-[12%]" />
      <col className="w-[28%]" />
      <col className="w-[16%]" />
      <col className="w-[16%]" />
      <col className="w-[16%]" />
    </colgroup>
  );
}

type FlatItem =
  | { type: "parent"; row: SimulationRow; index: number }
  | { type: "event"; event: SimulationEvent; parentIndex: number };

function ScheduleList({ rows }: { rows: SimulationRow[] }) {
  const [expandedSet, setExpandedSet] = useState<Set<number>>(
    () => new Set()
  );

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
    <div className="flex-1 min-h-0 overflow-y-auto px-4">
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
// ParentRow — summary row for each simulation step
// ---------------------------------------------------------------------------

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
  const eventCount = row.events.length;

  return (
    <tr
      className={cn(
        "border-t border-border/30 h-[34px] align-middle cursor-pointer hover:bg-muted/20",
        isFee && "bg-muted/30"
      )}
      onClick={onToggle}
    >
      {/* Time */}
      <td className="pr-3 py-0.5 font-mono tabular-nums text-muted-foreground text-[0.625rem] align-middle">
        <span className="flex items-center gap-1">
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

      {/* Hour */}
      <td className="pr-3 py-0.5 font-mono tabular-nums text-[0.625rem] align-middle">
        {row.label}
      </td>

      {/* Event */}
      <td className="pr-3 py-0.5 align-middle">
        <span className="text-muted-foreground text-[0.5625rem]">
          {isFee
            ? row.kind === "open"
              ? "Open"
              : "Close"
            : `${eventCount} ${eventCount === 1 ? "leg" : "legs"}`}
        </span>
      </td>

      {/* Rate */}
      <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-[0.625rem] text-muted-foreground align-middle">
        -
      </td>

      {/* $ P&L */}
      <td
        className={cn(
          "pr-3 py-0.5 text-right font-mono tabular-nums text-[0.625rem] align-middle",
          pnlUsd >= 0
            ? "text-emerald-600 dark:text-emerald-400"
            : "text-red-600 dark:text-red-400"
        )}
      >
        {formatUsd(row.pnl_usd)}
      </td>

      {/* $ Cumulative */}
      <td
        className={cn(
          "pr-3 py-0.5 text-right font-mono tabular-nums text-[0.625rem] font-medium align-middle",
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
// EventRow — per-exchange detail (expanded child)
// ---------------------------------------------------------------------------

function EventRow({ event }: { event: SimulationEvent }) {
  const pnlUsd = parseFloat(event.pnl_usd);
  return (
    <tr className="h-[34px] align-middle bg-muted/10">
      {/* Time — empty */}
      <td className="pr-3 py-0.5 align-middle" />

      {/* Hour — empty */}
      <td className="pr-3 py-0.5 align-middle" />

      {/* Event — exchange badge + side */}
      <td className="pr-3 py-0.5 align-middle">
        <span className="flex items-center gap-1 pl-4">
          <ExchangeBadge exchangeId={event.exchange_id} />
          <span className="text-muted-foreground text-[0.5625rem]">
            {event.side}
          </span>
        </span>
      </td>

      {/* Rate */}
      <td className="pr-3 py-0.5 text-right font-mono tabular-nums text-muted-foreground text-[0.5625rem] align-middle">
        {event.rate ? formatRate(event.rate) : "-"}
      </td>

      {/* $ P&L */}
      <td
        className={cn(
          "pr-3 py-0.5 text-right font-mono tabular-nums text-[0.5625rem] align-middle",
          pnlUsd >= 0
            ? "text-emerald-600/70 dark:text-emerald-400/70"
            : "text-red-600/70 dark:text-red-400/70"
        )}
      >
        {formatUsd(event.pnl_usd)}
      </td>

      {/* $ Cumulative — empty */}
      <td className="pr-3 py-0.5 align-middle" />
    </tr>
  );
}

// ---------------------------------------------------------------------------
// PnlSpan — colored P&L value
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
