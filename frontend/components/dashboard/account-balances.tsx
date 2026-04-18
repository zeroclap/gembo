"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, Minus } from "lucide-react";

import { fetchBalances, type Balance } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";

const USD_LIKE_CURRENCIES = new Set([
  "USDT",
  "USDC",
  "USD",
  "BUSD",
  "USDE",
  "FDUSD",
]);

export interface AccountBalance {
  account: string;
  exchange: string;
  totalUsd: string;
  available: string;
  margin: string;
  unrealizedPnl: string;
  status: "connected" | "error" | "degraded";
}

function parseDecimal(value: string): number {
  const n = Number.parseFloat(value);
  return Number.isFinite(n) ? n : 0;
}

function formatMoney(value: number): string {
  return value.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

function statusFromUpdatedAt(updatedAt: string): AccountBalance["status"] {
  const ts = Date.parse(updatedAt);
  if (Number.isNaN(ts)) {
    return "degraded";
  }

  const ageMs = Date.now() - ts;
  if (ageMs <= 45_000) {
    return "connected";
  }
  if (ageMs <= 5 * 60_000) {
    return "degraded";
  }
  return "error";
}

function mapBalancesToRows(balances: Balance[]): AccountBalance[] {
  const grouped = balances.reduce<Record<string, Balance[]>>((acc, b) => {
    if (!acc[b.exchange_id]) {
      acc[b.exchange_id] = [];
    }
    acc[b.exchange_id].push(b);
    return acc;
  }, {});

  return Object.entries(grouped)
    .map(([exchange, items]) => {
      const usdLike = items.filter((b) => USD_LIKE_CURRENCIES.has(b.currency));
      const selected = usdLike.length > 0 ? usdLike : items;

      const total = selected.reduce((sum, b) => sum + parseDecimal(b.total), 0);
      const available = selected.reduce(
        (sum, b) => sum + parseDecimal(b.free),
        0
      );
      const margin = selected.reduce((sum, b) => sum + parseDecimal(b.used), 0);
      const latestUpdatedAt = selected.reduce((latest, b) => {
        return b.updated_at > latest ? b.updated_at : latest;
      }, "");

      return {
        account: usdLike.length > 0 ? "Collateral" : "Mixed assets",
        exchange,
        totalUsd: formatMoney(total),
        available: formatMoney(available),
        margin: formatMoney(margin),
        unrealizedPnl: "0.00",
        status: statusFromUpdatedAt(latestUpdatedAt),
      };
    })
    .sort((a, b) => a.exchange.localeCompare(b.exchange));
}

interface BalanceRowProps {
  balance: AccountBalance;
}

function BalanceRow({ balance }: BalanceRowProps) {
  const pnl = parseFloat(balance.unrealizedPnl.replace(/[^0-9+\-.]/g, ""));
  const isPositive = pnl > 0;
  const isNegative = pnl < 0;

  const statusDot = {
    connected: "bg-emerald-500",
    error: "bg-red-500",
    degraded: "bg-amber-500",
  }[balance.status];

  return (
    <div className="flex items-center gap-3 px-4 py-2.5 hover:bg-muted/30 transition-colors">
      <div className="flex items-center gap-2 w-36 shrink-0">
        <span className={cn("h-1.5 w-1.5 rounded-full shrink-0", statusDot)} />
        <div className="min-w-0">
          <div className="text-xs font-semibold capitalize truncate">
            {balance.exchange}
          </div>
          <div className="text-[0.6rem] text-muted-foreground truncate">
            {balance.account}
          </div>
        </div>
      </div>

      <div className="flex-1 min-w-0">
        <div className="text-xs font-mono tabular-nums font-medium">
          ${balance.totalUsd}
        </div>
        <div className="text-[0.6rem] text-muted-foreground">total</div>
      </div>

      <div className="flex-1 min-w-0 hidden sm:block">
        <div className="text-xs font-mono tabular-nums text-muted-foreground">
          ${balance.available}
        </div>
        <div className="text-[0.6rem] text-muted-foreground">available</div>
      </div>

      <div className="flex-1 min-w-0 hidden md:block">
        <div className="text-xs font-mono tabular-nums text-muted-foreground">
          ${balance.margin}
        </div>
        <div className="text-[0.6rem] text-muted-foreground">margin</div>
      </div>

      <div className="shrink-0 flex items-center gap-1">
        {isPositive && (
          <ArrowUp className="h-3 w-3 text-emerald-500" strokeWidth={2.5} />
        )}
        {isNegative && (
          <ArrowDown className="h-3 w-3 text-red-500" strokeWidth={2.5} />
        )}
        {!isPositive && !isNegative && (
          <Minus className="h-3 w-3 text-muted-foreground" strokeWidth={2} />
        )}
        <span
          className={cn(
            "text-xs font-mono tabular-nums font-medium",
            isPositive && "text-emerald-600 dark:text-emerald-400",
            isNegative && "text-red-600 dark:text-red-400",
            !isPositive && !isNegative && "text-muted-foreground"
          )}
        >
          ${Math.abs(pnl).toFixed(2)}
        </span>
      </div>
    </div>
  );
}

interface AccountBalancesProps {
  isLoading?: boolean;
}

export function AccountBalances({ isLoading }: AccountBalancesProps) {
  const balancesQuery = useQuery({
    queryKey: ["balances"],
    queryFn: fetchBalances,
  });

  const balances = mapBalancesToRows(balancesQuery.data ?? []);

  if (isLoading || (balancesQuery.isLoading && !balancesQuery.data)) {
    return (
      <div className="rounded-lg border bg-card/50 overflow-hidden">
        <div className="px-4 py-3 border-b">
          <Skeleton className="h-3 w-32" />
        </div>
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-2.5">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-3 w-20" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-12" />
          </div>
        ))}
      </div>
    );
  }

  const totalUsd = balances.reduce(
    (sum, b) => sum + parseFloat(b.totalUsd.replace(/,/g, "")),
    0
  );

  return (
    <div className="rounded-lg border bg-card/50 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2.5 border-b">
        <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
          Cross-Exchange Balances
        </span>
        <div className="flex items-center gap-2">
          <span className="text-xs font-mono tabular-nums font-semibold">
            $
            {totalUsd.toLocaleString(undefined, {
              minimumFractionDigits: 2,
              maximumFractionDigits: 2,
            })}
          </span>
          <span className="text-[0.6rem] text-muted-foreground">total</span>
        </div>
      </div>

      <div className="flex items-center gap-3 px-4 py-1.5 border-b bg-muted/20">
        <div className="w-36 shrink-0 text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Account
        </div>
        <div className="flex-1 text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Total
        </div>
        <div className="flex-1 hidden sm:block text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Available
        </div>
        <div className="flex-1 hidden md:block text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Margin
        </div>
        <div className="shrink-0 text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
          Unr. PnL
        </div>
      </div>

      <div className="divide-y divide-border/40">
        {balances.length === 0 && (
          <div className="px-4 py-4 text-xs text-muted-foreground">
            {balancesQuery.isError
              ? "Failed to load balances from backend"
              : "No balances received yet"}
          </div>
        )}
        {balances.map((b, i) => (
          <BalanceRow key={`${b.exchange}-${i}`} balance={b} />
        ))}
      </div>
    </div>
  );
}
