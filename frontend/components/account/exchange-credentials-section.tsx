"use client";

import { useState } from "react";
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
  useAccountStore,
  type ExchangeId,
  type ExchangeCredential,
  EXCHANGE_META,
  EXCHANGES,
} from "@/lib/account-store";
import { CredentialDialog } from "@/components/account/credential-dialog";
import { cn } from "@/lib/utils";

interface ExchangeRowProps {
  exchangeId: ExchangeId;
  credentials: ExchangeCredential[];
  defaultId: string | null;
}

function ExchangeRow({ exchangeId, credentials, defaultId }: ExchangeRowProps) {
  const { addCredential, updateCredential, removeCredential, setDefault } =
    useAccountStore();
  const meta = EXCHANGE_META[exchangeId];

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
                    {cred.key.slice(0, 8)}…{cred.key.slice(-4)}
                  </span>
                </div>

                <div className="flex items-center gap-0.5 shrink-0">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() =>
                          setDefault(exchangeId, isDefault ? null : cred.id)
                        }
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
                          onClick={() => removeCredential(cred.id)}
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
        onSave={(data) => addCredential(data)}
      />

      {/* Edit dialog */}
      <CredentialDialog
        open={!!editTarget}
        onOpenChange={(open) => { if (!open) setEditTarget(null); }}
        exchangeId={exchangeId}
        credential={editTarget}
        onSave={(data) => {
          if (editTarget) updateCredential(editTarget.id, data);
        }}
      />
    </div>
  );
}

export function ExchangeCredentialsSection() {
  const { credentials, defaults } = useAccountStore();

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
        {EXCHANGES.map((exchangeId, idx) => (
          <div key={exchangeId}>
            <ExchangeRow
              exchangeId={exchangeId}
              credentials={credentials.filter((c) => c.exchangeId === exchangeId)}
              defaultId={defaults[exchangeId]}
            />
            {idx < EXCHANGES.length - 1 && <Separator className="mt-5" />}
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
