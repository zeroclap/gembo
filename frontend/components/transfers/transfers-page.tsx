"use client";

import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowRight,
  ArrowLeftRight,
  AlertTriangle,
  CheckCircle2,
  Clock,
  XCircle,
  Loader2,
} from "lucide-react";

import { fetchBalances, type Balance } from "@/lib/api";
import { EXCHANGE_META, EXCHANGES, type ExchangeId } from "@/lib/account-store";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

// ---------------------------------------------------------------------------
// Static mock data for chains and fees
// ---------------------------------------------------------------------------

const EXCHANGE_CHAINS: Record<ExchangeId, Record<string, string[]>> = {
  binance: {
    USDT: ["TRC20", "ERC20", "BEP20", "SOL"],
    USDC: ["ERC20", "BEP20", "SOL", "AVAXC"],
    BTC: ["BTC", "BEP20"],
    ETH: ["ERC20", "BEP20"],
    BNB: ["BEP20"],
  },
  bybit: {
    USDT: ["TRC20", "ERC20", "BEP20"],
    USDC: ["ERC20", "BEP20", "SOL"],
    BTC: ["BTC", "BEP20"],
    ETH: ["ERC20"],
    BNB: ["BEP20"],
  },
  gate: {
    USDT: ["TRC20", "ERC20", "BEP20", "SOL"],
    USDC: ["ERC20", "SOL"],
    BTC: ["BTC"],
    ETH: ["ERC20"],
  },
  okx: {
    USDT: ["TRC20", "ERC20", "BEP20", "SOL", "AVAXC", "MATIC"],
    USDC: ["ERC20", "BEP20", "SOL", "AVAXC"],
    BTC: ["BTC", "BEP20"],
    ETH: ["ERC20", "BEP20"],
    BNB: ["BEP20"],
  },
};

const WITHDRAWAL_FEES: Record<
  ExchangeId,
  Record<string, Record<string, string>>
> = {
  binance: {
    USDT: { TRC20: "1.00", ERC20: "4.90", BEP20: "0.29", SOL: "0.10" },
    USDC: { ERC20: "5.00", BEP20: "0.35", SOL: "0.10", AVAXC: "0.50" },
    BTC: { BTC: "0.00050", BEP20: "0.00010" },
    ETH: { ERC20: "0.00200", BEP20: "0.00020" },
    BNB: { BEP20: "0.00050" },
  },
  bybit: {
    USDT: { TRC20: "1.00", ERC20: "10.00", BEP20: "1.00" },
    USDC: { ERC20: "8.00", BEP20: "1.00", SOL: "0.10" },
    BTC: { BTC: "0.00050", BEP20: "0.00010" },
    ETH: { ERC20: "0.00300" },
    BNB: { BEP20: "0.00060" },
  },
  gate: {
    USDT: { TRC20: "1.00", ERC20: "6.00", BEP20: "0.50", SOL: "0.15" },
    USDC: { ERC20: "7.00", SOL: "0.15" },
    BTC: { BTC: "0.00060" },
    ETH: { ERC20: "0.00250" },
  },
  okx: {
    USDT: {
      TRC20: "0.80",
      ERC20: "3.50",
      BEP20: "0.25",
      SOL: "0.08",
      AVAXC: "0.40",
      MATIC: "0.20",
    },
    USDC: { ERC20: "4.00", BEP20: "0.30", SOL: "0.08", AVAXC: "0.40" },
    BTC: { BTC: "0.00040", BEP20: "0.00010" },
    ETH: { ERC20: "0.00180", BEP20: "0.00018" },
    BNB: { BEP20: "0.00050" },
  },
};

// Estimated confirmation times (minutes) per chain
const CHAIN_CONFIRM_MINUTES: Record<string, number> = {
  TRC20: 2,
  ERC20: 5,
  BEP20: 1,
  SOL: 1,
  BTC: 30,
  AVAXC: 2,
  MATIC: 2,
};

// ---------------------------------------------------------------------------
// Mock transfer history
// ---------------------------------------------------------------------------

type TransferStatus = "completed" | "pending" | "failed";

interface TransferRecord {
  id: string;
  from_exchange: ExchangeId;
  to_exchange: ExchangeId;
  asset: string;
  chain: string;
  amount: string;
  fee: string;
  status: TransferStatus;
  created_at: string;
}

