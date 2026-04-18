"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2, Star } from "lucide-react";
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
import { Separator } from "@/components/ui/separator";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  type ExchangeId,
  type ExchangeCredential,
  EXCHANGE_META,
  EXCHANGES,
} from "@/lib/account-store";
import {
  createExchangeCredential,
  deleteExchangeCredential,
  fetchExchangeCredentials,
  updateExchangeCredential,
} from "@/lib/api";
import { CredentialDialog } from "@/components/account/credential-dialog";
import { cn } from "@/lib/utils";

interface ExchangeRowProps {
  exchangeId: ExchangeId;
  credentials: ExchangeCredential[];
  isPending: boolean;
  onCreate: (data: Omit<ExchangeCredential, "id">) => void;
  onUpdate: (id: string, data: Omit<ExchangeCredential, "id">) => void;
  onDelete: (id: string) => void;
}

function ExchangeRow({
  exchangeId,
  credentials,
  isPending,
  onCreate,
  onUpdate,
  onDelete,
}: ExchangeRowProps) {
  const meta = EXCHANGE_META[exchangeId];
  const defaultId = credentials.find((c) => c.isDefault)?.id ?? null;

  const [addOpen, setAddOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ExchangeCredential | null>(null);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium">{meta.label}</span>
          {meta.hasPassphrase && (
            <Badge variant="secondary" className="text-[0.625rem] px-1.5 py-0">
              passphrase
            </Badge>
          )}
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={isPending}
          onClick={() => setAddOpen(true)}
        >
          <Plus data-icon="inline-start" />
          Add
        </Button>
      </div>

      {credentials.length === 0 ? (
        <p className="text-xs text-muted-foreground italic">No credentials configured.</p>
      ) : (
        <div className="flex flex-col gap-1.5">
          {credentials.map((cred) => {
            const isDefault = cred.id === defaultId;
            return (
              <div
                key={cred.id}
                className={cn(
                  "flex items-center gap-2 rounded-md border px-3 py-2 transition-colors",
                  isDefault ? "border-primary/40 bg-primary/5" : "border-border bg-muted/30"
                )}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-1.5">
                    <span className="text-xs font-medium truncate">{cred.title}</span>
                    {isDefault && (
                      <Badge variant="secondary" className="text-[0.625rem] px-1.5 py-0 shrink-0">
                        default
                      </Badge>
                    )}
                  </div>
                  <span className="text-[0.625rem] text-muted-foreground font-mono truncate block">
                    {cred.apiKey.slice(0, 8)}…{cred.apiKey.slice(-4)}
                  </span>
                </div>

                <div className="flex items-center gap-0.5 shrink-0">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() =>
                          onUpdate(cred.id, {
                            exchangeId: cred.exchangeId,
                            title: cred.title,
                            apiKey: cred.apiKey,
                            apiSecret: cred.apiSecret,
                            passphrase: cred.passphrase,
                            isDefault: !isDefault,
                          })
                        }
                        disabled={isPending}
                        className={cn(isDefault && "text-primary")}
                        aria-label={isDefault ? "Unset default" : "Set as default"}
                      >
                        <Star />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>
                      {isDefault ? "Unset default" : "Set as default"}
                    </TooltipContent>
                  </Tooltip>

                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        disabled={isPending}
                        onClick={() => setEditTarget(cred)}
                        aria-label="Edit credential"
                      >
                        <Pencil />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>Edit</TooltipContent>
                  </Tooltip>

                  <AlertDialog>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={isPending}
                            className="text-destructive hover:text-destructive"
                            aria-label="Delete credential"
                          >
                            <Trash2 />
                          </Button>
                        </AlertDialogTrigger>
                      </TooltipTrigger>
                      <TooltipContent>Delete</TooltipContent>
                    </Tooltip>
                    <AlertDialogContent size="sm">
                      <AlertDialogHeader>
                        <AlertDialogTitle>Delete credential?</AlertDialogTitle>
                        <AlertDialogDescription>
                          &ldquo;{cred.title}&rdquo; will be removed permanently.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                          variant="destructive"
                          onClick={() => onDelete(cred.id)}
                        >
                          Delete
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Add dialog */}
      <CredentialDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        exchangeId={exchangeId}
        onSave={onCreate}
      />

      {/* Edit dialog */}
      <CredentialDialog
        open={!!editTarget}
        onOpenChange={(open) => { if (!open) setEditTarget(null); }}
        exchangeId={exchangeId}
        credential={editTarget}
        onSave={(data) => {
          if (editTarget) onUpdate(editTarget.id, data);
        }}
      />
    </div>
  );
}

function normalizeExchangeId(value: string): ExchangeId | null {
  if (value === "binance" || value === "bybit" || value === "gate" || value === "okx") {
    return value;
  }
  return null;
}

export function ExchangeCredentialsSection() {
  const queryClient = useQueryClient();

  const credentialsQuery = useQuery({
    queryKey: ["account", "credentials"],
    queryFn: fetchExchangeCredentials,
    select: (items): ExchangeCredential[] => {
      const result: ExchangeCredential[] = [];
      for (const item of items) {
        const exchangeId = normalizeExchangeId(item.exchange_id);
        if (!exchangeId) continue;
        result.push({
          id: item.id,
          exchangeId,
          title: item.title,
          apiKey: item.api_key,
          apiSecret: item.api_secret,
          passphrase: item.passphrase,
          isDefault: item.is_default,
        });
      }
      return result;
    },
  });

  const createMutation = useMutation({
    mutationFn: createExchangeCredential,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["account", "credentials"] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Omit<ExchangeCredential, "id"> }) =>
      updateExchangeCredential(id, {
        exchange_id: data.exchangeId,
        title: data.title,
        api_key: data.apiKey,
        api_secret: data.apiSecret,
        passphrase: data.passphrase,
        is_default: data.isDefault,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["account", "credentials"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteExchangeCredential,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["account", "credentials"] });
    },
  });

  const isPending =
    credentialsQuery.isLoading ||
    createMutation.isPending ||
    updateMutation.isPending ||
    deleteMutation.isPending;

  const credentials = credentialsQuery.data ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Exchange Credentials</CardTitle>
        <CardDescription>
          Manage API keys for each exchange. Mark one as default per exchange to
          use it automatically.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-5">
        {credentialsQuery.isError && (
          <p className="text-xs text-destructive">Failed to load credentials.</p>
        )}

        {EXCHANGES.map((exchangeId, idx) => (
          <div key={exchangeId}>
            <ExchangeRow
              exchangeId={exchangeId}
              credentials={credentials.filter((c) => c.exchangeId === exchangeId)}
              isPending={isPending}
              onCreate={(data) =>
                createMutation.mutate({
                  exchange_id: data.exchangeId,
                  title: data.title,
                  api_key: data.apiKey,
                  api_secret: data.apiSecret,
                  passphrase: data.passphrase,
                  is_default: data.isDefault,
                })
              }
              onUpdate={(id, data) => updateMutation.mutate({ id, data })}
              onDelete={(id) => deleteMutation.mutate(id)}
            />
            {idx < EXCHANGES.length - 1 && <Separator className="mt-5" />}
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
