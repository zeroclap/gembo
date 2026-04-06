"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  fetchPositions,
  closePosition,
  deletePosition,
  fetchPositionOrders,
  fetchPositionFunding,
  type Position,
  type PositionStatus,
  type ExchangeOrder,
  type FundingPayment,
} from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import {
  ArrowUp,
  ArrowDown,
  ArrowUpDown,
  X,
  Trash2,
  Loader2,
  TrendingUp,
  TrendingDown,
  SlidersHorizontal,
  ChevronDown,
  ChevronUp,
  RotateCcw,
  Clock,
  DollarSign,
  Activity,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

function formatPrice(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "-";
  if (num >= 1)
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  if (num >= 0.01)
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 4,
      maximumFractionDigits: 4,
    });
  return num.toLocaleString(undefined, {
    minimumFractionDigits: 6,
    maximumFractionDigits: 8,
  });
}

function formatUsd(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "$0.00";
  const abs = Math.abs(num);
  const sign = num >= 0 ? "+" : "-";
  if (abs >= 1000) {
    return `${sign}$${abs.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }
  if (abs >= 1) return `${sign}$${abs.toFixed(2)}`;
  return `${sign}$${abs.toFixed(4)}`;
}

function formatUsdPlain(value: string): string {
  const num = parseFloat(value);
  if (!isFinite(num)) return "-";
  const abs = Math.abs(num);
  if (abs >= 1000) {
    return `$${abs.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }
  if (abs >= 1) return `$${abs.toFixed(2)}`;
  return `$${abs.toFixed(4)}`;
}

function formatQty(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "-";
  if (num >= 1) return num.toFixed(4);
  return num.toFixed(8);
}

function formatRate(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  const pct = num * 100;
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
}

function formatTimeShort(iso: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "-";
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatDuration(from: string, to?: string): string {
  if (!from) return "-";
  const start = new Date(from).getTime();
  if (isNaN(start)) return "-";
  const end = to ? new Date(to).getTime() : Date.now();
  if (isNaN(end)) return "-";
  const diffMs = end - start;
  if (diffMs < 0) return "-";
  const hours = Math.floor(diffMs / (1000 * 60 * 60));
  const mins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));
  if (hours >= 24) {
    const days = Math.floor(hours / 24);
    const remHours = hours % 24;
    return `${days}d ${remHours}h`;
  }
  return `${hours}h ${mins}m`;
}

// ---------------------------------------------------------------------------
// Small components
// ---------------------------------------------------------------------------

function ExchangeBadge({ exchangeId }: { exchangeId: string }) {
  const colorMap: Record<string, string> = {
    bybit: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
    okx: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
    gate: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
    binance: "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400",
  };

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-1.5 py-0.5 text-[0.625rem] font-medium uppercase tracking-wider",
        colorMap[exchangeId] ?? "bg-muted text-muted-foreground"
      )}
    >
      {exchangeId}
    </span>
  );
}

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

function StatusBadge({ status }: { status: PositionStatus }) {
  const cfg = statusConfig[status] ?? statusConfig.pending;
  return (
    <Badge variant="outline" className={cn("text-[0.625rem]", cfg.className)}>
      {cfg.label}
    </Badge>
  );
}

