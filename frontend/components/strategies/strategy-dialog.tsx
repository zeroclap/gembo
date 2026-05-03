"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import { X, Plus } from "lucide-react";
import { type Strategy, type CreateStrategyRequest, type ArbitrageType } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { fetchExchangeCredentials } from "@/lib/api";

interface StrategyFormState {
  name: string;
  enabled: boolean;
  arbitrage_type: ArbitrageType;
  min_apr: string;
  min_net_spread_pct: string;
  symbol_whitelist: string[];
  exchange_whitelist: string[];
  credentials: Record<string, string>;
  size_usd: string;
  leverage: string;
  max_concurrent_positions: string;
  target_spread_pct: string;
  stop_loss_spread_pct: string;
  max_hold_duration: string;
}

const DEFAULT_FORM: StrategyFormState = {
  name: "",
  enabled: true,
  arbitrage_type: "funding",
  min_apr: "0",
  min_net_spread_pct: "0",
  symbol_whitelist: [],
  exchange_whitelist: [],
  credentials: {},
  size_usd: "100",
  leverage: "1",
  max_concurrent_positions: "1",
  target_spread_pct: "0",
  stop_loss_spread_pct: "0",
  max_hold_duration: "0",
};

function fromStrategy(s: Strategy): StrategyFormState {
  return {
    name: s.name,
    enabled: s.enabled,
    arbitrage_type: s.arbitrage_type,
    min_apr: s.min_apr,
    min_net_spread_pct: s.min_net_spread_pct,
    symbol_whitelist: s.symbol_whitelist ?? [],
    exchange_whitelist: s.exchange_whitelist ?? [],
    credentials: s.credentials ?? {},
    size_usd: s.size_usd,
    leverage: String(s.leverage),
    max_concurrent_positions: String(s.max_concurrent_positions),
    target_spread_pct: s.target_spread_pct,
    stop_loss_spread_pct: s.stop_loss_spread_pct,
    max_hold_duration: String(s.max_hold_duration),
  };
}

function toRequest(f: StrategyFormState): CreateStrategyRequest {
  return {
    name: f.name.trim(),
    enabled: f.enabled,
    arbitrage_type: f.arbitrage_type,
    min_apr: f.min_apr || "0",
    min_net_spread_pct: f.min_net_spread_pct || "0",
    symbol_whitelist: f.symbol_whitelist,
    exchange_whitelist: f.exchange_whitelist,
    credentials: f.credentials,
    size_usd: f.size_usd || "0",
    leverage: parseInt(f.leverage) || 1,
    max_concurrent_positions: parseInt(f.max_concurrent_positions) || 1,
    target_spread_pct: f.target_spread_pct || "0",
    stop_loss_spread_pct: f.stop_loss_spread_pct || "0",
    max_hold_duration: parseInt(f.max_hold_duration) || 0,
  };
}

interface Props {
  open: boolean;
  onClose: () => void;
  initial?: Strategy | null;
  onSave: (req: CreateStrategyRequest) => void;
  isPending: boolean;
}

