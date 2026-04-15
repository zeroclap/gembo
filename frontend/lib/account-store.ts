"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

export type ExchangeId = "binance" | "bybit" | "gate" | "okx";

export interface ExchangeCredential {
  id: string;
  exchangeId: ExchangeId;
  title: string;
  key: string;
  secret: string;
  passphrase?: string;
}

export interface AccountState {
  email: string;
  credentials: ExchangeCredential[];
  defaults: Record<ExchangeId, string | null>;

  setEmail: (email: string) => void;
  addCredential: (cred: Omit<ExchangeCredential, "id">) => void;
  updateCredential: (id: string, cred: Partial<Omit<ExchangeCredential, "id">>) => void;
  removeCredential: (id: string) => void;
  setDefault: (exchangeId: ExchangeId, credentialId: string | null) => void;
}

export const EXCHANGE_META: Record<ExchangeId, { label: string; hasPassphrase: boolean }> = {
  binance: { label: "Binance", hasPassphrase: false },
  bybit: { label: "Bybit", hasPassphrase: false },
  gate: { label: "Gate.io", hasPassphrase: false },
  okx: { label: "OKX", hasPassphrase: true },
};

export const EXCHANGES: ExchangeId[] = ["binance", "bybit", "gate", "okx"];

let nextId = 1;
const genId = () => `cred-${Date.now()}-${nextId++}`;

export const useAccountStore = create<AccountState>()(
  persist(
    (set) => ({
      email: "",
      credentials: [],
      defaults: { binance: null, bybit: null, gate: null, okx: null },

      setEmail: (email) => set({ email }),

      addCredential: (cred) =>
        set((state) => ({
          credentials: [...state.credentials, { ...cred, id: genId() }],
        })),

      updateCredential: (id, cred) =>
        set((state) => ({
          credentials: state.credentials.map((c) =>
            c.id === id ? { ...c, ...cred } : c
          ),
        })),

      removeCredential: (id) =>
        set((state) => {
          const newDefaults = { ...state.defaults };
          for (const key of Object.keys(newDefaults) as ExchangeId[]) {
            if (newDefaults[key] === id) newDefaults[key] = null;
          }
          return {
            credentials: state.credentials.filter((c) => c.id !== id),
            defaults: newDefaults,
          };
        }),

      setDefault: (exchangeId, credentialId) =>
        set((state) => ({
          defaults: { ...state.defaults, [exchangeId]: credentialId },
        })),
    }),
    { name: "gembo-account" }
  )
);
