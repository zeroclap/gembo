"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2, Bell } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import {
  type AlertRule,
  type AlertRuleType,
  type NotificationChannel,
  fetchAlertRules,
  fetchNotificationChannels,
  createAlertRule,
  updateAlertRule,
  deleteAlertRule,
} from "@/lib/api";

const RULE_TYPE_LABELS: Record<AlertRuleType, string> = {
  funding_apr_threshold: "Funding APR threshold",
  spread_threshold: "Spread threshold",
  position_opened: "Position opened",
  position_closed: "Position closed",
  funding_payment: "Funding payment received",
  margin_warning: "Margin health warning",
};

const RULE_TYPE_DESCRIPTIONS: Record<AlertRuleType, string> = {
  funding_apr_threshold: "Fire when funding arbitrage APR exceeds a minimum.",
  spread_threshold: "Fire when spread arbitrage net spread exceeds a minimum.",
  position_opened: "Fire when a position is opened.",
  position_closed: "Fire when a position is closed.",
  funding_payment: "Fire when a funding payment is received.",
  margin_warning: "Fire when margin ratio drops below a threshold.",
};

interface RuleFormState {
  channel_id: string;
  type: AlertRuleType;
  enabled: boolean;
  cooldown_minutes: number;
  // funding_apr_threshold
  min_apr: string;
  apr_symbol: string;
  apr_long_exchange: string;
  apr_short_exchange: string;
  // spread_threshold
  min_spread_pct: string;
  spread_symbol: string;
  // margin_warning
  max_margin_ratio: string;
  // position events
  event_symbol: string;
}

const DEFAULT_FORM: RuleFormState = {
  channel_id: "",
  type: "funding_apr_threshold",
  enabled: true,
  cooldown_minutes: 60,
  min_apr: "50",
  apr_symbol: "",
  apr_long_exchange: "",
  apr_short_exchange: "",
  min_spread_pct: "0.1",
  spread_symbol: "",
  max_margin_ratio: "0.2",
  event_symbol: "",
};

function ruleToForm(rule: AlertRule): RuleFormState {
  const p = rule.params as Record<string, string>;
  return {
    channel_id: rule.channel_id,
    type: rule.type,
    enabled: rule.enabled,
    cooldown_minutes: rule.cooldown_minutes,
    min_apr: p.min_apr ?? "50",
    apr_symbol: p.symbol ?? "",
    apr_long_exchange: p.long_exchange_id ?? "",
    apr_short_exchange: p.short_exchange_id ?? "",
    min_spread_pct: p.min_spread_pct ?? "0.1",
    spread_symbol: p.symbol ?? "",
    max_margin_ratio: p.max_margin_ratio ?? "0.2",
    event_symbol: p.symbol ?? "",
  };
}

function formToRule(
  form: RuleFormState
): Omit<AlertRule, "id" | "user_id" | "last_triggered_at" | "created_at" | "updated_at"> {
  let params: Record<string, string> = {};
  switch (form.type) {
    case "funding_apr_threshold":
      params = { min_apr: form.min_apr };
      if (form.apr_symbol) params.symbol = form.apr_symbol;
      if (form.apr_long_exchange) params.long_exchange_id = form.apr_long_exchange;
      if (form.apr_short_exchange) params.short_exchange_id = form.apr_short_exchange;
      break;
    case "spread_threshold":
      params = { min_spread_pct: form.min_spread_pct };
      if (form.spread_symbol) params.symbol = form.spread_symbol;
      break;
    case "margin_warning":
      params = { max_margin_ratio: form.max_margin_ratio };
      break;
    case "position_opened":
    case "position_closed":
    case "funding_payment":
      if (form.event_symbol) params.symbol = form.event_symbol;
      break;
  }
  return {
    channel_id: form.channel_id,
    type: form.type,
    params,
    enabled: form.enabled,
    cooldown_minutes: form.cooldown_minutes,
  };
}

interface RuleDialogProps {
  open: boolean;
  onClose: () => void;
  initial?: AlertRule | null;
  channels: NotificationChannel[];
  onSave: (data: ReturnType<typeof formToRule>) => void;
  isPending: boolean;
}

