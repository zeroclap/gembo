"use client";

import { useState, useCallback, useRef, useEffect, type KeyboardEvent } from "react";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { X } from "lucide-react";
import type { ArbitrageFilters, ArbitrageFiltersActions } from "@/hooks/use-arbitrage-filters";

// ---------------------------------------------------------------------------
// Tag Input with autocomplete dropdown
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

  // Filter suggestions: exclude already-selected tags, match input substring
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
    (e: KeyboardEvent<HTMLInputElement>) => {
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
        setHighlightIndex((prev) => (prev < filtered.length - 1 ? prev + 1 : 0));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlightIndex((prev) => (prev > 0 ? prev - 1 : filtered.length - 1));
      } else if (e.key === "Escape") {
        setShowDropdown(false);
        setHighlightIndex(-1);
      } else if (e.key === "Backspace" && !input && tags.length > 0) {
        removeTag(tags.length - 1);
      }
    },
    [input, tags, filtered, highlightIndex, addTag, removeTag]
  );

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
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

      {/* Autocomplete dropdown */}
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
                // mouseDown instead of click so it fires before blur
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
// Range Input – two number inputs for min/max bounds
// ---------------------------------------------------------------------------

function RangeInput({
  label,
  minValue,
  maxValue,
  onMinChange,
  onMaxChange,
  minPlaceholder = "Min",
  maxPlaceholder = "Max",
}: {
  label: string;
  minValue: string;
  maxValue: string;
  onMinChange: (value: string) => void;
  onMaxChange: (value: string) => void;
  minPlaceholder?: string;
  maxPlaceholder?: string;
}) {
  const isActive = minValue !== "" || maxValue !== "";

  return (
    <div className="space-y-1">
      <label
        className={cn(
          "text-[0.625rem] font-medium uppercase tracking-wider",
          isActive ? "text-foreground" : "text-muted-foreground"
        )}
      >
        {label}
      </label>
      <div className="flex items-center gap-1.5">
        <input
          type="text"
          inputMode="decimal"
          value={minValue}
          onChange={(e) => onMinChange(e.target.value)}
          placeholder={minPlaceholder}
          className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
        />
        <span className="text-[0.625rem] text-muted-foreground">&ndash;</span>
        <input
          type="text"
          inputMode="decimal"
          value={maxValue}
          onChange={(e) => onMaxChange(e.target.value)}
          placeholder={maxPlaceholder}
          className="h-7 w-full min-w-0 rounded-md border bg-background px-2 text-xs tabular-nums outline-none ring-ring placeholder:text-muted-foreground focus-visible:ring-2"
        />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Filter Panel (content only, toggle is in parent)
// ---------------------------------------------------------------------------

interface FilterPanelProps {
  filters: ArbitrageFilters;
  actions: ArbitrageFiltersActions;
  availableSymbols: string[];
  availableExchanges: string[];
}

export function FilterPanel({
  filters,
  actions,
  availableSymbols,
  availableExchanges,
}: FilterPanelProps) {
  return (
    <div className="grid gap-3 rounded-lg border bg-card/50 p-3 sm:grid-cols-2 lg:grid-cols-3">
      {/* Symbol tags */}
      <div className="space-y-1">
        <label
          className={cn(
            "text-[0.625rem] font-medium uppercase tracking-wider",
            filters.symbols.length > 0 ? "text-foreground" : "text-muted-foreground"
          )}
        >
          Symbol
        </label>
        <TagInput
          tags={filters.symbols}
          onChange={actions.setSymbols}
          placeholder="Type symbol, press Enter..."
          suggestions={availableSymbols}
        />
      </div>

      {/* Exchange tags */}
      <div className="space-y-1">
        <label
          className={cn(
            "text-[0.625rem] font-medium uppercase tracking-wider",
            filters.exchanges.length > 0 ? "text-foreground" : "text-muted-foreground"
          )}
        >
          Exchange
        </label>
        <TagInput
          tags={filters.exchanges}
          onChange={actions.setExchanges}
          placeholder="Type exchange, press Enter..."
          suggestions={availableExchanges}
        />
      </div>

      {/* Volume range */}
      <RangeInput
        label="Min Leg Volume 24h, M"
        minValue={filters.volumeMin}
        maxValue={filters.volumeMax}
        onMinChange={actions.setVolumeMin}
        onMaxChange={actions.setVolumeMax}
        minPlaceholder="0"
        maxPlaceholder="No limit"
      />

      {/* Open spread range */}
      <RangeInput
        label="Open Spread, %"
        minValue={filters.spreadMin}
        maxValue={filters.spreadMax}
        onMinChange={actions.setSpreadMin}
        onMaxChange={actions.setSpreadMax}
      />

      {/* Funding spread (8h net rate) */}
      <RangeInput
        label="Funding Spread (8h), %"
        minValue={filters.fundingSpreadMin}
        maxValue={filters.fundingSpreadMax}
        onMinChange={actions.setFundingSpreadMin}
        onMaxChange={actions.setFundingSpreadMax}
      />

      {/* Full loop net */}
      <RangeInput
        label="Full Loop Net, %"
        minValue={filters.fundingLoopMin}
        maxValue={filters.fundingLoopMax}
        onMinChange={actions.setFundingLoopMin}
        onMaxChange={actions.setFundingLoopMax}
      />
    </div>
  );
}
