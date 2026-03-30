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