function RuleDialog({ open, onClose, initial, channels, onSave, isPending }: RuleDialogProps) {
  const [form, setForm] = useState<RuleFormState>(() =>
    initial ? ruleToForm(initial) : { ...DEFAULT_FORM, channel_id: channels[0]?.id ?? "" }
  );

  const set = (patch: Partial<RuleFormState>) => setForm((f) => ({ ...f, ...patch }));

  const isValid = !!form.channel_id;

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-sm font-semibold">
            {initial ? "Edit alert rule" : "Add alert rule"}
          </DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4 py-2">
          {/* Channel */}
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Channel</Label>
            {channels.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                Create a notification channel first.
              </p>
            ) : (
              <Select value={form.channel_id} onValueChange={(v) => set({ channel_id: v })}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="Select channel…" />
                </SelectTrigger>
                <SelectContent>
                  {channels.map((ch) => (
                    <SelectItem key={ch.id} value={ch.id} className="text-xs">
                      {ch.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          {/* Rule type */}
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Alert type</Label>
            <Select value={form.type} onValueChange={(v) => set({ type: v as AlertRuleType })}>
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(RULE_TYPE_LABELS) as AlertRuleType[]).map((t) => (
                  <SelectItem key={t} value={t} className="text-xs">
                    {RULE_TYPE_LABELS[t]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-[0.625rem] text-muted-foreground">{RULE_TYPE_DESCRIPTIONS[form.type]}</p>
          </div>

          {/* Type-specific params */}
          {form.type === "funding_apr_threshold" && (
            <>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs">Minimum APR (%)</Label>
                <Input
                  className="text-xs h-8"
                  type="number"
                  min="0"
                  step="1"
                  value={form.min_apr}
                  onChange={(e) => set({ min_apr: e.target.value })}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs">Symbol filter <span className="text-muted-foreground">(optional)</span></Label>
                <Input className="text-xs h-8" placeholder="BTC/USDT:USDT" value={form.apr_symbol} onChange={(e) => set({ apr_symbol: e.target.value })} />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div className="flex flex-col gap-1.5">
                  <Label className="text-xs">Long exchange <span className="text-muted-foreground">(opt.)</span></Label>
                  <Input className="text-xs h-8" placeholder="bybit" value={form.apr_long_exchange} onChange={(e) => set({ apr_long_exchange: e.target.value })} />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label className="text-xs">Short exchange <span className="text-muted-foreground">(opt.)</span></Label>
                  <Input className="text-xs h-8" placeholder="okx" value={form.apr_short_exchange} onChange={(e) => set({ apr_short_exchange: e.target.value })} />
                </div>
              </div>
            </>
          )}

          {form.type === "spread_threshold" && (
            <>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs">Minimum spread (%)</Label>
                <Input
                  className="text-xs h-8"
                  type="number"
                  min="0"
                  step="0.01"
                  value={form.min_spread_pct}
                  onChange={(e) => set({ min_spread_pct: e.target.value })}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs">Symbol filter <span className="text-muted-foreground">(optional)</span></Label>
                <Input className="text-xs h-8" placeholder="BTC/USDT:USDT" value={form.spread_symbol} onChange={(e) => set({ spread_symbol: e.target.value })} />
              </div>
            </>
          )}

          {form.type === "margin_warning" && (
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Max margin ratio (0–1)</Label>
              <Input
                className="text-xs h-8"
                type="number"
                min="0"
                max="1"
                step="0.01"
                value={form.max_margin_ratio}
                onChange={(e) => set({ max_margin_ratio: e.target.value })}
              />
              <p className="text-[0.625rem] text-muted-foreground">
                Alert fires when margin ratio drops below this value. e.g. 0.2 = 20%.
              </p>
            </div>
          )}

          {(form.type === "position_opened" ||
            form.type === "position_closed" ||
            form.type === "funding_payment") && (
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Symbol filter <span className="text-muted-foreground">(optional)</span></Label>
              <Input className="text-xs h-8" placeholder="BTC/USDT:USDT" value={form.event_symbol} onChange={(e) => set({ event_symbol: e.target.value })} />
            </div>
          )}

          {/* Cooldown */}
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Cooldown (minutes)</Label>
            <Input
              className="text-xs h-8"
              type="number"
              min="1"
              value={form.cooldown_minutes}
              onChange={(e) => set({ cooldown_minutes: parseInt(e.target.value, 10) || 60 })}
            />
            <p className="text-[0.625rem] text-muted-foreground">
              Minimum time between repeated alerts of the same rule.
            </p>
          </div>

          {/* Enabled */}
          <div className="flex items-center gap-2">
            <Switch
              id="rule-enabled"
              checked={form.enabled}
              onCheckedChange={(v) => set({ enabled: v })}
            />
            <Label htmlFor="rule-enabled" className="text-xs cursor-pointer">
              Enabled
            </Label>
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" size="sm" className="text-xs" onClick={onClose}>
            Cancel
          </Button>
          <Button
            size="sm"
            className="text-xs"
            onClick={() => onSave(formToRule(form))}
            disabled={!isValid || isPending || channels.length === 0}
          >
            {isPending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function AlertRulesSection() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<AlertRule | null>(null);

  const { data: rules = [], isLoading: rulesLoading } = useQuery({
    queryKey: ["alert-rules"],
    queryFn: fetchAlertRules,
  });

  const { data: channels = [] } = useQuery({
    queryKey: ["notification-channels"],
    queryFn: fetchNotificationChannels,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["alert-rules"] });

  const createMut = useMutation({
    mutationFn: createAlertRule,
    onSuccess: () => { invalidate(); setDialogOpen(false); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof updateAlertRule>[1] }) =>
      updateAlertRule(id, data),
    onSuccess: () => { invalidate(); setEditTarget(null); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteAlertRule,
    onSuccess: invalidate,
  });

  const isPending = createMut.isPending || updateMut.isPending;

  const channelName = (id: string) => channels.find((c) => c.id === id)?.name ?? id;

  return (
    <>
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-xs font-semibold">Alert rules</CardTitle>
              <CardDescription className="text-[0.625rem] mt-0.5">
                Conditions that trigger notifications to your channels.
              </CardDescription>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="text-xs h-7 gap-1"
              onClick={() => { setEditTarget(null); setDialogOpen(true); }}
            >
              <Plus className="h-3 w-3" />
              Add rule
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {rulesLoading ? (
            <p className="text-xs text-muted-foreground">Loading…</p>
          ) : rules.length === 0 ? (
            <p className="text-xs text-muted-foreground">
              No rules configured. Add a rule to start receiving alerts.
            </p>
          ) : (
            <div className="flex flex-col gap-0">
              {rules.map((rule, i) => (
                <div key={rule.id}>
                  {i > 0 && <Separator className="my-2" />}
                  <div className="flex items-start justify-between py-0.5 gap-2">
                    <div className="flex items-start gap-2 min-w-0">
                      <Bell className="h-3.5 w-3.5 mt-0.5 text-muted-foreground shrink-0" />
                      <div className="flex flex-col min-w-0">
                        <span className="text-xs font-medium">
                          {RULE_TYPE_LABELS[rule.type]}
                        </span>
                        <span className="text-[0.625rem] text-muted-foreground truncate">
                          via {channelName(rule.channel_id)} · cooldown {rule.cooldown_minutes}m
                          {rule.last_triggered_at &&
                            ` · last triggered ${new Date(rule.last_triggered_at).toLocaleString()}`}
                        </span>
                      </div>
                      <Badge
                        variant={rule.enabled ? "default" : "secondary"}
                        className="text-[0.625rem] px-1.5 py-0 shrink-0"
                      >
                        {rule.enabled ? "on" : "off"}
                      </Badge>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => { setEditTarget(rule); setDialogOpen(true); }}
                      >
                        <Pencil className="h-3 w-3" />
                      </Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button variant="ghost" size="icon" className="h-6 w-6 text-destructive">
                            <Trash2 className="h-3 w-3" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle className="text-sm">Delete rule?</AlertDialogTitle>
                            <AlertDialogDescription className="text-xs">
                              This alert rule will be permanently removed.
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel className="text-xs">Cancel</AlertDialogCancel>
                            <AlertDialogAction
                              className="text-xs"
                              onClick={() => deleteMut.mutate(rule.id)}
                            >
                              Delete
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <RuleDialog
        open={dialogOpen}
        onClose={() => { setDialogOpen(false); setEditTarget(null); }}
        initial={editTarget}
        channels={channels}
        isPending={isPending}
        onSave={(data) => {
          if (editTarget) {
            updateMut.mutate({ id: editTarget.id, data });
          } else {
            createMut.mutate(data);
          }
        }}
      />
    </>
  );
}