function StrategyDialogInner({
  onClose,
  initial,
  onSave,
  isPending,
}: Omit<Props, "open">) {
  const [form, setForm] = useState<StrategyFormState>(() =>
    initial ? fromStrategy(initial) : { ...DEFAULT_FORM }
  );
  const [symbolInput, setSymbolInput] = useState("");
  const [exchangeInput, setExchangeInput] = useState("");
  const [credExchange, setCredExchange] = useState("");
  const [credId, setCredId] = useState("");

  const { data: credentials } = useQuery({
    queryKey: ["exchange-credentials"],
    queryFn: fetchExchangeCredentials,
  });

  function set<K extends keyof StrategyFormState>(key: K, value: StrategyFormState[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  function addSymbol() {
    const v = symbolInput.trim();
    if (v && !form.symbol_whitelist.includes(v)) {
      set("symbol_whitelist", [...form.symbol_whitelist, v]);
    }
    setSymbolInput("");
  }

  function removeSymbol(s: string) {
    set("symbol_whitelist", form.symbol_whitelist.filter((x) => x !== s));
  }

  function addExchange() {
    const v = exchangeInput.trim();
    if (v && !form.exchange_whitelist.includes(v)) {
      set("exchange_whitelist", [...form.exchange_whitelist, v]);
    }
    setExchangeInput("");
  }

  function removeExchange(e: string) {
    set("exchange_whitelist", form.exchange_whitelist.filter((x) => x !== e));
  }

  function addCredential() {
    if (!credExchange.trim() || !credId.trim()) return;
    set("credentials", { ...form.credentials, [credExchange.trim()]: credId.trim() });
    setCredExchange("");
    setCredId("");
  }

  function removeCredential(exchange: string) {
    const next = { ...form.credentials };
    delete next[exchange];
    set("credentials", next);
  }

  const isValid = form.name.trim() && parseFloat(form.size_usd) > 0;

  return (
    <>
      <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-sm font-semibold">
            {initial ? "Edit Strategy" : "New Strategy"}
          </DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4 py-1">
          {/* Basic */}
          <div className="flex gap-3">
            <div className="flex-1">
              <Label className="text-xs mb-1 block">Name</Label>
              <Input
                className="h-8 text-xs"
                value={form.name}
                onChange={(e) => set("name", e.target.value)}
                placeholder="My funding strategy"
              />
            </div>
            <div className="flex items-end gap-2 pb-0.5">
              <Label className="text-xs">Enabled</Label>
              <Switch
                checked={form.enabled}
                onCheckedChange={(v) => set("enabled", v)}
              />
            </div>
          </div>

          <div>
            <Label className="text-xs mb-1 block">Arbitrage Type</Label>
            <Select
              value={form.arbitrage_type}
              onValueChange={(v) => set("arbitrage_type", v as ArbitrageType)}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="funding" className="text-xs">Funding Rate</SelectItem>
                <SelectItem value="spread" className="text-xs">Spread</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Separator />

          {/* Criteria */}
          <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
            Entry Criteria
          </p>

          {form.arbitrage_type === "funding" && (
            <div>
              <Label className="text-xs mb-1 block">Min APR (%)</Label>
              <Input
                className="h-8 text-xs"
                type="number"
                min="0"
                step="0.1"
                value={form.min_apr}
                onChange={(e) => set("min_apr", e.target.value)}
              />
            </div>
          )}

          {form.arbitrage_type === "spread" && (
            <div>
              <Label className="text-xs mb-1 block">Min Net Spread (%)</Label>
              <Input
                className="h-8 text-xs"
                type="number"
                min="0"
                step="0.01"
                value={form.min_net_spread_pct}
                onChange={(e) => set("min_net_spread_pct", e.target.value)}
              />
            </div>
          )}

          {/* Symbol whitelist */}
          <div>
            <Label className="text-xs mb-1 block">Symbol Whitelist <span className="text-muted-foreground">(empty = any)</span></Label>
            <div className="flex gap-2">
              <Input
                className="h-8 text-xs"
                value={symbolInput}
                onChange={(e) => setSymbolInput(e.target.value)}
                placeholder="BTC/USDT:USDT"
                onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), addSymbol())}
              />
              <Button variant="outline" size="icon" className="h-8 w-8 shrink-0" onClick={addSymbol}>
                <Plus className="h-3.5 w-3.5" />
              </Button>
            </div>
            {form.symbol_whitelist.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-1.5">
                {form.symbol_whitelist.map((s) => (
                  <Badge key={s} variant="secondary" className="text-[10px] gap-1 pr-1">
                    {s}
                    <button onClick={() => removeSymbol(s)} className="hover:text-destructive">
                      <X className="h-2.5 w-2.5" />
                    </button>
                  </Badge>
                ))}
              </div>
            )}
          </div>

          {/* Exchange whitelist */}
          <div>
            <Label className="text-xs mb-1 block">Exchange Whitelist <span className="text-muted-foreground">(empty = any)</span></Label>
            <div className="flex gap-2">
              <Input
                className="h-8 text-xs"
                value={exchangeInput}
                onChange={(e) => setExchangeInput(e.target.value)}
                placeholder="bybit"
                onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), addExchange())}
              />
              <Button variant="outline" size="icon" className="h-8 w-8 shrink-0" onClick={addExchange}>
                <Plus className="h-3.5 w-3.5" />
              </Button>
            </div>
            {form.exchange_whitelist.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-1.5">
                {form.exchange_whitelist.map((e) => (
                  <Badge key={e} variant="secondary" className="text-[10px] gap-1 pr-1">
                    {e}
                    <button onClick={() => removeExchange(e)} className="hover:text-destructive">
                      <X className="h-2.5 w-2.5" />
                    </button>
                  </Badge>
                ))}
              </div>
            )}
          </div>

          <Separator />

          {/* Position parameters */}
          <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
            Position Parameters
          </p>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <Label className="text-xs mb-1 block">Size (USD)</Label>
              <Input
                className="h-8 text-xs"
                type="number"
                min="1"
                value={form.size_usd}
                onChange={(e) => set("size_usd", e.target.value)}
              />
            </div>
            <div>
              <Label className="text-xs mb-1 block">Leverage</Label>
              <Input
                className="h-8 text-xs"
                type="number"
                min="1"
                value={form.leverage}
                onChange={(e) => set("leverage", e.target.value)}
              />
            </div>
            <div>
              <Label className="text-xs mb-1 block">Max Concurrent</Label>
              <Input
                className="h-8 text-xs"
                type="number"
                min="1"
                value={form.max_concurrent_positions}
                onChange={(e) => set("max_concurrent_positions", e.target.value)}
              />
            </div>
          </div>

          {form.arbitrage_type === "spread" && (
            <div className="grid grid-cols-3 gap-3">
              <div>
                <Label className="text-xs mb-1 block">Target Spread (%)</Label>
                <Input
                  className="h-8 text-xs"
                  type="number"
                  min="0"
                  step="0.01"
                  value={form.target_spread_pct}
                  onChange={(e) => set("target_spread_pct", e.target.value)}
                />
              </div>
              <div>
                <Label className="text-xs mb-1 block">Stop Loss (%)</Label>
                <Input
                  className="h-8 text-xs"
                  type="number"
                  min="0"
                  step="0.01"
                  value={form.stop_loss_spread_pct}
                  onChange={(e) => set("stop_loss_spread_pct", e.target.value)}
                />
              </div>
              <div>
                <Label className="text-xs mb-1 block">Max Hold (sec)</Label>
                <Input
                  className="h-8 text-xs"
                  type="number"
                  min="0"
                  value={form.max_hold_duration}
                  onChange={(e) => set("max_hold_duration", e.target.value)}
                />
              </div>
            </div>
          )}

          <Separator />

          {/* Credentials */}
          <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
            Credentials
          </p>

          <div className="flex gap-2">
            <Input
              className="h-8 text-xs"
              value={credExchange}
              onChange={(e) => setCredExchange(e.target.value)}
              placeholder="exchange_id (e.g. bybit)"
            />
            {credentials && credentials.length > 0 ? (
              <Select value={credId} onValueChange={setCredId}>
                <SelectTrigger className="h-8 text-xs flex-1">
                  <SelectValue placeholder="Select credential" />
                </SelectTrigger>
                <SelectContent>
                  {credentials.map((c) => (
                    <SelectItem key={c.id} value={c.id} className="text-xs">
                      {c.title} ({c.exchange_id})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Input
                className="h-8 text-xs"
                value={credId}
                onChange={(e) => setCredId(e.target.value)}
                placeholder="credential_id"
              />
            )}
            <Button variant="outline" size="icon" className="h-8 w-8 shrink-0" onClick={addCredential}>
              <Plus className="h-3.5 w-3.5" />
            </Button>
          </div>

          {Object.entries(form.credentials).length > 0 && (
            <div className="flex flex-col gap-1">
              {Object.entries(form.credentials).map(([exch, cid]) => (
                <div key={exch} className="flex items-center justify-between text-xs rounded-md bg-muted px-2 py-1">
                  <span className="font-medium">{exch}</span>
                  <span className="text-muted-foreground truncate mx-2 text-[10px]">
                    {credentials?.find((c) => c.id === cid)?.title ?? cid}
                  </span>
                  <button
                    onClick={() => removeCredential(exch)}
                    className="text-muted-foreground hover:text-destructive"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" className="h-8 text-xs" onClick={onClose}>
            Cancel
          </Button>
          <Button
            size="sm"
            className="h-8 text-xs"
            disabled={!isValid || isPending}
            onClick={() => onSave(toRequest(form))}
          >
            {isPending ? "Saving…" : initial ? "Save Changes" : "Create Strategy"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </>
  );
}

export function StrategyDialog({ open, onClose, initial, onSave, isPending }: Props) {
  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <StrategyDialogInner
        key={initial?.id ?? "new"}
        onClose={onClose}
        initial={initial}
        onSave={onSave}
        isPending={isPending}
      />
    </Dialog>
  );
}
