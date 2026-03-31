const API_BASE = "/api/v1";

export interface ApiResponse<T> {
  data: T;
  error: { code: string; message: string } | null;
  ts: string;
}

export interface Ticker {
  exchange_id: string;
  symbol: string;
  price: string;
  bid: string;
  ask: string;
  high_24h: string;
  low_24h: string;
  volume_24h: string;
  change_24h: string;
  updated_at: string;
}

export interface TickerRow {
  symbol: string;
  tickers: Ticker[];
}

export interface FlushResult {
  deleted_keys: number;
}

async function fetchApi<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, init);
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  const json: ApiResponse<T> = await res.json();
  if (json.error) {
    throw new Error(json.error.message);
  }
  return json.data;
}

export function fetchAllTickers(): Promise<TickerRow[]> {
  return fetchApi<TickerRow[]>("/tickers");
}

export function fetchExchanges(): Promise<string[]> {
  return fetchApi<string[]>("/exchanges");
}

export function flushAllData(): Promise<FlushResult> {
  return fetchApi<FlushResult>("/flush", { method: "DELETE" });
}

// --- Funding Arbitrage ---

export interface ArbitrageLeg {
  exchange_id: string;
  funding_rate: string;
  interval_hours: number;
  rate_8h: string;
  next_funding_at: string;
  next_funding_rate: string;
  price: string;
  bid: string;
  ask: string;
  mark_price: string;
  index_price: string;
  volume_24h: string;
  maker_fee_pct: string;
  taker_fee_pct: string;
}

export interface ArbitrageFees {
  open_fees_pct: string;
  close_fees_pct: string;
  round_trip_fees_pct: string;
  net_apr_after_fees: string;
}

export interface NextPaymentInfo {
  exchange_id: string;
  side: string;
  rate: string;
  payment_pct: string;
  at: string;
  time_until_secs: number;
}

export interface LoopPayment {
  exchange_id: string;
  side: string;
  rate: string;
  payment_pct: string;
  at_hour: number;
  cumulative: string;
}

export interface FullLoopInfo {
  duration_hours: number;
  payments: LoopPayment[];
  net_pct_per_loop: string;
}

export interface FundingArbitrage {
  symbol: string;
  long: ArbitrageLeg;
  short: ArbitrageLeg;
  net_rate_8h: string;
  apr: string;
  open_spread: string;
  fees: ArbitrageFees;
  next_payment: NextPaymentInfo;
  full_loop: FullLoopInfo;
  updated_at: string;
}

export function fetchFundingArbitrage(): Promise<FundingArbitrage[]> {
  return fetchApi<FundingArbitrage[]>("/arbitrage/funding");
}
