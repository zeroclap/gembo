export type ExchangeId = "binance" | "bybit" | "gate" | "okx";

export interface ExchangeCredential {
  id: string;
  exchangeId: ExchangeId;
  title: string;
  apiKey: string;
  apiSecret: string;
  passphrase?: string;
  isDefault: boolean;
}

export const EXCHANGE_META: Record<ExchangeId, { label: string; hasPassphrase: boolean }> = {
  binance: { label: "Binance", hasPassphrase: false },
  bybit: { label: "Bybit", hasPassphrase: false },
  gate: { label: "Gate.io", hasPassphrase: false },
  okx: { label: "OKX", hasPassphrase: true },
};

export const EXCHANGES: ExchangeId[] = ["binance", "bybit", "gate", "okx"];