const MOCK_HISTORY: TransferRecord[] = [
  {
    id: "t1",
    from_exchange: "bybit",
    to_exchange: "binance",
    asset: "USDT",
    chain: "TRC20",
    amount: "5000.00",
    fee: "1.00",
    status: "completed",
    created_at: new Date(Date.now() - 3600_000).toISOString(),
  },
  {
    id: "t2",
    from_exchange: "okx",
    to_exchange: "gate",
    asset: "BTC",
    chain: "BTC",
    amount: "0.15000",
    fee: "0.00040",
    status: "completed",
    created_at: new Date(Date.now() - 7200_000 * 3).toISOString(),
  },
  {
    id: "t3",
    from_exchange: "binance",
    to_exchange: "okx",
    asset: "ETH",
    chain: "ERC20",
    amount: "2.00000",
    fee: "0.00200",
    status: "pending",
    created_at: new Date(Date.now() - 900_000).toISOString(),
  },
  {
    id: "t4",
    from_exchange: "gate",
    to_exchange: "bybit",
    asset: "USDC",
    chain: "SOL",
    amount: "3000.00",
    fee: "0.15",
    status: "failed",
    created_at: new Date(Date.now() - 86400_000).toISOString(),
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function parseDecimal(v: string): number {
  const n = Number.parseFloat(v);
  return Number.isFinite(n) ? n : 0;
}

function fmtNumber(v: number, decimals = 2): string {
  return v.toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

function timeAgo(iso: string): string {
  const delta = Math.floor((Date.now() - Date.parse(iso)) / 1000);
  if (delta < 60) return `${delta}s ago`;
  if (delta < 3600) return `${Math.floor(delta / 60)}m ago`;
  if (delta < 86400) return `${Math.floor(delta / 3600)}h ago`;
  return `${Math.floor(delta / 86400)}d ago`;
}

// Get all assets available on a given exchange from live balance data
function assetsForExchange(balances: Balance[], exchange: string): string[] {
  const fromApi = balances
    .filter((b) => b.exchange_id === exchange && parseDecimal(b.total) > 0)
    .map((b) => b.currency);
  // Merge with known chain assets
  const known = Object.keys(EXCHANGE_CHAINS[exchange as ExchangeId] ?? {});
  return [...new Set([...fromApi, ...known])].sort();
}

function getBalance(
  balances: Balance[],
  exchange: string,
  asset: string
): { free: number; total: number } | null {
  const b = balances.find(
    (b) => b.exchange_id === exchange && b.currency === asset
  );
  if (!b) return null;
  return { free: parseDecimal(b.free), total: parseDecimal(b.total) };
}

function getFee(
  exchange: ExchangeId,
  asset: string,
  chain: string
): string | null {
  return WITHDRAWAL_FEES[exchange]?.[asset]?.[chain] ?? null;
}

function getChains(exchange: ExchangeId, asset: string): string[] {
  return EXCHANGE_CHAINS[exchange]?.[asset] ?? [];
}

// Common chains between two exchanges for an asset
function commonChains(
  fromEx: ExchangeId,
  toEx: ExchangeId,
  asset: string
): string[] {
  const from = new Set(getChains(fromEx, asset));
  const to = new Set(getChains(toEx, asset));
  return [...from].filter((c) => to.has(c));
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function ExchangeBadge({
  exchange,
  className,
}: {
  exchange: ExchangeId;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 text-[0.65rem] font-semibold uppercase tracking-wide",
        className
      )}
    >
      {EXCHANGE_META[exchange].label}
    </span>
  );
}

function StatusIcon({ status }: { status: TransferStatus }) {
  if (status === "completed")
    return <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />;
  if (status === "pending")
    return <Clock className="h-3.5 w-3.5 text-amber-500" />;
  return <XCircle className="h-3.5 w-3.5 text-red-500" />;
}

// ---------------------------------------------------------------------------
// Balance overview strip
// ---------------------------------------------------------------------------

interface BalanceStripProps {
  balances: Balance[];
  selectedAsset: string;
}

function BalanceStrip({ balances, selectedAsset }: BalanceStripProps) {
  const rows = EXCHANGES.map((ex) => {
    const b = getBalance(balances, ex, selectedAsset);
    return { exchange: ex, free: b?.free ?? null, total: b?.total ?? null };
  });

  return (
    <div className="grid grid-cols-4 gap-2">
      {rows.map(({ exchange, free, total }) => (
        <div
          key={exchange}
          className="rounded-md border bg-card/50 px-3 py-2 flex flex-col gap-0.5"
        >
          <span className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
            {EXCHANGE_META[exchange].label}
          </span>
          {free !== null ? (
            <>
              <span className="text-xs font-mono tabular-nums font-semibold">
                {fmtNumber(free, selectedAsset === "BTC" ? 5 : 2)}
              </span>
              <span className="text-[0.6rem] text-muted-foreground">
                {fmtNumber(total ?? 0, selectedAsset === "BTC" ? 5 : 2)} total
              </span>
            </>
          ) : (
            <span className="text-xs text-muted-foreground/50">—</span>
          )}
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Transfer history row
// ---------------------------------------------------------------------------

function HistoryRow({ record }: { record: TransferRecord }) {
  const decimals =
    record.asset === "BTC" || record.asset === "ETH" ? 5 : 2;

  return (
    <div className="flex items-center gap-3 px-3 py-2.5 hover:bg-muted/20 transition-colors text-xs">
      <StatusIcon status={record.status} />

      <div className="flex items-center gap-1 shrink-0">
        <ExchangeBadge exchange={record.from_exchange} />
        <ArrowRight className="h-3 w-3 text-muted-foreground" />
        <ExchangeBadge exchange={record.to_exchange} />
      </div>

      <div className="flex-1 min-w-0 flex items-center gap-1.5">
        <span className="font-mono tabular-nums font-semibold">
          {fmtNumber(parseDecimal(record.amount), decimals)}
        </span>
        <span className="text-muted-foreground">{record.asset}</span>
        <Badge
          variant="outline"
          className="text-[0.55rem] px-1 py-0 h-4 font-mono"
        >
          {record.chain}
        </Badge>
      </div>

      <div className="shrink-0 text-right">
        <div className="font-mono tabular-nums text-muted-foreground">
          fee {fmtNumber(parseDecimal(record.fee), decimals)} {record.asset}
        </div>
        <div className="text-[0.6rem] text-muted-foreground/60">
          {timeAgo(record.created_at)}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export function TransfersPage() {
  const balancesQuery = useQuery({
    queryKey: ["balances"],
    queryFn: fetchBalances,
    refetchInterval: 15_000,
  });

  const balances = useMemo(
    () => balancesQuery.data ?? [],
    [balancesQuery.data]
  );

  // Form state
  const [fromExchange, setFromExchange] = useState<ExchangeId>("bybit");
  const [toExchange, setToExchange] = useState<ExchangeId>("binance");
  const [selectedAsset, setSelectedAsset] = useState("USDT");
  const [selectedChain, setSelectedChain] = useState("TRC20");
  const [amount, setAmount] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  // Derived
  const availableAssets = useMemo(
    () => assetsForExchange(balances, fromExchange),
    [balances, fromExchange]
  );

  const availableChains = useMemo(
    () => commonChains(fromExchange, toExchange, selectedAsset),
    [fromExchange, toExchange, selectedAsset]
  );

  const fromBalance = getBalance(balances, fromExchange, selectedAsset);
  const toBalance = getBalance(balances, toExchange, selectedAsset);

  const feeStr = selectedChain
    ? getFee(fromExchange, selectedAsset, selectedChain)
    : null;
  const fee = feeStr ? parseDecimal(feeStr) : null;
  const amountNum = parseDecimal(amount);
  const receiveAmount =
    amountNum > 0 && fee !== null ? Math.max(0, amountNum - fee) : null;

  const confirmMinutes = CHAIN_CONFIRM_MINUTES[selectedChain] ?? 5;

  // Validation
  const maxAmount = fromBalance?.free ?? 0;
  const hasInsufficientBalance =
    amountNum > 0 && amountNum > maxAmount;
  const belowFee = fee !== null && amountNum > 0 && amountNum <= fee;
  const canSubmit =
    amountNum > 0 &&
    !hasInsufficientBalance &&
    !belowFee &&
    fromExchange !== toExchange &&
    selectedChain !== "" &&
    !submitting;

  // When from/to changes ensure chain is still valid
  function handleFromExchangeChange(ex: ExchangeId) {
    setFromExchange(ex);
    setSubmitted(false);
    // reset chain if no longer available
    const chains = commonChains(ex, toExchange, selectedAsset);
    if (!chains.includes(selectedChain)) setSelectedChain(chains[0] ?? "");
  }

  function handleToExchangeChange(ex: ExchangeId) {
    setToExchange(ex);
    setSubmitted(false);
    const chains = commonChains(fromExchange, ex, selectedAsset);
    if (!chains.includes(selectedChain)) setSelectedChain(chains[0] ?? "");
  }

  function handleAssetChange(asset: string) {
    setSelectedAsset(asset);
    setSubmitted(false);
    const chains = commonChains(fromExchange, toExchange, asset);
    setSelectedChain(chains[0] ?? "");
    setAmount("");
  }

  function handleSwap() {
    const prev = fromExchange;
    setFromExchange(toExchange);
    setToExchange(prev);
    setSubmitted(false);
    const chains = commonChains(toExchange, prev, selectedAsset);
    if (!chains.includes(selectedChain)) setSelectedChain(chains[0] ?? "");
  }

  function handleMaxAmount() {
    if (fromBalance) {
      const decimals =
        selectedAsset === "BTC" || selectedAsset === "ETH" ? 5 : 2;
      setAmount(fromBalance.free.toFixed(decimals));
    }
  }

  function handleSubmit() {
    if (!canSubmit) return;
    setSubmitting(true);
    // Simulate async call
    setTimeout(() => {
      setSubmitting(false);
      setSubmitted(true);
      setAmount("");
    }, 1500);
  }

  const decimals = selectedAsset === "BTC" || selectedAsset === "ETH" ? 5 : 2;

  return (
    <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8 py-5 space-y-5">
      {/* Page title */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-sm font-semibold">Transfers</h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            Move assets between exchanges across supported networks
          </p>
        </div>
        {balancesQuery.isLoading && (
          <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
        )}
      </div>

      {/* Balance overview */}
      <BalanceStrip balances={balances} selectedAsset={selectedAsset} />

      {/* Main layout */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
        {/* Transfer form — 3/5 width */}
        <div className="lg:col-span-3 space-y-3">
          <div className="rounded-lg border bg-card/50 overflow-hidden">
            {/* Header */}
            <div className="px-4 py-2.5 border-b flex items-center justify-between">
              <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                New Transfer
              </span>
              <div className="flex items-center gap-2">
                {/* Asset selector */}
                <Select value={selectedAsset} onValueChange={handleAssetChange}>
                  <SelectTrigger className="h-6 text-xs px-2 w-24 gap-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {availableAssets.map((a) => (
                      <SelectItem key={a} value={a} className="text-xs">
                        {a}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="p-4 space-y-3">
              {/* From / To row */}
              <div className="flex items-start gap-2">
                {/* From */}
                <div className="flex-1 space-y-1.5">
                  <label className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
                    From
                  </label>
                  <Select
                    value={fromExchange}
                    onValueChange={(v) =>
                      handleFromExchangeChange(v as ExchangeId)
                    }
                  >
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {EXCHANGES.filter((ex) => ex !== toExchange).map(
                        (ex) => (
                          <SelectItem key={ex} value={ex} className="text-xs">
                            {EXCHANGE_META[ex].label}
                          </SelectItem>
                        )
                      )}
                    </SelectContent>
                  </Select>
                  {fromBalance !== null ? (
                    <div className="text-[0.6rem] text-muted-foreground px-0.5">
                      Available:{" "}
                      <span className="font-mono tabular-nums font-medium text-foreground">
                        {fmtNumber(fromBalance.free, decimals)}
                      </span>{" "}
                      {selectedAsset}
                    </div>
                  ) : (
                    <div className="text-[0.6rem] text-muted-foreground/50 px-0.5">
                      No balance data
                    </div>
                  )}
                </div>

                {/* Swap button */}
                <div className="pt-5">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 rounded-full border"
                    onClick={handleSwap}
                    aria-label="Swap direction"
                  >
                    <ArrowLeftRight className="h-3.5 w-3.5" />
                  </Button>
                </div>

                {/* To */}
                <div className="flex-1 space-y-1.5">
                  <label className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
                    To
                  </label>
                  <Select
                    value={toExchange}
                    onValueChange={(v) =>
                      handleToExchangeChange(v as ExchangeId)
                    }
                  >
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {EXCHANGES.filter((ex) => ex !== fromExchange).map(
                        (ex) => (
                          <SelectItem key={ex} value={ex} className="text-xs">
                            {EXCHANGE_META[ex].label}
                          </SelectItem>
                        )
                      )}
                    </SelectContent>
                  </Select>
                  {toBalance !== null ? (
                    <div className="text-[0.6rem] text-muted-foreground px-0.5">
                      Current:{" "}
                      <span className="font-mono tabular-nums font-medium text-foreground">
                        {fmtNumber(toBalance.total, decimals)}
                      </span>{" "}
                      {selectedAsset}
                    </div>
                  ) : (
                    <div className="text-[0.6rem] text-muted-foreground/50 px-0.5">
                      No balance data
                    </div>
                  )}
                </div>
              </div>

              {/* Network / chain */}
              <div className="space-y-1.5">
                <label className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
                  Network
                </label>
                {availableChains.length === 0 ? (
                  <div className="flex items-center gap-1.5 rounded-md border border-amber-500/30 bg-amber-500/5 px-3 py-2">
                    <AlertTriangle className="h-3 w-3 text-amber-500 shrink-0" />
                    <span className="text-xs text-amber-600 dark:text-amber-400">
                      No common networks between{" "}
                      {EXCHANGE_META[fromExchange].label} and{" "}
                      {EXCHANGE_META[toExchange].label} for {selectedAsset}
                    </span>
                  </div>
                ) : (
                  <div className="flex flex-wrap gap-1.5">
                    {availableChains.map((chain) => {
                      const chainFee = getFee(fromExchange, selectedAsset, chain);
                      return (
                        <button
                          key={chain}
                          onClick={() => setSelectedChain(chain)}
                          className={cn(
                            "flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors",
                            selectedChain === chain
                              ? "border-primary bg-primary/10 text-primary"
                              : "border-border bg-card hover:bg-muted/50 text-muted-foreground hover:text-foreground"
                          )}
                        >
                          <span>{chain}</span>
                          {chainFee && (
                            <span
                              className={cn(
                                "text-[0.6rem] font-mono",
                                selectedChain === chain
                                  ? "text-primary/70"
                                  : "text-muted-foreground/60"
                              )}
                            >
                              {chainFee} {selectedAsset}
                            </span>
                          )}
                          <span
                            className={cn(
                              "text-[0.6rem]",
                              selectedChain === chain
                                ? "text-primary/60"
                                : "text-muted-foreground/40"
                            )}
                          >
                            ~{CHAIN_CONFIRM_MINUTES[chain] ?? "?"}m
                          </span>
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* Amount */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <label className="text-[0.6rem] font-medium uppercase tracking-wider text-muted-foreground">
                    Amount
                  </label>
                  {fromBalance && (
                    <button
                      onClick={handleMaxAmount}
                      className="text-[0.6rem] text-primary hover:text-primary/80 font-medium uppercase tracking-wider"
                    >
                      Max
                    </button>
                  )}
                </div>
                <div className="relative">
                  <Input
                    type="number"
                    placeholder="0.00"
                    value={amount}
                    onChange={(e) => {
                      setAmount(e.target.value);
                      setSubmitted(false);
                    }}
                    className={cn(
                      "h-9 text-sm font-mono pr-14",
                      (hasInsufficientBalance || belowFee) &&
                        "border-red-500/50 focus-visible:ring-red-500/30"
                    )}
                  />
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground font-medium">
                    {selectedAsset}
                  </span>
                </div>
                {hasInsufficientBalance && (
                  <p className="text-[0.6rem] text-red-500 flex items-center gap-1">
                    <AlertTriangle className="h-2.5 w-2.5" />
                    Insufficient balance. Available:{" "}
                    {fmtNumber(maxAmount, decimals)} {selectedAsset}
                  </p>
                )}
                {belowFee && !hasInsufficientBalance && (
                  <p className="text-[0.6rem] text-red-500 flex items-center gap-1">
                    <AlertTriangle className="h-2.5 w-2.5" />
                    Amount must exceed network fee ({feeStr} {selectedAsset})
                  </p>
                )}
              </div>

              {/* Fee breakdown */}
              {amountNum > 0 && selectedChain && (
                <div className="rounded-md border bg-muted/20 px-3 py-2.5 space-y-1.5 text-xs">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">
                      Withdrawal fee ({selectedChain})
                    </span>
                    <span className="font-mono tabular-nums">
                      {feeStr ?? "—"} {selectedAsset}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">
                      Est. confirmation
                    </span>
                    <span className="font-mono tabular-nums">
                      ~{confirmMinutes} min
                    </span>
                  </div>
                  <div className="border-t pt-1.5 flex justify-between font-medium">
                    <span>You receive</span>
                    <span
                      className={cn(
                        "font-mono tabular-nums",
                        receiveAmount !== null && receiveAmount > 0
                          ? "text-emerald-600 dark:text-emerald-400"
                          : "text-red-500"
                      )}
                    >
                      {receiveAmount !== null
                        ? `${fmtNumber(receiveAmount, decimals)} ${selectedAsset}`
                        : "—"}
                    </span>
                  </div>
                </div>
              )}

              {/* Submit */}
              {submitted ? (
                <div className="flex items-center gap-2 rounded-md border border-emerald-500/30 bg-emerald-500/5 px-3 py-2.5">
                  <CheckCircle2 className="h-4 w-4 text-emerald-500 shrink-0" />
                  <div>
                    <p className="text-xs font-medium text-emerald-600 dark:text-emerald-400">
                      Transfer submitted
                    </p>
                    <p className="text-[0.6rem] text-muted-foreground">
                      Funds should arrive in approximately {confirmMinutes}{" "}
                      minutes
                    </p>
                  </div>
                </div>
              ) : (
                <Button
                  className="w-full h-9 text-xs font-medium"
                  disabled={!canSubmit}
                  onClick={handleSubmit}
                >
                  {submitting ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin mr-1.5" />
                      Submitting...
                    </>
                  ) : (
                    <>
                      <ArrowRight className="h-3.5 w-3.5 mr-1.5" />
                      Transfer {selectedAsset}
                      {fromExchange && toExchange && (
                        <span className="ml-1 text-primary-foreground/70">
                          · {EXCHANGE_META[fromExchange].label} →{" "}
                          {EXCHANGE_META[toExchange].label}
                        </span>
                      )}
                    </>
                  )}
                </Button>
              )}
            </div>
          </div>

          {/* Cross-chain warning */}
          {selectedChain && fromExchange !== toExchange && (
            <div className="flex items-start gap-2 rounded-md border border-border/50 bg-card/30 px-3 py-2.5">
              <AlertTriangle className="h-3.5 w-3.5 text-muted-foreground shrink-0 mt-0.5" />
              <p className="text-[0.65rem] text-muted-foreground leading-relaxed">
                Ensure the destination address on{" "}
                <span className="font-medium text-foreground">
                  {EXCHANGE_META[toExchange].label}
                </span>{" "}
                supports <span className="font-medium text-foreground">{selectedChain}</span>{" "}
                deposits for{" "}
                <span className="font-medium text-foreground">{selectedAsset}</span>.
                Sending to an incompatible network may result in permanent loss
                of funds.
              </p>
            </div>
          )}
        </div>

        {/* Transfer history — 2/5 width */}
        <div className="lg:col-span-2">
          <div className="rounded-lg border bg-card/50 overflow-hidden">
            <div className="px-3 py-2.5 border-b flex items-center justify-between">
              <span className="text-[0.625rem] font-medium uppercase tracking-wider text-muted-foreground">
                History
              </span>
              <div className="flex items-center gap-2 text-[0.6rem] text-muted-foreground">
                <span className="flex items-center gap-1">
                  <CheckCircle2 className="h-3 w-3 text-emerald-500" />
                  completed
                </span>
                <span className="flex items-center gap-1">
                  <Clock className="h-3 w-3 text-amber-500" />
                  pending
                </span>
                <span className="flex items-center gap-1">
                  <XCircle className="h-3 w-3 text-red-500" />
                  failed
                </span>
              </div>
            </div>
            <div className="divide-y divide-border/40">
              {MOCK_HISTORY.map((r) => (
                <HistoryRow key={r.id} record={r} />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
