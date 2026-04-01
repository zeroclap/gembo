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

// --- Positions ---

export type PositionStatus =
  | "pending"
  | "opening"
  | "open"
  | "partially_open"
  | "closing"
  | "closed"
  | "failed";

export type SizeType = "usd" | "base";

export interface Position {
  id: string;
  symbol: string;
  status: PositionStatus;
  size_type: SizeType;
  requested_size: string;

  long_exchange_id: string;
  long_entry_price: string;
  long_filled_qty: string;
  long_entry_cost: string;
  long_entry_fees: string;

  short_exchange_id: string;
  short_entry_price: string;
  short_filled_qty: string;
  short_entry_cost: string;
  short_entry_fees: string;

  long_exit_price: string;
  long_close_fees: string;
  short_exit_price: string;
  short_close_fees: string;

  snapshot: FundingArbitrage | Record<string, never>;

  long_mark_price: string;
  short_mark_price: string;
  long_unrealized_pnl: string;
  short_unrealized_pnl: string;
  total_funding_pnl: string;

  error_message?: string;

  created_at: string;
  opened_at?: string;
  closed_at?: string;
  updated_at: string;
}

export interface ExchangeOrder {
  id: string;
  position_id: string;
  exchange_id: string;
  exchange_order_id: string;
  symbol: string;
  side: string;
  leg: string;
  action: string;
  requested_qty: string;
  filled_qty: string;
  average_price: string;
  cost: string;
  fee_cost: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface FundingPayment {
  id: string;
  position_id: string;
  exchange_id: string;
  leg: string;
  symbol: string;
  rate: string;
  payment: string;
  funded_at: string;
  created_at: string;
}

export interface CreatePositionRequest {
  symbol: string;
  long_exchange_id: string;
  short_exchange_id: string;
  size_type: SizeType;
  requested_size: string;
}

export function fetchPositions(params?: {
  status?: string;
  symbol?: string;
}): Promise<Position[]> {
  const query = new URLSearchParams();
  if (params?.status) query.set("status", params.status);
  if (params?.symbol) query.set("symbol", params.symbol);
  const qs = query.toString();
  return fetchApi<Position[]>(`/positions${qs ? `?${qs}` : ""}`);
}

export function fetchPosition(id: string): Promise<Position> {
  return fetchApi<Position>(`/positions/${id}`);
}

export function createPosition(req: CreatePositionRequest): Promise<Position> {
  return fetchApi<Position>("/positions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}

export function closePosition(id: string): Promise<Position> {
  return fetchApi<Position>(`/positions/${id}/close`, { method: "POST" });
}

export async function deletePosition(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/positions/${id}`, {
    method: "DELETE",
  });
  if (!res.ok) {
    const text = await res.text();
    let message = `API error: ${res.status} ${res.statusText}`;
    try {
      const json = JSON.parse(text);
      if (json.error?.message) message = json.error.message;
    } catch {
      // response wasn't JSON, use default message
    }
    throw new Error(message);
  }
}

export function fetchPositionOrders(id: string): Promise<ExchangeOrder[]> {
  return fetchApi<ExchangeOrder[]>(`/positions/${id}/orders`);
}

export function fetchPositionFunding(id: string): Promise<FundingPayment[]> {
  return fetchApi<FundingPayment[]>(`/positions/${id}/funding`);
}
