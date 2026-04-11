"use client";

import { useState, useMemo, useCallback } from "react";
import { useQuery, useQueries } from "@tanstack/react-query";
import {
  fetchAllTickers,
  fetchExchangeFundingRates,
  fetchExchanges,
  type FundingRate,
} from "@/lib/api";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { ExchangeBadge } from "@/components/monitor/shared";
import { formatPrice, formatVolume } from "@/lib/format";
import {
  ArrowUp,
  ArrowDown,
  ArrowLeft,
  ChevronUp,
  ChevronDown,
  ChevronsUpDown,
} from "lucide-react";
import { cn } from "@/lib/utils";

export interface PairSelection {
  symbol: string;
  longExchange: string;
  shortExchange: string;
}

interface PairCommandProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelect: (pair: PairSelection) => void;
  currentSymbol?: string;
}

type Step = "symbol" | "pair";

interface ExchangeInfo {
  exchangeId: string;
  price: string;
  volume: string;
  fundingRate: number | null; // rate as raw decimal (e.g. 0.0001)
  rate8h: number | null; // normalized to 8h
}

interface PairPermutation {
  long: ExchangeInfo;
  short: ExchangeInfo;
  spreadPct: number;
  fundingSpreadPct: number | null; // short funding - long funding (as %)
}

/** Summary metrics for the symbol list (best values across all pairs) */
interface SymbolSummary {
  symbol: string;
  totalVolume: number;
  exchangeCount: number;
  // Best funding pair (highest absolute funding spread)
  bestLongFunding: number | null; // 8h rate
  bestShortFunding: number | null; // 8h rate
  bestFundingRate: number | null; // net spread as % (short - long)
  // Best price spread (highest absolute price spread, may differ from funding pair)
  bestPriceSpread: number | null;
  // Keep tickers for pair-step drill-down
  tickers: { exchange_id: string; price: string; volume_24h: string }[];
}

type SymbolSortKey =
  | "symbol"
  | "volume"
  | "longFunding"
  | "shortFunding"
  | "fundingRate"
  | "priceSpread";
type PairSortKey =
  | "long"
  | "short"
  | "priceSpread"
  | "longFunding"
  | "shortFunding"
  | "fundingSpread"
  | "longPrice"
  | "shortPrice";
type SortDir = "asc" | "desc";

interface SortState<K> {
  key: K;
  dir: SortDir;
}

function SortHeader({
  label,
  active,
  dir,
  onClick,
  align = "left",
}: {
  label: string;
  active: boolean;
  dir: SortDir;
  onClick: () => void;
  align?: "left" | "right";
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex items-center gap-0.5 cursor-pointer select-none hover:text-foreground transition-colors",
        align === "right" && "justify-end",
        active ? "text-foreground" : "text-muted-foreground"
      )}
    >
      <span>{label}</span>
      {active ? (
        dir === "asc" ? (
          <ChevronUp className="h-2.5 w-2.5 shrink-0" />
        ) : (
          <ChevronDown className="h-2.5 w-2.5 shrink-0" />
        )
      ) : (
        <ChevronsUpDown className="h-2.5 w-2.5 shrink-0 opacity-40" />
      )}
    </button>
  );
}

function formatFundingPct(rate8h: number | null): string {
  if (rate8h === null) return "-";
  const pct = rate8h * 100;
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
}

function formatSpreadPct(pct: number | null): string {
  if (pct === null) return "-";
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
}

function fundingColor(val: number | null): string {
  if (val === null) return "text-muted-foreground";
  if (val > 0) return "text-emerald-500";
  if (val < 0) return "text-red-500";
  return "text-muted-foreground";
}

