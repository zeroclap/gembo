"use client";

import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatPrice,
  formatRate,
  formatVolume,
  formatCountdown,
} from "@/lib/format";
import { ArrowUp, ArrowDown } from "lucide-react";
import type { ArbitrageTickers, ArbitrageFundingRates } from "@/hooks/use-arbitrage-data";

// ---------------------------------------------------------------------------
// Countdown that ticks every second
// ---------------------------------------------------------------------------

function CountdownValue({ target }: { target: string }) {
  const [display, setDisplay] = useState(() => formatCountdown(target));
  useEffect(() => {
    const id = setInterval(() => setDisplay(formatCountdown(target)), 1000);
    return () => clearInterval(id);
  }, [target]);
  return <>{display}</>;
}

// ---------------------------------------------------------------------------
// ColoredValue
// ---------------------------------------------------------------------------

function ColoredValue({
  value,
  formatted,
  className,
}: {
  value: number;
  formatted: string;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "tabular-nums",
        value > 0 && "text-emerald-600 dark:text-emerald-400",
        value < 0 && "text-red-600 dark:text-red-400",
        className
      )}
    >
      {formatted}
    </span>
  );
}

// ---------------------------------------------------------------------------
// StatItem — a single key/value unit inside a status bar row
// ---------------------------------------------------------------------------

function StatItem({
  label,
  children,
  className,
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <span className={cn("flex items-baseline gap-1 shrink-0", className)}>
      <span className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground leading-none">
        {label}
      </span>
      <span className="text-[0.75rem] font-mono font-medium leading-none">
        {children}
      </span>
    </span>
  );
}

// ---------------------------------------------------------------------------
// Separator between stat items
// ---------------------------------------------------------------------------

function Sep() {
  return <span className="text-border/70 select-none shrink-0 text-xs">·</span>;
}

// ---------------------------------------------------------------------------
// ExchangeStatusRow — one horizontal line per exchange
// ---------------------------------------------------------------------------

export interface ExchangeStatusRowProps {
  exchangeId: string;
  side: "long" | "short";
  ticker: { price: string; bid: string; ask: string; volume_24h: string } | undefined;
  fundingRate: { rate: string; interval_hours: number; next_funding_time: string } | undefined;
}

export function ExchangeStatusRow({ exchangeId, side, ticker, fundingRate }: ExchangeStatusRowProps) {
  const isLong = side === "long";
  const rate = parseFloat(fundingRate?.rate ?? "0");
  const interval = fundingRate?.interval_hours ?? 8;

  return (
    <div className="flex items-center gap-2 px-3 py-2 min-w-0">
      {/* Side badge + exchange name */}
      <span className="flex items-center gap-1.5 shrink-0">
        <span
          className={cn(
            "inline-flex items-center justify-center h-4 w-4 rounded-sm shrink-0",
            isLong
              ? "bg-emerald-600 dark:bg-emerald-500"
              : "bg-red-600 dark:bg-red-500"
          )}
        >
          {isLong ? (
            <ArrowUp className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
          ) : (
            <ArrowDown className="h-2.5 w-2.5 text-white" strokeWidth={2.5} />
          )}
        </span>
        <span className="text-xs font-semibold capitalize w-12 shrink-0">
          {exchangeId || "—"}
        </span>
      </span>

      <span className="w-px h-3 bg-border/60 shrink-0" />

      {/* Metrics inline */}
      <div className="flex items-center gap-2 min-w-0 flex-wrap">
        <StatItem label="Price">
          {ticker ? (
            <span>{formatPrice(ticker.price)}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
        </StatItem>

        {ticker && (
          <>
            <Sep />
            <StatItem label="Bid/Ask">
              <span className="text-muted-foreground">
                {formatPrice(ticker.bid)}
                <span className="text-border/70 mx-0.5">/</span>
                {formatPrice(ticker.ask)}
              </span>
            </StatItem>
          </>
        )}

        <Sep />

        <StatItem label={`Funding/${interval}h`}>
          {fundingRate ? (
            <span className="flex items-baseline gap-1">
              <ColoredValue
                value={isLong ? -rate : rate}
                formatted={formatRate(fundingRate.rate)}
              />
              {fundingRate.next_funding_time && (
                <span className="text-[0.65rem] text-muted-foreground font-normal">
                  <CountdownValue target={fundingRate.next_funding_time} />
                </span>
              )}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
        </StatItem>

        {ticker && (
          <>
            <Sep />
            <StatItem label="Vol 24h">
              <span className="text-muted-foreground">{formatVolume(ticker.volume_24h)}</span>
            </StatItem>
          </>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// MetricCards — now a vertical stack of status-bar rows
// ---------------------------------------------------------------------------

interface MetricCardsProps {
 tickers: ArbitrageTickers;
 fundingRates: ArbitrageFundingRates;
  longExchange: string;
  shortExchange: string;
}

export function MetricCards({
  tickers,
  fundingRates,
  longExchange,
  shortExchange,
}: MetricCardsProps) {
  const isLoading = tickers.isLoading || fundingRates.isLoading;

  if (isLoading) {
    return (
      <div className="divide-y divide-border/40 border-b">
        {[0, 1].map((i) => (
          <div key={i} className="flex items-center gap-3 px-3 py-2">
            <Skeleton className="h-4 w-4 rounded-sm" />
            <Skeleton className="h-3 w-12" />
            <Skeleton className="h-3 w-20" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-14" />
          </div>
        ))}
        <div className="flex gap-2 px-3 py-2">
          <Skeleton className="flex-1 h-16 rounded-md" />
          <Skeleton className="flex-1 h-16 rounded-md" />
        </div>
      </div>
    );
  }

  return (
    <div className="divide-y divide-border/40 border-b">
      <div className="md:hidden">
        <ExchangeStatusRow
          exchangeId={longExchange}
          side="long"
          ticker={tickers.long}
          fundingRate={fundingRates.long}
        />
      </div>
      <div className="md:hidden">
        <ExchangeStatusRow
          exchangeId={shortExchange}
          side="short"
          ticker={tickers.short}
          fundingRate={fundingRates.short}
        />
      </div>
    </div>
  );
}
