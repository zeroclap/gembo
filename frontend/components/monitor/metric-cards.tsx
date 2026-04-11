"use client";

import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatPrice,
  formatRate,
  formatVolume,
  formatPct,
  formatAPR,
  formatCountdown,
} from "@/lib/format";
import type { MonitorTickers, MonitorFundingRates, MonitorArbitrage } from "@/hooks/use-monitor-data";

// ---------------------------------------------------------------------------
// MetricCard — small KPI card
// ---------------------------------------------------------------------------

function MetricCard({
  label,
  children,
  sublabel,
  className,
}: {
  label: string;
  children: React.ReactNode;
  sublabel?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-lg border bg-card/50 px-3 py-2.5 space-y-1",
        className
      )}
    >
      <div className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className="text-sm font-mono tabular-nums font-medium leading-none">
        {children}
      </div>
      {sublabel && (
        <div className="text-[0.625rem] text-muted-foreground leading-tight">
          {sublabel}
        </div>
      )}
    </div>
  );
}

function MetricCardSkeleton() {
  return (
    <div className="rounded-lg border bg-card/50 px-3 py-2.5 space-y-1.5">
      <Skeleton className="h-2.5 w-12" />
      <Skeleton className="h-4 w-20" />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Countdown that ticks every second
// ---------------------------------------------------------------------------

function CountdownValue({ target }: { target: string }) {
  const [display, setDisplay] = useState(() => formatCountdown(target));

  useEffect(() => {
    const id = setInterval(() => {
      setDisplay(formatCountdown(target));
    }, 1000);
    return () => clearInterval(id);
  }, [target]);

  return <>{display}</>;
}

// ---------------------------------------------------------------------------
// PriceValue — colored based on positive/negative
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
// MetricCards
// ---------------------------------------------------------------------------

interface MetricCardsProps {
  tickers: MonitorTickers;
  fundingRates: MonitorFundingRates;
  arbitrage: MonitorArbitrage;
  longExchange: string;
  shortExchange: string;
}

export function MetricCards({
  tickers,
  fundingRates,
  arbitrage,
  longExchange,
  shortExchange,
}: MetricCardsProps) {
  const isLoading = tickers.isLoading || fundingRates.isLoading;

  if (isLoading) {
    return (
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
        {Array.from({ length: 10 }).map((_, i) => (
          <MetricCardSkeleton key={i} />
        ))}
      </div>
    );
  }

  const longTicker = tickers.long;
  const shortTicker = tickers.short;
  const longFunding = fundingRates.long;
  const shortFunding = fundingRates.short;
  const fundingArb = arbitrage.funding;
  const spreadArb = arbitrage.spread;

  // Compute spread from tickers
  const longAsk = parseFloat(longTicker?.ask ?? "0");
  const shortBid = parseFloat(shortTicker?.bid ?? "0");
  const midPrice =
    longAsk && shortBid ? (longAsk + shortBid) / 2 : 0;
  const spreadPct =
    midPrice > 0 ? ((shortBid - longAsk) / midPrice) * 100 : 0;

  // Net funding rate (normalized to 8h)
  const longRate = parseFloat(longFunding?.rate ?? "0");
  const shortRate = parseFloat(shortFunding?.rate ?? "0");
  const longInterval = longFunding?.interval_hours ?? 8;
  const shortInterval = shortFunding?.interval_hours ?? 8;
  const longRate8h = longRate * (8 / longInterval);
  const shortRate8h = shortRate * (8 / shortInterval);
  const netRate8h = shortRate8h - longRate8h;

  // Next funding times
  const longNextFunding = longFunding?.next_funding_time;
  const shortNextFunding = shortFunding?.next_funding_time;
  const nearestFunding =
    longNextFunding && shortNextFunding
      ? new Date(longNextFunding) < new Date(shortNextFunding)
        ? longNextFunding
        : shortNextFunding
      : longNextFunding ?? shortNextFunding;

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
      {/* Long price */}
      <MetricCard
        label={`Long (${longExchange})`}
        sublabel={
          longTicker
            ? `Bid ${formatPrice(longTicker.bid)} / Ask ${formatPrice(longTicker.ask)}`
            : undefined
        }
      >
        {longTicker ? formatPrice(longTicker.price) : "-"}
      </MetricCard>

      {/* Short price */}
      <MetricCard
        label={`Short (${shortExchange})`}
        sublabel={
          shortTicker
            ? `Bid ${formatPrice(shortTicker.bid)} / Ask ${formatPrice(shortTicker.ask)}`
            : undefined
        }
      >
        {shortTicker ? formatPrice(shortTicker.price) : "-"}
      </MetricCard>

      {/* Spread */}
      <MetricCard
        label="Spread"
        sublabel={
          spreadArb
            ? `Net ${formatPct(spreadArb.net_spread_pct)}`
            : midPrice > 0
              ? `Mid ${formatPrice(String(midPrice))}`
              : undefined
        }
      >
        <ColoredValue
          value={spreadPct}
          formatted={`${spreadPct >= 0 ? "+" : ""}${spreadPct.toFixed(4)}%`}
        />
      </MetricCard>

      {/* Long funding rate */}
      <MetricCard
        label={`Funding (${longExchange})`}
        sublabel={`Every ${longInterval}h`}
      >
        <ColoredValue
          value={-longRate}
          formatted={formatRate(longFunding?.rate ?? "0")}
        />
      </MetricCard>

      {/* Short funding rate */}
      <MetricCard
        label={`Funding (${shortExchange})`}
        sublabel={`Every ${shortInterval}h`}
      >
        <ColoredValue
          value={shortRate}
          formatted={formatRate(shortFunding?.rate ?? "0")}
        />
      </MetricCard>

      {/* Net funding rate */}
      <MetricCard
        label="Net Rate (8h)"
        sublabel={
          fundingArb
            ? `APR ${formatAPR(fundingArb.apr)}`
            : undefined
        }
      >
        <ColoredValue
          value={netRate8h}
          formatted={`${netRate8h >= 0 ? "+" : ""}${(netRate8h * 100).toFixed(4)}%`}
        />
      </MetricCard>

      {/* APR after fees */}
      <MetricCard
        label="Net APR"
        sublabel={
          fundingArb
            ? `Fees ${formatPct(fundingArb.fees.round_trip_fees_pct)}`
            : undefined
        }
      >
        {fundingArb ? (
          <ColoredValue
            value={parseFloat(fundingArb.fees.net_apr_after_fees)}
            formatted={formatAPR(fundingArb.fees.net_apr_after_fees)}
          />
        ) : (
          "-"
        )}
      </MetricCard>

      {/* Next funding */}
      <MetricCard
        label="Next Funding"
        sublabel={
          fundingArb
            ? fundingArb.next_payment.exchange_id === "both"
              ? "both legs"
              : `${fundingArb.next_payment.exchange_id} (${fundingArb.next_payment.side})`
            : undefined
        }
      >
        {nearestFunding ? <CountdownValue target={nearestFunding} /> : "-"}
      </MetricCard>

      {/* Volume long */}
      <MetricCard label={`Volume (${longExchange})`}>
        {longTicker ? formatVolume(longTicker.volume_24h) : "-"}
      </MetricCard>

      {/* Volume short */}
      <MetricCard label={`Volume (${shortExchange})`}>
        {shortTicker ? formatVolume(shortTicker.volume_24h) : "-"}
      </MetricCard>
    </div>
  );
}