export function PairCommand({
  open,
  onOpenChange,
  onSelect,
  currentSymbol,
}: PairCommandProps) {
  const [step, setStep] = useState<Step>("symbol");
  const [selectedSymbol, setSelectedSymbol] = useState<string>("");
  const [pairPermutations, setPairPermutations] = useState<PairPermutation[]>(
    []
  );

  const [symbolSort, setSymbolSort] = useState<SortState<SymbolSortKey>>({
    key: "fundingRate",
    dir: "desc",
  });
  const [pairSort, setPairSort] = useState<SortState<PairSortKey>>({
    key: "fundingSpread",
    dir: "desc",
  });

  const { data: tickers } = useQuery({
    queryKey: ["all-tickers"],
    queryFn: fetchAllTickers,
  });

  const { data: exchanges } = useQuery({
    queryKey: ["exchanges"],
    queryFn: fetchExchanges,
  });

  // Fetch funding rates for all exchanges
  const fundingQueries = useQueries({
    queries: (exchanges ?? []).map((ex) => ({
      queryKey: ["funding-rates", ex],
      queryFn: () => fetchExchangeFundingRates(ex),
      staleTime: 10_000,
    })),
  });

  // Build a lookup: fundingMap[exchangeId][symbol] => FundingRate
  const fundingDataKey = fundingQueries
    .map((q) => q.dataUpdatedAt)
    .join(",");
  const fundingMap = useMemo(() => {
    const map: Record<string, Record<string, FundingRate>> = {};
    if (!exchanges) return map;
    for (let i = 0; i < exchanges.length; i++) {
      const ex = exchanges[i];
      const query = fundingQueries[i];
      if (query?.data) {
        map[ex] = {};
        for (const fr of query.data) {
          map[ex][fr.symbol] = fr;
        }
      }
    }
    return map;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exchanges, fundingDataKey]);

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (nextOpen) {
        setStep("symbol");
        setSelectedSymbol("");
        setPairPermutations([]);
      }
      onOpenChange(nextOpen);
    },
    [onOpenChange]
  );

  /** Helper: get ExchangeInfo for all tickers of a symbol */
  const getExchangeInfos = useCallback(
    (
      tickerList: { exchange_id: string; price: string; volume_24h: string }[],
      symbol: string
    ): ExchangeInfo[] =>
      tickerList.map((t) => {
        const fr = fundingMap[t.exchange_id]?.[symbol];
        const rate = fr ? parseFloat(fr.rate) : null;
        const intervalHours = fr?.interval_hours ?? 8;
        const rate8h =
          rate !== null && isFinite(rate) ? rate * (8 / intervalHours) : null;
        return {
          exchangeId: t.exchange_id,
          price: t.price,
          volume: t.volume_24h,
          fundingRate: rate,
          rate8h,
        };
      }),
    [fundingMap]
  );

  /** Helper: compute all pair permutations for a set of exchanges */
  const computePermutations = useCallback(
    (exchangeInfos: ExchangeInfo[]): PairPermutation[] => {
      const perms: PairPermutation[] = [];
      for (const long of exchangeInfos) {
        for (const short of exchangeInfos) {
          if (long.exchangeId === short.exchangeId) continue;
          const longPrice = parseFloat(long.price) || 0;
          const shortPrice = parseFloat(short.price) || 0;
          const spreadPct =
            longPrice > 0 ? ((shortPrice - longPrice) / longPrice) * 100 : 0;
          const fundingSpreadPct =
            long.rate8h !== null && short.rate8h !== null
              ? (short.rate8h - long.rate8h) * 100
              : null;
          perms.push({ long, short, spreadPct, fundingSpreadPct });
        }
      }
      return perms;
    },
    []
  );

  // Pre-compute symbol summaries with best funding pair + best price spread
  const symbolSummaries = useMemo(() => {
    if (!tickers) return [];
    return tickers
      .map((row): SymbolSummary | null => {
        if (row.tickers.length < 2) return null;
        const totalVolume = row.tickers.reduce(
          (sum, t) => sum + (parseFloat(t.volume_24h) || 0),
          0
        );
        const infos = getExchangeInfos(row.tickers, row.symbol);
        const perms = computePermutations(infos);

        // Best funding pair: highest funding spread (short - long)
        let bestFundingPerm: PairPermutation | null = null;
        for (const p of perms) {
          if (p.fundingSpreadPct === null) continue;
          if (
            !bestFundingPerm ||
            bestFundingPerm.fundingSpreadPct === null ||
            Math.abs(p.fundingSpreadPct) >
              Math.abs(bestFundingPerm.fundingSpreadPct)
          ) {
            bestFundingPerm = p;
          }
        }

        // Best price spread: highest absolute price spread
        let bestPriceSpread: number | null = null;
        for (const p of perms) {
          if (
            bestPriceSpread === null ||
            Math.abs(p.spreadPct) > Math.abs(bestPriceSpread)
          ) {
            bestPriceSpread = p.spreadPct;
          }
        }

        return {
          symbol: row.symbol,
          totalVolume,
          exchangeCount: row.tickers.length,
          bestLongFunding: bestFundingPerm?.long.rate8h ?? null,
          bestShortFunding: bestFundingPerm?.short.rate8h ?? null,
          bestFundingRate: bestFundingPerm?.fundingSpreadPct ?? null,
          bestPriceSpread,
          tickers: row.tickers,
        };
      })
      .filter((s): s is SymbolSummary => s !== null);
  }, [tickers, getExchangeInfos, computePermutations]);

  const sortedSymbols = useMemo(() => {
    const rows = [...symbolSummaries];
    const { key, dir } = symbolSort;
    rows.sort((a, b) => {
      let cmp = 0;
      switch (key) {
        case "symbol":
          cmp = a.symbol.localeCompare(b.symbol);
          break;
        case "volume":
          cmp = a.totalVolume - b.totalVolume;
          break;
        case "longFunding":
          cmp =
            (a.bestLongFunding ?? -Infinity) -
            (b.bestLongFunding ?? -Infinity);
          break;
        case "shortFunding":
          cmp =
            (a.bestShortFunding ?? -Infinity) -
            (b.bestShortFunding ?? -Infinity);
          break;
        case "fundingRate":
          cmp =
            (a.bestFundingRate ?? -Infinity) -
            (b.bestFundingRate ?? -Infinity);
          break;
        case "priceSpread":
          cmp =
            Math.abs(a.bestPriceSpread ?? 0) -
            Math.abs(b.bestPriceSpread ?? 0);
          break;
      }
      return dir === "asc" ? cmp : -cmp;
    });
    return rows;
  }, [symbolSummaries, symbolSort]);

  const handleSymbolSelect = useCallback(
    (symbol: string) => {
      const row = tickers?.find((r) => r.symbol === symbol);
      if (!row) return;
      const infos = getExchangeInfos(row.tickers, symbol);
      const perms = computePermutations(infos);

      setSelectedSymbol(symbol);
      setPairPermutations(perms);
      setPairSort({ key: "fundingSpread", dir: "desc" });
      setStep("pair");
    },
    [tickers, getExchangeInfos, computePermutations]
  );

  const sortedPermutations = useMemo(() => {
    const sorted = [...pairPermutations];
    const { key, dir } = pairSort;
    sorted.sort((a, b) => {
      let cmp = 0;
      switch (key) {
        case "long":
          cmp = a.long.exchangeId.localeCompare(b.long.exchangeId);
          break;
        case "short":
          cmp = a.short.exchangeId.localeCompare(b.short.exchangeId);
          break;
        case "priceSpread":
          cmp = a.spreadPct - b.spreadPct;
          break;
        case "longFunding":
          cmp = (a.long.rate8h ?? -Infinity) - (b.long.rate8h ?? -Infinity);
          break;
        case "shortFunding":
          cmp = (a.short.rate8h ?? -Infinity) - (b.short.rate8h ?? -Infinity);
          break;
        case "fundingSpread":
          cmp =
            (a.fundingSpreadPct ?? -Infinity) -
            (b.fundingSpreadPct ?? -Infinity);
          break;
        case "longPrice":
          cmp =
            (parseFloat(a.long.price) || 0) - (parseFloat(b.long.price) || 0);
          break;
        case "shortPrice":
          cmp =
            (parseFloat(a.short.price) || 0) -
            (parseFloat(b.short.price) || 0);
          break;
      }
      return dir === "asc" ? cmp : -cmp;
    });
    return sorted;
  }, [pairPermutations, pairSort]);

  const handlePairSelect = useCallback(
    (perm: PairPermutation) => {
      onSelect({
        symbol: selectedSymbol,
        longExchange: perm.long.exchangeId,
        shortExchange: perm.short.exchangeId,
      });
      onOpenChange(false);
    },
    [selectedSymbol, onSelect, onOpenChange]
  );

  const handleBack = useCallback(() => {
    setStep("symbol");
    setSelectedSymbol("");
    setPairPermutations([]);
  }, []);

  function toggleSymbolSort(key: SymbolSortKey) {
    setSymbolSort((prev) =>
      prev.key === key
        ? { key, dir: prev.dir === "asc" ? "desc" : "asc" }
        : {
            key,
            dir: key === "symbol" ? "asc" : "desc",
          }
    );
  }

  function togglePairSort(key: PairSortKey) {
    setPairSort((prev) =>
      prev.key === key
        ? { key, dir: prev.dir === "asc" ? "desc" : "asc" }
        : { key, dir: key === "long" || key === "short" ? "asc" : "desc" }
    );
  }

  const symbolGridCols =
    "grid-cols-[1fr_80px_72px_72px_72px_72px]";
  const pairGridCols =
    "grid-cols-[minmax(0,1fr)_minmax(0,1fr)_72px_72px_72px_72px_72px]";

  return (
    <CommandDialog
      open={open}
      onOpenChange={handleOpenChange}
      className="sm:max-w-3xl"
    >
      <CommandInput
        placeholder={
          step === "symbol"
            ? "Search symbols..."
            : `Select pair for ${selectedSymbol}`
        }
      />
      <CommandList className="max-h-96 overflow-hidden">
        <CommandEmpty>No results found.</CommandEmpty>

        {step === "symbol" && (
          <CommandGroup>
            {/* Sticky table header */}
            <div
              className={cn(
                "grid gap-x-2 px-2.5 py-1 text-[0.5625rem] font-medium uppercase tracking-wider sticky top-0 z-10 bg-popover border-b border-border/40",
                symbolGridCols
              )}
            >
              <SortHeader
                label="Symbol"
                active={symbolSort.key === "symbol"}
                dir={symbolSort.dir}
                onClick={() => toggleSymbolSort("symbol")}
              />
              <SortHeader
                label="Volume"
                active={symbolSort.key === "volume"}
                dir={symbolSort.dir}
                onClick={() => toggleSymbolSort("volume")}
                align="right"
              />
              <SortHeader
                label="L Fund"
                active={symbolSort.key === "longFunding"}
                dir={symbolSort.dir}
                onClick={() => toggleSymbolSort("longFunding")}
                align="right"
              />
              <SortHeader
                label="S Fund"
                active={symbolSort.key === "shortFunding"}
                dir={symbolSort.dir}
                onClick={() => toggleSymbolSort("shortFunding")}
                align="right"
              />
              <SortHeader
                label="F Rate"
                active={symbolSort.key === "fundingRate"}
                dir={symbolSort.dir}
                onClick={() => toggleSymbolSort("fundingRate")}
                align="right"
              />
              <SortHeader
                label="P Spread"
                active={symbolSort.key === "priceSpread"}
                dir={symbolSort.dir}
                onClick={() => toggleSymbolSort("priceSpread")}
                align="right"
              />
            </div>
            <div className="overflow-y-auto max-h-[calc(24rem-3rem)]">
              {sortedSymbols.map((row) => (
                <CommandItem
                  key={row.symbol}
                  value={row.symbol}
                  onSelect={() => handleSymbolSelect(row.symbol)}
                >
                  <div
                    className={cn(
                      "grid gap-x-2 items-center w-full",
                      symbolGridCols
                    )}
                  >
                    <span
                      className={cn(
                        "font-medium text-sm truncate",
                        currentSymbol === row.symbol && "text-primary"
                      )}
                    >
                      {row.symbol}
                    </span>
                    <span className="text-right text-[0.625rem] text-muted-foreground font-mono tabular-nums">
                      {formatVolume(String(row.totalVolume))}
                    </span>
                    <span
                      className={cn(
                        "text-right font-mono tabular-nums text-[0.625rem]",
                        fundingColor(row.bestLongFunding)
                      )}
                    >
                      {formatFundingPct(row.bestLongFunding)}
                    </span>
                    <span
                      className={cn(
                        "text-right font-mono tabular-nums text-[0.625rem]",
                        fundingColor(row.bestShortFunding)
                      )}
                    >
                      {formatFundingPct(row.bestShortFunding)}
                    </span>
                    <span
                      className={cn(
                        "text-right font-mono tabular-nums text-[0.625rem] font-medium",
                        fundingColor(row.bestFundingRate)
                      )}
                    >
                      {formatSpreadPct(row.bestFundingRate)}
                    </span>
                    <span
                      className={cn(
                        "text-right font-mono tabular-nums text-[0.625rem]",
                        fundingColor(row.bestPriceSpread)
                      )}
                    >
                      {formatSpreadPct(row.bestPriceSpread)}
                    </span>
                  </div>
                </CommandItem>
              ))}
            </div>
          </CommandGroup>
        )}

        {step === "pair" && (
          <>
            <CommandGroup>
              <CommandItem onSelect={handleBack} className="text-xs">
                <ArrowLeft className="mr-2 h-3 w-3 opacity-50" />
                <span className="text-muted-foreground">
                  Back to symbol selection
                </span>
              </CommandItem>
            </CommandGroup>
            <CommandGroup
              heading={
                <span>
                  <span className="font-medium text-foreground">
                    {selectedSymbol}
                  </span>
                  <span className="ml-2 font-normal text-muted-foreground">
                    {pairPermutations.length} pairs
                  </span>
                </span>
              }
            >
              {/* Sticky table header */}
              <div
                className={cn(
                  "grid gap-x-2 px-2.5 py-1 text-[0.5625rem] font-medium uppercase tracking-wider sticky top-0 z-10 bg-popover border-b border-border/40",
                  pairGridCols
                )}
              >
                <SortHeader
                  label="Long"
                  active={pairSort.key === "long"}
                  dir={pairSort.dir}
                  onClick={() => togglePairSort("long")}
                />
                <SortHeader
                  label="Short"
                  active={pairSort.key === "short"}
                  dir={pairSort.dir}
                  onClick={() => togglePairSort("short")}
                />
                <SortHeader
                  label="L Fund"
                  active={pairSort.key === "longFunding"}
                  dir={pairSort.dir}
                  onClick={() => togglePairSort("longFunding")}
                  align="right"
                />
                <SortHeader
                  label="S Fund"
                  active={pairSort.key === "shortFunding"}
                  dir={pairSort.dir}
                  onClick={() => togglePairSort("shortFunding")}
                  align="right"
                />
                <SortHeader
                  label="F Spread"
                  active={pairSort.key === "fundingSpread"}
                  dir={pairSort.dir}
                  onClick={() => togglePairSort("fundingSpread")}
                  align="right"
                />
                <SortHeader
                  label="P Spread"
                  active={pairSort.key === "priceSpread"}
                  dir={pairSort.dir}
                  onClick={() => togglePairSort("priceSpread")}
                  align="right"
                />
                <span className="text-right text-muted-foreground">
                  L Price
                </span>
              </div>
              <div className="overflow-y-auto max-h-[calc(24rem-6.5rem)]">
                {sortedPermutations.map((perm) => (
                  <CommandItem
                    key={`${perm.long.exchangeId}-${perm.short.exchangeId}`}
                    value={`${perm.long.exchangeId} ${perm.short.exchangeId}`}
                    onSelect={() => handlePairSelect(perm)}
                  >
                    <div
                      className={cn(
                        "grid gap-x-2 items-center w-full",
                        pairGridCols
                      )}
                    >
                      {/* Long exchange */}
                      <div className="flex items-center gap-1.5 min-w-0">
                        <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-emerald-600 dark:bg-emerald-500 shrink-0">
                          <ArrowUp
                            className="h-3 w-3 text-white"
                            strokeWidth={2.5}
                          />
                        </span>
                        <ExchangeBadge exchangeId={perm.long.exchangeId} />
                      </div>

                      {/* Short exchange */}
                      <div className="flex items-center gap-1.5 min-w-0">
                        <span className="inline-flex items-center justify-center h-4 w-4 rounded-sm bg-red-600 dark:bg-red-500 shrink-0">
                          <ArrowDown
                            className="h-3 w-3 text-white"
                            strokeWidth={2.5}
                          />
                        </span>
                        <ExchangeBadge exchangeId={perm.short.exchangeId} />
                      </div>

                      {/* Long funding rate (8h) */}
                      <span
                        className={cn(
                          "text-right font-mono tabular-nums text-[0.625rem]",
                          fundingColor(perm.long.rate8h)
                        )}
                      >
                        {formatFundingPct(perm.long.rate8h)}
                      </span>

                      {/* Short funding rate (8h) */}
                      <span
                        className={cn(
                          "text-right font-mono tabular-nums text-[0.625rem]",
                          fundingColor(perm.short.rate8h)
                        )}
                      >
                        {formatFundingPct(perm.short.rate8h)}
                      </span>

                      {/* Funding spread */}
                      <span
                        className={cn(
                          "text-right font-mono tabular-nums text-[0.625rem] font-medium",
                          fundingColor(perm.fundingSpreadPct)
                        )}
                      >
                        {formatSpreadPct(perm.fundingSpreadPct)}
                      </span>

                      {/* Price spread */}
                      <span
                        className={cn(
                          "text-right font-mono tabular-nums text-[0.625rem]",
                          fundingColor(perm.spreadPct)
                        )}
                      >
                        {formatSpreadPct(perm.spreadPct)}
                      </span>

                      {/* Long price */}
                      <span className="text-right font-mono tabular-nums text-[0.625rem] text-muted-foreground">
                        {formatPrice(perm.long.price)}
                      </span>
                    </div>
                  </CommandItem>
                ))}
              </div>
            </CommandGroup>
          </>
        )}
      </CommandList>
    </CommandDialog>
  );
}