function PnlValue({
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
// Status filter options
// ---------------------------------------------------------------------------

const statusOptions: { label: string; value: string }[] = [
  { label: "All", value: "" },
  { label: "Active", value: "pending,opening,open,partially_open,closing" },
  { label: "Open", value: "open,partially_open" },
  { label: "Closed", value: "closed" },
  { label: "Failed", value: "failed" },
];

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

type SortKey = "symbol" | "status" | "pnl" | "funding" | "total" | "created";

interface SortState {
  key: SortKey;
  desc: boolean;
}

function getSortValue(p: Position, key: SortKey): number | string {
  switch (key) {
    case "symbol":
      return p.symbol;
    case "status":
      return p.status;
    case "pnl": {
      const unrealized =
        parseFloat(p.long_unrealized_pnl) +
        parseFloat(p.short_unrealized_pnl);
      return isNaN(unrealized) ? 0 : unrealized;
    }
    case "funding":
      return parseFloat(p.total_funding_pnl) || 0;
    case "total": {
      const unrealized =
        parseFloat(p.long_unrealized_pnl) +
        parseFloat(p.short_unrealized_pnl);
      const funding = parseFloat(p.total_funding_pnl) || 0;
      return (isNaN(unrealized) ? 0 : unrealized) + funding;
    }
    case "created":
      return new Date(p.created_at).getTime() || 0;
  }
}

function sortPositions(data: Position[], sort: SortState): Position[] {
  const sorted = [...data].sort((a, b) => {
    const va = getSortValue(a, sort.key);
    const vb = getSortValue(b, sort.key);
    if (typeof va === "string" && typeof vb === "string") {
      return va.localeCompare(vb);
    }
    return (va as number) - (vb as number);
  });
  if (sort.desc) sorted.reverse();
  return sorted;
}

function SortButton({
  label,
  sortKey,
  current,
  onToggle,
}: {
  label: string;
  sortKey: SortKey;
  current: SortState;
  onToggle: (key: SortKey) => void;
}) {
  const active = current.key === sortKey;
  return (
    <button
      className="flex items-center font-medium"
      onClick={() => onToggle(sortKey)}
    >
      {label}
      {active && current.desc ? (
        <ArrowDown className="ml-1 h-3 w-3" />
      ) : active && !current.desc ? (
        <ArrowUp className="ml-1 h-3 w-3" />
      ) : (
        <ArrowUpDown className="ml-1 h-3 w-3 opacity-30" />
      )}
    </button>
  );
}

// ---------------------------------------------------------------------------
// Filter state (symbol + exchange)
// ---------------------------------------------------------------------------

interface PositionFilters {
  symbols: string[];
  exchanges: string[];
  status: string;
}

const EMPTY_FILTERS: PositionFilters = {
  symbols: [],
  exchanges: [],
  status: "",
};

function filterPositions(
  data: Position[],
  filters: PositionFilters
): Position[] {
  return data.filter((p) => {
    if (filters.status) {
      const allowed = filters.status.split(",");
      if (!allowed.includes(p.status)) return false;
    }
    if (filters.symbols.length > 0) {
      const sym = p.symbol.toLowerCase();
      if (!filters.symbols.some((tag) => sym.includes(tag.toLowerCase()))) {
        return false;
      }
    }
    if (filters.exchanges.length > 0) {
      const exLower = filters.exchanges.map((e) => e.toLowerCase());
      const longEx = p.long_exchange_id.toLowerCase();
      const shortEx = p.short_exchange_id.toLowerCase();
      if (!exLower.some((e) => longEx === e || shortEx === e)) {
        return false;
      }
    }
    return true;
  });
}

// ---------------------------------------------------------------------------
// Tag input for filters (shared with arbitrage but self-contained here)
// ---------------------------------------------------------------------------

function TagInput({
  tags,
  onChange,
  placeholder,
  suggestions,
}: {
  tags: string[];
  onChange: (tags: string[]) => void;
  placeholder: string;
  suggestions: string[];
}) {
  const [input, setInput] = useState("");
  const [showDropdown, setShowDropdown] = useState(false);
  const [highlightIndex, setHighlightIndex] = useState(-1);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const filtered = input
    ? suggestions.filter(
        (s) =>
          s.toLowerCase().includes(input.toLowerCase()) &&
          !tags.some((t) => t.toLowerCase() === s.toLowerCase())
      )
    : suggestions.filter(
        (s) => !tags.some((t) => t.toLowerCase() === s.toLowerCase())
      );

  const addTag = useCallback(
    (value: string) => {
      const trimmed = value.trim();
      if (!trimmed) return;
      if (tags.some((t) => t.toLowerCase() === trimmed.toLowerCase())) {
        setInput("");
        return;
      }
      onChange([...tags, trimmed]);
      setInput("");
      setShowDropdown(false);
      setHighlightIndex(-1);
    },
    [tags, onChange]
  );

  const removeTag = useCallback(
    (index: number) => {
      onChange(tags.filter((_, i) => i !== index));
    },
    [tags, onChange]
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter") {
        e.preventDefault();
        if (highlightIndex >= 0 && highlightIndex < filtered.length) {
          addTag(filtered[highlightIndex]);
        } else if (input.trim()) {
          addTag(input);
        }
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        setShowDropdown(true);
        setHighlightIndex((prev) =>
          prev < filtered.length - 1 ? prev + 1 : 0
        );
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlightIndex((prev) =>
          prev > 0 ? prev - 1 : filtered.length - 1
        );
      } else if (e.key === "Escape") {
        setShowDropdown(false);
        setHighlightIndex(-1);
      } else if (e.key === "Backspace" && !input && tags.length > 0) {
        removeTag(tags.length - 1);
      }
    },
    [input, tags, filtered, highlightIndex, addTag, removeTag]
  );

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setShowDropdown(false);
        setHighlightIndex(-1);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  return (
    <div ref={containerRef} className="relative">
      <div
        className="flex min-h-7 flex-wrap items-center gap-1 rounded-md border bg-background px-2 py-1 ring-ring focus-within:ring-2"
        onClick={() => inputRef.current?.focus()}
      >
        {tags.map((tag, i) => (
          <Badge
            key={tag}
            variant="secondary"
            className="h-5 gap-0.5 pl-1.5 pr-1 text-[0.625rem]"
          >
            {tag}
            <button
              type="button"
              className="ml-0.5 rounded-full p-0 hover:bg-muted"
              onClick={(e) => {
                e.stopPropagation();
                removeTag(i);
              }}
            >
              <X className="h-2.5 w-2.5" />
            </button>
          </Badge>
        ))}
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => {
            setInput(e.target.value);
            setHighlightIndex(-1);
            setShowDropdown(true);
          }}
          onFocus={() => setShowDropdown(true)}
          onKeyDown={handleKeyDown}
          placeholder={tags.length === 0 ? placeholder : ""}
          className="h-5 min-w-[60px] flex-1 bg-transparent text-xs outline-none placeholder:text-muted-foreground"
        />
      </div>

      {showDropdown && filtered.length > 0 && (
        <div className="absolute left-0 z-50 mt-1 max-h-48 w-full overflow-y-auto rounded-md border bg-popover p-1 shadow-md">
          {filtered.map((item, i) => (
            <button
              key={item}
              type="button"
              className={cn(
                "flex w-full items-center rounded-sm px-2 py-1 text-xs outline-none",
                i === highlightIndex
                  ? "bg-accent text-accent-foreground"
                  : "text-popover-foreground hover:bg-accent hover:text-accent-foreground"
              )}
              onMouseEnter={() => setHighlightIndex(i)}
              onMouseDown={(e) => {
                e.preventDefault();
                addTag(item);
              }}
            >
              {item}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Filter panel component
// ---------------------------------------------------------------------------

function FilterPanel({
  filters,
  setSymbols,
  setExchanges,
  setStatus,
  availableSymbols,
  availableExchanges,
}: {
  filters: PositionFilters;
  setSymbols: (s: string[]) => void;
  setExchanges: (s: string[]) => void;
  setStatus: (s: string) => void;
  availableSymbols: string[];
  availableExchanges: string[];
}) {
  return (
    <div className="grid gap-3 rounded-lg border bg-card/50 p-3 sm:grid-cols-3">
      <div className="space-y-1">
        <label
          className={cn(
            "text-[0.625rem] font-medium uppercase tracking-wider",
            filters.symbols.length > 0
              ? "text-foreground"
              : "text-muted-foreground"
          )}
        >
          Symbol
        </label>
        <TagInput
          tags={filters.symbols}
          onChange={setSymbols}
          placeholder="Type symbol, press Enter..."
          suggestions={availableSymbols}
        />
      </div>
      <div className="space-y-1">
        <label
          className={cn(
            "text-[0.625rem] font-medium uppercase tracking-wider",
            filters.exchanges.length > 0
              ? "text-foreground"
              : "text-muted-foreground"
          )}
        >
          Exchange
        </label>
        <TagInput
          tags={filters.exchanges}
          onChange={setExchanges}
          placeholder="Type exchange, press Enter..."
          suggestions={availableExchanges}
        />
      </div>
      <div className="space-y-1">
        <label
          className={cn(
            "text-[0.625rem] font-medium uppercase tracking-wider",
            filters.status ? "text-foreground" : "text-muted-foreground"
          )}
        >
          Status
        </label>
        <div className="flex flex-wrap items-center gap-1">
          {statusOptions.map((opt) => (
            <button
              key={opt.value}
              type="button"
              className={cn(
                "rounded-md px-2 py-1 text-[0.625rem] font-medium transition-colors",
                filters.status === opt.value
                  ? "bg-violet-500/15 text-violet-600 dark:text-violet-400"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
              onClick={() => setStatus(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Virtualized row (div-based, two sub-rows: long + short leg)
// ---------------------------------------------------------------------------

function PositionDataRow({
  position,
  isSelected,
  onSelect,
}: {
  position: Position;
  isSelected: boolean;
  onSelect: () => void;
}) {
  const unrealizedPnl =
    parseFloat(position.long_unrealized_pnl) +
    parseFloat(position.short_unrealized_pnl);
  const unrealizedStr = isNaN(unrealizedPnl) ? "0" : String(unrealizedPnl);
  const fundingPnl = parseFloat(position.total_funding_pnl) || 0;
  const totalPnl = (isNaN(unrealizedPnl) ? 0 : unrealizedPnl) + fundingPnl;
  const totalStr = String(totalPnl);

  return (
    <div
      className={cn(
        "group/row cursor-pointer border-b transition-colors hover:bg-muted/50 text-[11px] leading-tight",
        isSelected &&
          "bg-violet-500/5 dark:bg-violet-500/10 border-violet-500/20"
      )}
      onClick={onSelect}
    >
      <div className="flex">
        {/* Col 1: Symbol + Status */}
        <div className="flex items-center px-2 py-0.5 min-w-[100px] flex-[1]">
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-1 whitespace-nowrap font-medium text-xs">
              {isSelected && (
                <span className="h-1.5 w-1.5 rounded-full bg-violet-500 shrink-0" />
              )}
              {position.symbol}
            </div>
            <StatusBadge status={position.status} />
          </div>
        </div>

        {/* Cols 2: Pair (long/short legs) */}
        <div className="flex-[2.5] min-w-0 flex flex-col justify-center gap-0.5 py-0.5">
          {/* Long leg */}
          <div className="flex">
            <div className="flex-1 min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
                <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                  <ArrowUp
                    className="h-2.5 w-2.5 text-white"
                    strokeWidth={2.5}
                  />
                </span>
                <ExchangeBadge exchangeId={position.long_exchange_id} />
                <span className="flex-1" />
                <span className="font-mono tabular-nums text-muted-foreground">
                  {formatPrice(position.long_entry_price)}
                </span>
              </div>
            </div>
          </div>
          {/* Short leg */}
          <div className="flex">
            <div className="flex-1 min-w-0 px-2 whitespace-nowrap">
              <div className="flex items-center gap-1">
                <span className="inline-flex items-center justify-center h-3.5 w-3.5 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                  <ArrowDown
                    className="h-2.5 w-2.5 text-white"
                    strokeWidth={2.5}
                  />
                </span>
                <ExchangeBadge exchangeId={position.short_exchange_id} />
                <span className="flex-1" />
                <span className="font-mono tabular-nums text-muted-foreground">
                  {formatPrice(position.short_entry_price)}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Col 3: Size */}
        <div className="flex items-center justify-end px-2 py-0.5 min-w-[80px] flex-[0.8] whitespace-nowrap">
          <div className="flex flex-col items-end gap-0.5">
            <span className="font-mono tabular-nums">
              {position.size_type === "usd" ? "$" : ""}
              {formatQty(position.requested_size)}
            </span>
            <span className="text-[0.5625rem] text-muted-foreground uppercase">
              {position.size_type}
            </span>
          </div>
        </div>

        {/* Col 4: Unrealized P&L */}
        <div className="flex items-center justify-end px-2 py-0.5 min-w-[90px] flex-[1] whitespace-nowrap">
          <PnlValue value={unrealizedStr} />
        </div>

        {/* Col 5: Funding P&L */}
        <div className="flex items-center justify-end px-2 py-0.5 min-w-[90px] flex-[1] whitespace-nowrap">
          <PnlValue value={position.total_funding_pnl} />
        </div>

        {/* Col 6: Total P&L */}
        <div className="flex items-center justify-end px-2 py-0.5 min-w-[90px] flex-[1] whitespace-nowrap">
          <PnlValue value={totalStr} className="font-medium" />
        </div>

        {/* Col 7: Created */}
        <div className="flex items-center justify-end px-2 py-0.5 min-w-[90px] flex-[1] whitespace-nowrap">
          <span className="text-muted-foreground">
            {formatTimeShort(position.created_at)}
          </span>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Detail Panel — right side, similar to arbitrage detail panel
// ---------------------------------------------------------------------------

function DetailPanel({
  position,
  onClose,
}: {
  position: Position;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();

  const closeMutation = useMutation({
    mutationFn: closePosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deletePosition,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] });
    },
  });

  const canClose =
    position.status === "open" || position.status === "partially_open";
  const canDelete = position.status === "pending";

  // Computed metrics
  const longUnrealized = parseFloat(position.long_unrealized_pnl) || 0;
  const shortUnrealized = parseFloat(position.short_unrealized_pnl) || 0;
  const totalUnrealized = longUnrealized + shortUnrealized;
  const fundingPnl = parseFloat(position.total_funding_pnl) || 0;
  const totalPnl = totalUnrealized + fundingPnl;

  const longEntryFees = parseFloat(position.long_entry_fees) || 0;
  const shortEntryFees = parseFloat(position.short_entry_fees) || 0;
  const longCloseFees = parseFloat(position.long_close_fees) || 0;
  const shortCloseFees = parseFloat(position.short_close_fees) || 0;
  const totalEntryFees = longEntryFees + shortEntryFees;
  const totalCloseFees = longCloseFees + shortCloseFees;
  const totalFees = totalEntryFees + totalCloseFees;

  const longEntryCost = parseFloat(position.long_entry_cost) || 0;
  const shortEntryCost = parseFloat(position.short_entry_cost) || 0;

  return (
    <div className="shrink-0 border-l flex flex-col overflow-y-auto min-h-0 h-full w-full">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-3">
        <div>
          <div className="flex items-center gap-2">
            <span className="font-medium">{position.symbol}</span>
            <StatusBadge status={position.status} />
          </div>
          <div className="text-xs text-muted-foreground flex items-center gap-1.5 mt-0.5">
            <span className="inline-flex items-center gap-1">
              <TrendingUp className="h-3 w-3 text-emerald-500" />
              <ExchangeBadge exchangeId={position.long_exchange_id} />
            </span>
            <span className="text-muted-foreground/50">/</span>
            <span className="inline-flex items-center gap-1">
              <TrendingDown className="h-3 w-3 text-red-500" />
              <ExchangeBadge exchangeId={position.short_exchange_id} />
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          {canClose && (
            <Button
              variant="outline"
              size="sm"
              className="h-7 px-2 text-xs text-red-600 dark:text-red-400 border-red-500/30 hover:bg-red-500/10"
              disabled={closeMutation.isPending}
              onClick={(e) => {
                e.stopPropagation();
                closeMutation.mutate(position.id);
              }}
            >
              {closeMutation.isPending ? (
                <Loader2 className="h-3 w-3 animate-spin mr-1" />
              ) : (
                <X className="h-3 w-3 mr-1" />
              )}
              Close
            </Button>
          )}
          {canDelete && (
            <Button
              variant="outline"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground hover:text-red-600 hover:bg-red-500/10"
              disabled={deleteMutation.isPending}
              onClick={(e) => {
                e.stopPropagation();
                deleteMutation.mutate(position.id);
              }}
            >
              {deleteMutation.isPending ? (
                <Loader2 className="h-3 w-3 animate-spin mr-1" />
              ) : (
                <Trash2 className="h-3 w-3 mr-1" />
              )}
              Delete
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={onClose}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <Tabs defaultValue="overview" className="flex-1 min-h-0 gap-0">
        <TabsList className="mx-4 mt-3">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="orders">Orders</TabsTrigger>
          <TabsTrigger value="funding">Funding</TabsTrigger>
        </TabsList>

        {/* Overview tab */}
        <TabsContent
          value="overview"
          className="flex-1 min-h-0 overflow-y-auto"
        >
          <div className="p-4 space-y-5">
            {/* P&L Summary */}
            <div>
              <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
                <DollarSign className="inline h-3 w-3 mr-0.5 align-text-top" />
                Profit &amp; Loss
              </div>
              <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
                <span className="text-muted-foreground">Unrealized P&L</span>
                <PnlValue
                  value={String(totalUnrealized)}
                  className="text-right"
                />
                <span className="text-muted-foreground pl-3">Long</span>
                <PnlValue
                  value={position.long_unrealized_pnl}
                  className="text-right text-[0.625rem]"
                />
                <span className="text-muted-foreground pl-3">Short</span>
                <PnlValue
                  value={position.short_unrealized_pnl}
                  className="text-right text-[0.625rem]"
                />
                <span className="text-muted-foreground">Funding P&L</span>
                <PnlValue
                  value={position.total_funding_pnl}
                  className="text-right"
                />

                <span className="col-span-2 border-t border-border/30 my-1" />

                <span className="text-muted-foreground font-medium">
                  Total P&L
                </span>
                <PnlValue
                  value={String(totalPnl)}
                  className="text-right font-medium"
                />
              </div>
            </div>

            {/* Entry Comparison */}
            <div>
              <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
                <Activity className="inline h-3 w-3 mr-0.5 align-text-top" />
                Position Details
              </div>
              <table className="text-xs w-full">
                <thead>
                  <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                    <th className="pr-4 pb-1 text-left font-medium" />
                    <th className="pr-4 pb-1 text-right font-medium">
                      <span className="inline-flex items-center gap-1">
                        <TrendingUp className="h-3 w-3 text-emerald-500" />
                        Long
                      </span>
                    </th>
                    <th className="pb-1 text-right font-medium">
                      <span className="inline-flex items-center gap-1">
                        <TrendingDown className="h-3 w-3 text-red-500" />
                        Short
                      </span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-t border-border/30">
                    <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                      Exchange
                    </td>
                    <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      <ExchangeBadge exchangeId={position.long_exchange_id} />
                    </td>
                    <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      <ExchangeBadge exchangeId={position.short_exchange_id} />
                    </td>
                  </tr>
                  <tr className="border-t border-border/30">
                    <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                      Entry price
                    </td>
                    <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatPrice(position.long_entry_price)}
                    </td>
                    <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatPrice(position.short_entry_price)}
                    </td>
                  </tr>
                  <tr className="border-t border-border/30">
                    <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                      Mark price
                    </td>
                    <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatPrice(position.long_mark_price)}
                    </td>
                    <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatPrice(position.short_mark_price)}
                    </td>
                  </tr>
                  {(position.long_exit_price !== "0" ||
                    position.short_exit_price !== "0") && (
                    <tr className="border-t border-border/30">
                      <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                        Exit price
                      </td>
                      <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                        {formatPrice(position.long_exit_price)}
                      </td>
                      <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                        {formatPrice(position.short_exit_price)}
                      </td>
                    </tr>
                  )}
                  <tr className="border-t border-border/30">
                    <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                      Filled qty
                    </td>
                    <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatQty(position.long_filled_qty)}
                    </td>
                    <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatQty(position.short_filled_qty)}
                    </td>
                  </tr>
                  <tr className="border-t border-border/30">
                    <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                      Entry cost
                    </td>
                    <td className="pr-4 py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatUsdPlain(position.long_entry_cost)}
                    </td>
                    <td className="py-0.5 text-right font-mono tabular-nums whitespace-nowrap">
                      {formatUsdPlain(position.short_entry_cost)}
                    </td>
                  </tr>
                  <tr className="border-t border-border/30">
                    <td className="pr-4 py-0.5 text-muted-foreground whitespace-nowrap">
                      Unrealized
                    </td>
                    <td className="pr-4 py-0.5 text-right whitespace-nowrap">
                      <PnlValue
                        value={position.long_unrealized_pnl}
                        className="text-[0.625rem]"
                      />
                    </td>
                    <td className="py-0.5 text-right whitespace-nowrap">
                      <PnlValue
                        value={position.short_unrealized_pnl}
                        className="text-[0.625rem]"
                      />
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            {/* Fees breakdown */}
            <div>
              <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
                Fees
              </div>
              <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <span className="text-muted-foreground">Entry fees</span>
                <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
                  {formatUsdPlain(String(totalEntryFees))}
                </span>
                <span className="text-muted-foreground pl-3">Long</span>
                <span className="font-mono tabular-nums text-right text-muted-foreground text-[0.625rem]">
                  {formatUsdPlain(position.long_entry_fees)}
                </span>
                <span className="text-muted-foreground pl-3">Short</span>
                <span className="font-mono tabular-nums text-right text-muted-foreground text-[0.625rem]">
                  {formatUsdPlain(position.short_entry_fees)}
                </span>
                {totalCloseFees > 0 && (
                  <>
                    <span className="text-muted-foreground">Close fees</span>
                    <span className="font-mono tabular-nums text-right text-red-600 dark:text-red-400">
                      {formatUsdPlain(String(totalCloseFees))}
                    </span>
                    <span className="text-muted-foreground pl-3">Long</span>
                    <span className="font-mono tabular-nums text-right text-muted-foreground text-[0.625rem]">
                      {formatUsdPlain(position.long_close_fees)}
                    </span>
                    <span className="text-muted-foreground pl-3">Short</span>
                    <span className="font-mono tabular-nums text-right text-muted-foreground text-[0.625rem]">
                      {formatUsdPlain(position.short_close_fees)}
                    </span>
                  </>
                )}

                <span className="col-span-2 border-t border-border/30 my-1" />

                <span className="text-muted-foreground font-medium">
                  Total fees
                </span>
                <span className="font-mono tabular-nums text-right font-medium text-red-600 dark:text-red-400">
                  {formatUsdPlain(String(totalFees))}
                </span>
              </div>
            </div>

            {/* Size & Capital */}
            <div>
              <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
                Size &amp; Capital
              </div>
              <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <span className="text-muted-foreground">Requested size</span>
                <span className="font-mono tabular-nums text-right">
                  {position.size_type === "usd" ? "$" : ""}
                  {formatQty(position.requested_size)}
                  <span className="ml-1 text-[0.625rem] text-muted-foreground uppercase">
                    {position.size_type}
                  </span>
                </span>
                <span className="text-muted-foreground">Capital deployed</span>
                <span className="font-mono tabular-nums text-right">
                  {formatUsdPlain(String(longEntryCost + shortEntryCost))}
                </span>
              </div>
            </div>

            {/* Timing */}
            <div>
              <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
                <Clock className="inline h-3 w-3 mr-0.5 align-text-top" />
                Timing
              </div>
              <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <span className="text-muted-foreground">Created</span>
                <span className="font-mono tabular-nums text-right">
                  {formatTimeShort(position.created_at)}
                </span>
                {position.opened_at && (
                  <>
                    <span className="text-muted-foreground">Opened</span>
                    <span className="font-mono tabular-nums text-right">
                      {formatTimeShort(position.opened_at)}
                    </span>
                  </>
                )}
                {position.closed_at && (
                  <>
                    <span className="text-muted-foreground">Closed</span>
                    <span className="font-mono tabular-nums text-right">
                      {formatTimeShort(position.closed_at)}
                    </span>
                  </>
                )}
                <span className="text-muted-foreground">Duration</span>
                <span className="font-mono tabular-nums text-right">
                  {formatDuration(
                    position.opened_at || position.created_at,
                    position.closed_at
                  )}
                </span>
                <span className="text-muted-foreground">Last update</span>
                <span className="font-mono tabular-nums text-right">
                  {formatTimeShort(position.updated_at)}
                </span>
              </div>
            </div>

            {/* Error message if any */}
            {position.error_message && (
              <div>
                <div className="text-[0.625rem] font-medium uppercase tracking-wider text-red-600 dark:text-red-400 mb-2">
                  Error
                </div>
                <div className="text-xs text-red-600 dark:text-red-400 bg-red-500/5 rounded-md px-3 py-2">
                  {position.error_message}
                </div>
              </div>
            )}
          </div>
        </TabsContent>

        {/* Orders tab */}
        <TabsContent
          value="orders"
          className="flex-1 min-h-0 overflow-y-auto"
        >
          <OrdersPanel positionId={position.id} />
        </TabsContent>

        {/* Funding tab */}
        <TabsContent
          value="funding"
          className="flex-1 min-h-0 overflow-y-auto"
        >
          <FundingPanel positionId={position.id} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Orders panel (used in detail panel tab)
// ---------------------------------------------------------------------------

function OrdersPanel({ positionId }: { positionId: string }) {
  const { data: orders, isLoading } = useQuery({
    queryKey: ["position-orders", positionId],
    queryFn: () => fetchPositionOrders(positionId),
  });

  if (isLoading) {
    return (
      <div className="p-4 space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-8 w-full" />
        ))}
      </div>
    );
  }

  if (!orders || orders.length === 0) {
    return (
      <div className="p-4 text-xs text-muted-foreground">No orders yet.</div>
    );
  }

  return (
    <div className="p-4">
      <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
        {orders.length} order{orders.length !== 1 ? "s" : ""}
      </div>

      <table className="text-xs w-full">
        <thead>
          <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            <th className="pr-3 pb-1 text-left font-medium">Exchange</th>
            <th className="pr-3 pb-1 text-left font-medium">Leg</th>
            <th className="pr-3 pb-1 text-left font-medium">Action</th>
            <th className="pr-3 pb-1 text-right font-medium">Qty</th>
            <th className="pr-3 pb-1 text-right font-medium">Avg Price</th>
            <th className="pr-3 pb-1 text-right font-medium">Cost</th>
            <th className="pr-3 pb-1 text-right font-medium">Fee</th>
            <th className="pr-3 pb-1 text-left font-medium">Status</th>
            <th className="pb-1 text-left font-medium">Time</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((o: ExchangeOrder) => {
            const feeNum = parseFloat(o.fee_cost) || 0;
            return (
              <tr key={o.id} className="border-t border-border/30">
                <td className="pr-3 py-0.5">
                  <ExchangeBadge exchangeId={o.exchange_id} />
                </td>
                <td className="pr-3 py-0.5">
                  <span
                    className={cn(
                      "text-[0.625rem] font-medium uppercase",
                      o.leg === "long"
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {o.leg}
                  </span>
                </td>
                <td className="pr-3 py-0.5 text-[0.625rem] text-muted-foreground uppercase">
                  {o.action}
                </td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                  {formatQty(o.filled_qty)} / {formatQty(o.requested_qty)}
                </td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                  {formatPrice(o.average_price)}
                </td>
                <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                  {formatUsdPlain(o.cost)}
                </td>
                <td
                  className={cn(
                    "pr-3 py-0.5 text-right font-mono tabular-nums",
                    feeNum > 0 && "text-red-600 dark:text-red-400"
                  )}
                >
                  {formatUsdPlain(o.fee_cost)}
                </td>
                <td className="pr-3 py-0.5">
                  <span
                    className={cn(
                      "text-[0.625rem] font-medium",
                      o.status === "filled" &&
                        "text-emerald-600 dark:text-emerald-400",
                      o.status === "failed" &&
                        "text-red-600 dark:text-red-400",
                      o.status === "cancelled" &&
                        "text-amber-600 dark:text-amber-400"
                    )}
                  >
                    {o.status}
                  </span>
                </td>
                <td className="py-0.5 text-muted-foreground">
                  {formatTimeShort(o.created_at)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Funding payments panel (used in detail panel tab)
// ---------------------------------------------------------------------------

function FundingPanel({ positionId }: { positionId: string }) {
  const { data: funding, isLoading } = useQuery({
    queryKey: ["position-funding", positionId],
    queryFn: () => fetchPositionFunding(positionId),
  });

  if (isLoading) {
    return (
      <div className="p-4 space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-6 w-full" />
        ))}
      </div>
    );
  }

  if (!funding || funding.length === 0) {
    return (
      <div className="p-4 text-xs text-muted-foreground">
        No funding payments yet.
      </div>
    );
  }

  // Compute cumulative using reduce to avoid mutable variable
  const rows = funding.reduce<Array<FundingPayment & { cumulative: number }>>(
    (acc, f) => {
      const payment = parseFloat(f.payment) || 0;
      const prev = acc.length > 0 ? acc[acc.length - 1].cumulative : 0;
      acc.push({ ...f, cumulative: prev + payment });
      return acc;
    },
    []
  );
  const totalCumulative = rows.length > 0 ? rows[rows.length - 1].cumulative : 0;

  return (
    <div className="p-4">
      {/* Summary header */}
      <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground mb-2">
        {funding.length} payments &mdash; Net:{" "}
        <PnlValue value={String(totalCumulative)} className="text-[0.625rem]" />
      </div>

      <table className="text-xs w-full">
        <thead>
          <tr className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
            <th className="pr-3 pb-1 text-left font-medium">Exchange</th>
            <th className="pr-3 pb-1 text-left font-medium">Leg</th>
            <th className="pr-3 pb-1 text-right font-medium">Rate</th>
            <th className="pr-3 pb-1 text-right font-medium">Payment</th>
            <th className="pr-3 pb-1 text-right font-medium">Cum.</th>
            <th className="pb-1 text-left font-medium">Time</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(
            (f) => {
              const payment = parseFloat(f.payment);
              return (
                <tr key={f.id} className="border-t border-border/30">
                  <td className="pr-3 py-0.5">
                    <ExchangeBadge exchangeId={f.exchange_id} />
                  </td>
                  <td className="pr-3 py-0.5">
                    <span
                      className={cn(
                        "text-[0.625rem] font-medium uppercase",
                        f.leg === "long"
                          ? "text-emerald-600 dark:text-emerald-400"
                          : "text-red-600 dark:text-red-400"
                      )}
                    >
                      {f.leg}
                    </span>
                  </td>
                  <td className="pr-3 py-0.5 text-right font-mono tabular-nums">
                    {formatRate(f.rate)}
                  </td>
                  <td
                    className={cn(
                      "pr-3 py-0.5 text-right font-mono tabular-nums",
                      payment >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {formatUsd(f.payment)}
                  </td>
                  <td
                    className={cn(
                      "pr-3 py-0.5 text-right font-mono tabular-nums font-medium",
                      f.cumulative >= 0
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-red-600 dark:text-red-400"
                    )}
                  >
                    {formatUsd(String(f.cumulative))}
                  </td>
                  <td className="py-0.5 text-muted-foreground">
                    {formatTimeShort(f.funded_at)}
                  </td>
                </tr>
              );
            }
          )}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main table component
// ---------------------------------------------------------------------------

export function PositionsTable() {
  const [sort, setSort] = useState<SortState>({ key: "created", desc: true });
  const [selectedPositionId, setSelectedPositionId] = useState<string | null>(
    null
  );
  const [filters, setFilters] = useState<PositionFilters>(EMPTY_FILTERS);
  const [openPanel, setOpenPanel] = useState<"none" | "filters">("none");

  // Resizable detail panel
  const PANEL_MIN = 340;
  const PANEL_MAX = 700;
  const PANEL_DEFAULT = 420;
  const [panelWidth, setPanelWidth] = useState(PANEL_DEFAULT);
  const isDragging = useRef(false);
  const dragStartX = useRef(0);
  const dragStartWidth = useRef(0);

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (!isDragging.current) return;
      e.preventDefault();
      const delta = dragStartX.current - e.clientX;
      const newWidth = Math.min(
        PANEL_MAX,
        Math.max(PANEL_MIN, dragStartWidth.current + delta)
      );
      setPanelWidth(newWidth);
    };
    const onMouseUp = () => {
      if (!isDragging.current) return;
      isDragging.current = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
    return () => {
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };
  }, []);

  const startResize = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      isDragging.current = true;
      dragStartX.current = e.clientX;
      dragStartWidth.current = panelWidth;
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
    },
    [panelWidth]
  );

  const {
    data: positions,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["positions", filters.status],
    queryFn: () =>
      fetchPositions(filters.status ? { status: filters.status } : undefined),
  });

  const activeFilterCount = useMemo(() => {
    let count = 0;
    if (filters.symbols.length > 0) count++;
    if (filters.exchanges.length > 0) count++;
    if (filters.status) count++;
    return count;
  }, [filters]);

  const filteredData = useMemo(
    () => (positions ? filterPositions(positions, filters) : []),
    [positions, filters]
  );

  const sortedData = useMemo(
    () => sortPositions(filteredData, sort),
    [filteredData, sort]
  );

  // Derived lists for filter suggestions
  const availableSymbols = useMemo(() => {
    if (!positions) return [];
    return [...new Set(positions.map((p) => p.symbol))].sort();
  }, [positions]);

  const availableExchanges = useMemo(() => {
    if (!positions) return [];
    const set = new Set<string>();
    for (const p of positions) {
      set.add(p.long_exchange_id);
      set.add(p.short_exchange_id);
    }
    return [...set].sort();
  }, [positions]);

  const selectedPosition = useMemo(() => {
    if (!selectedPositionId || !sortedData) return null;
    return sortedData.find((p) => p.id === selectedPositionId) ?? null;
  }, [selectedPositionId, sortedData]);

  const toggleSort = useCallback((key: SortKey) => {
    setSort((prev) => ({
      key,
      desc: prev.key === key ? !prev.desc : true,
    }));
  }, []);

  const selectRow = useCallback(
    (p: Position) => {
      setSelectedPositionId((prev) => (prev === p.id ? null : p.id));
    },
    []
  );

  const togglePanel = useCallback((panel: "filters") => {
    setOpenPanel((prev) => (prev === panel ? "none" : panel));
  }, []);

  const setSymbols = useCallback(
    (s: string[]) => setFilters((prev) => ({ ...prev, symbols: s })),
    []
  );
  const setExchanges = useCallback(
    (s: string[]) => setFilters((prev) => ({ ...prev, exchanges: s })),
    []
  );
  const setStatus = useCallback(
    (s: string) => setFilters((prev) => ({ ...prev, status: s })),
    []
  );
  const clearFilters = useCallback(() => setFilters(EMPTY_FILTERS), []);

  // Virtualization
  const ROW_HEIGHT = 40;
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: sortedData.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  });

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">
          Failed to load positions: {error.message}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0">
      {/* Left: table area */}
      <div className="flex-1 min-w-0 flex flex-col">
        {/* Toolbar & filters */}
        <div className="px-4 pt-3 pb-2 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-screen-xl space-y-2">
            <div className="flex items-center justify-between gap-4">
              <div className="text-xs text-muted-foreground">
                Funding rate arbitrage positions
              </div>
              {positions && (
                <div className="flex items-center gap-2">
                  {activeFilterCount > 0 &&
                    filteredData.length !== positions.length && (
                      <Badge
                        variant="outline"
                        className="font-mono text-muted-foreground"
                      >
                        {filteredData.length} / {positions.length}
                      </Badge>
                    )}
                  <Badge variant="outline" className="font-mono">
                    {filteredData.length} position
                    {filteredData.length !== 1 ? "s" : ""}
                  </Badge>
                </div>
              )}
            </div>

            {/* Toolbar */}
            <div className="space-y-2">
              <div className="flex items-center gap-2 flex-wrap">
                {/* Filters button */}
                <Button
                  variant="outline"
                  size="sm"
                  className={cn(
                    "h-7 gap-1.5 text-xs",
                    activeFilterCount > 0 &&
                      "border-primary/40 bg-primary/5 text-primary"
                  )}
                  onClick={() => togglePanel("filters")}
                >
                  <SlidersHorizontal className="h-3 w-3" />
                  Filters
                  {activeFilterCount > 0 && (
                    <Badge
                      variant="default"
                      className="ml-0.5 h-4 min-w-4 px-1 text-[0.5rem]"
                    >
                      {activeFilterCount}
                    </Badge>
                  )}
                  {openPanel === "filters" ? (
                    <ChevronUp className="h-3 w-3" />
                  ) : (
                    <ChevronDown className="h-3 w-3" />
                  )}
                </Button>

                {activeFilterCount > 0 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 gap-1 text-xs text-muted-foreground"
                    onClick={clearFilters}
                  >
                    <RotateCcw className="h-3 w-3" />
                    Clear all
                  </Button>
                )}
              </div>

              {openPanel === "filters" && (
                <FilterPanel
                  filters={filters}
                  setSymbols={setSymbols}
                  setExchanges={setExchanges}
                  setStatus={setStatus}
                  availableSymbols={availableSymbols}
                  availableExchanges={availableExchanges}
                />
              )}
            </div>
          </div>
        </div>

        {/* Table with sticky header + virtualized rows */}
        <div className="flex-1 min-h-0 px-4 pb-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-screen-xl h-full">
            <div className="rounded-lg border flex flex-col overflow-hidden h-full">
              {/* Header */}
              <div className="flex items-center border-b bg-background text-xs text-foreground font-medium">
                <div className="px-2 py-1.5 min-w-[100px] flex-[1]">
                  <SortButton
                    label="Symbol"
                    sortKey="symbol"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
                <div className="px-2 py-1.5 flex-[2.5] min-w-0">Pair</div>
                <div className="px-2 py-1.5 min-w-[80px] flex-[0.8] text-right">
                  Size
                </div>
                <div className="px-2 py-1.5 min-w-[90px] flex-[1] text-right">
                  <SortButton
                    label="Unrealized"
                    sortKey="pnl"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
                <div className="px-2 py-1.5 min-w-[90px] flex-[1] text-right">
                  <SortButton
                    label="Funding"
                    sortKey="funding"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
                <div className="px-2 py-1.5 min-w-[90px] flex-[1] text-right">
                  <SortButton
                    label="Total"
                    sortKey="total"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
                <div className="px-2 py-1.5 min-w-[90px] flex-[1] text-right">
                  <SortButton
                    label="Created"
                    sortKey="created"
                    current={sort}
                    onToggle={toggleSort}
                  />
                </div>
              </div>

              {/* Scrollable body */}
              {isLoading ? (
                <div className="p-2 space-y-2">
                  {Array.from({ length: 8 }).map((_, i) => (
                    <div key={i} className="flex gap-3">
                      <Skeleton className="h-6 w-20" />
                      <Skeleton className="h-6 flex-1" />
                      <Skeleton className="h-6 w-16" />
                      <Skeleton className="h-6 w-16" />
                      <Skeleton className="h-6 w-16" />
                      <Skeleton className="h-6 w-16" />
                      <Skeleton className="h-6 w-20" />
                    </div>
                  ))}
                </div>
              ) : sortedData.length === 0 ? (
                <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground p-8">
                  {activeFilterCount > 0
                    ? "No positions match the current filters."
                    : "No positions found. Open a position from the Arbitrage page."}
                </div>
              ) : (
                <div
                  ref={scrollContainerRef}
                  className="flex-1 overflow-y-auto min-h-0"
                >
                  <div
                    style={{
                      height: virtualizer.getTotalSize(),
                      position: "relative",
                    }}
                  >
                    {virtualizer.getVirtualItems().map((virtualRow) => {
                      const pos = sortedData[virtualRow.index];
                      return (
                        <div
                          key={pos.id}
                          data-index={virtualRow.index}
                          ref={virtualizer.measureElement}
                          style={{
                            position: "absolute",
                            top: 0,
                            left: 0,
                            width: "100%",
                            transform: `translateY(${virtualRow.start}px)`,
                          }}
                        >
                          <PositionDataRow
                            position={pos}
                            isSelected={selectedPositionId === pos.id}
                            onSelect={() => selectRow(pos)}
                          />
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Right: detail panel */}
      {selectedPosition && (
        <>
          {/* Resize handle */}
          <div
            className="shrink-0 w-1 cursor-col-resize hover:bg-violet-500/30 active:bg-violet-500/50 transition-colors"
            onMouseDown={startResize}
          />
          <div style={{ width: panelWidth }} className="shrink-0">
            <DetailPanel
              key={selectedPosition.id}
              position={selectedPosition}
              onClose={() => setSelectedPositionId(null)}
            />
          </div>
        </>
      )}
    </div>
  );
}
