"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2, Send, CheckCircle2, XCircle } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import {
  type NotificationChannel,
  type TelegramConfig,
  fetchNotificationChannels,
  createNotificationChannel,
  updateNotificationChannel,
  deleteNotificationChannel,
  testNotificationChannel,
} from "@/lib/api";

interface ChannelFormState {
  name: string;
  enabled: boolean;
  bot_token: string;
  chat_id: string;
}

const DEFAULT_FORM: ChannelFormState = {
  name: "",
  enabled: true,
  bot_token: "",
  chat_id: "",
};

interface ChannelDialogProps {
  open: boolean;
  onClose: () => void;
  initial?: NotificationChannel | null;
  onSave: (data: Omit<NotificationChannel, "id" | "user_id" | "created_at" | "updated_at">) => void;
  isPending: boolean;
}

function ChannelDialog({ open, onClose, initial, onSave, isPending }: ChannelDialogProps) {
  const [form, setForm] = useState<ChannelFormState>(() => {
    if (initial) {
      const cfg = initial.config as TelegramConfig;
      return {
        name: initial.name,
        enabled: initial.enabled,
        bot_token: cfg?.bot_token ?? "",
        chat_id: cfg?.chat_id ?? "",
      };
    }
    return { ...DEFAULT_FORM };
  });

  function handleSave() {
    onSave({
      type: "telegram",
      name: form.name.trim(),
      enabled: form.enabled,
      config: { bot_token: form.bot_token.trim(), chat_id: form.chat_id.trim() },
    });
  }

  const isValid = form.name.trim() && form.bot_token.trim() && form.chat_id.trim();

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-sm font-semibold">
            {initial ? "Edit Telegram Channel" : "Add Telegram Channel"}
          </DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Channel name</Label>
            <Input
              className="text-xs h-8"
              placeholder="My Telegram bot"
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Bot token</Label>
            <Input
              className="text-xs h-8 font-mono"
              placeholder="1234567890:ABCDefGH..."
              value={form.bot_token}
              onChange={(e) => setForm((f) => ({ ...f, bot_token: e.target.value }))}
              type="password"
              autoComplete="off"
            />
            <p className="text-[0.625rem] text-muted-foreground">
              Obtain from @BotFather on Telegram.
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Chat ID</Label>
            <Input
              className="text-xs h-8 font-mono"
              placeholder="-1001234567890 or @username"
              value={form.chat_id}
              onChange={(e) => setForm((f) => ({ ...f, chat_id: e.target.value }))}
            />
            <p className="text-[0.625rem] text-muted-foreground">
              User ID, group ID, or @channel_username.
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Switch
              id="ch-enabled"
              checked={form.enabled}
              onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))}
            />
            <Label htmlFor="ch-enabled" className="text-xs cursor-pointer">
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
            onClick={handleSave}
            disabled={!isValid || isPending}
          >
            {isPending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function NotificationChannelsSection() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<NotificationChannel | null>(null);
  const [testResult, setTestResult] = useState<{ id: string; ok: boolean } | null>(null);

  const { data: channels = [], isLoading } = useQuery({
    queryKey: ["notification-channels"],
    queryFn: fetchNotificationChannels,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["notification-channels"] });

  const createMut = useMutation({
    mutationFn: createNotificationChannel,
    onSuccess: () => { invalidate(); setDialogOpen(false); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof updateNotificationChannel>[1] }) =>
      updateNotificationChannel(id, data),
    onSuccess: () => { invalidate(); setEditTarget(null); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteNotificationChannel,
    onSuccess: invalidate,
  });

  const testMut = useMutation({
    mutationFn: testNotificationChannel,
    onSuccess: (_, id) => {
      setTestResult({ id, ok: true });
      setTimeout(() => setTestResult(null), 3000);
    },
    onError: (_, id) => {
      setTestResult({ id, ok: false });
      setTimeout(() => setTestResult(null), 4000);
    },
  });

  const isPending = createMut.isPending || updateMut.isPending;

  return (
    <>
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-xs font-semibold">Notification channels</CardTitle>
              <CardDescription className="text-[0.625rem] mt-0.5">
                Delivery endpoints for alerts (e.g. Telegram bots).
              </CardDescription>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="text-xs h-7 gap-1"
              onClick={() => { setEditTarget(null); setDialogOpen(true); }}
            >
              <Plus className="h-3 w-3" />
              Add channel
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-xs text-muted-foreground">Loading…</p>
          ) : channels.length === 0 ? (
            <p className="text-xs text-muted-foreground">
              No channels configured. Add a Telegram bot to get started.
            </p>
          ) : (
            <div className="flex flex-col gap-0">
              {channels.map((ch, i) => {
                const result = testResult?.id === ch.id ? testResult : null;
                return (
                  <div key={ch.id}>
                    {i > 0 && <Separator className="my-2" />}
                    <div className="flex items-center justify-between py-0.5">
                      <div className="flex items-center gap-2 min-w-0">
                        <div className="flex flex-col min-w-0">
                          <span className="text-xs font-medium truncate">{ch.name}</span>
                          <span className="text-[0.625rem] text-muted-foreground capitalize">
                            {ch.type}
                          </span>
                        </div>
                        <Badge
                          variant={ch.enabled ? "default" : "secondary"}
                          className="text-[0.625rem] px-1.5 py-0 shrink-0"
                        >
                          {ch.enabled ? "on" : "off"}
                        </Badge>
                        {result && (
                          <span className={`text-[0.625rem] flex items-center gap-1 ${result.ok ? "text-green-600" : "text-destructive"}`}>
                            {result.ok ? (
                              <><CheckCircle2 className="h-3 w-3" /> Sent</>
                            ) : (
                              <><XCircle className="h-3 w-3" /> Failed</>
                            )}
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-1 shrink-0">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-6 w-6"
                          title="Send test message"
                          disabled={testMut.isPending}
                          onClick={() => testMut.mutate(ch.id)}
                        >
                          <Send className="h-3 w-3" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-6 w-6"
                          onClick={() => { setEditTarget(ch); setDialogOpen(true); }}
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
                              <AlertDialogTitle className="text-sm">Delete channel?</AlertDialogTitle>
                              <AlertDialogDescription className="text-xs">
                                This will also delete all alert rules using this channel.
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel className="text-xs">Cancel</AlertDialogCancel>
                              <AlertDialogAction
                                className="text-xs"
                                onClick={() => deleteMut.mutate(ch.id)}
                              >
                                Delete
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <ChannelDialog
        open={dialogOpen}
        onClose={() => { setDialogOpen(false); setEditTarget(null); }}
        initial={editTarget}
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
