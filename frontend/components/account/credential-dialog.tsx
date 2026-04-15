"use client";

import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  type ExchangeCredential,
  type ExchangeId,
  EXCHANGE_META,
} from "@/lib/account-store";

interface CredentialFormProps {
  exchangeId: ExchangeId;
  credential?: ExchangeCredential | null;
  onSave: (data: Omit<ExchangeCredential, "id">) => void;
  onCancel: () => void;
}

function CredentialForm({ exchangeId, credential, onSave, onCancel }: CredentialFormProps) {
  const meta = EXCHANGE_META[exchangeId];

  const [title, setTitle] = useState(credential?.title ?? "");
  const [key, setKey] = useState(credential?.key ?? "");
  const [secret, setSecret] = useState(credential?.secret ?? "");
  const [passphrase, setPassphrase] = useState(credential?.passphrase ?? "");
  const [showSecret, setShowSecret] = useState(false);
  const [showPassphrase, setShowPassphrase] = useState(false);

  const isValid =
    title.trim() &&
    key.trim() &&
    secret.trim() &&
    (!meta.hasPassphrase || passphrase.trim());

  function handleSave() {
    if (!isValid) return;
    onSave({
      exchangeId,
      title: title.trim(),
      key: key.trim(),
      secret: secret.trim(),
      ...(meta.hasPassphrase ? { passphrase: passphrase.trim() } : {}),
    });
  }

  return (
    <>
      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="cred-title">Title / Note</Label>
          <Input
            id="cred-title"
            placeholder="e.g. Main account, Read-only…"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="cred-key">API Key</Label>
          <Input
            id="cred-key"
            placeholder="API key"
            value={key}
            onChange={(e) => setKey(e.target.value)}
            autoComplete="off"
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="cred-secret">Secret</Label>
          <div className="relative">
            <Input
              id="cred-secret"
              type={showSecret ? "text" : "password"}
              placeholder="Secret key"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              autoComplete="off"
              className="pr-8"
            />
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="absolute top-1/2 right-1 -translate-y-1/2"
              onClick={() => setShowSecret((v) => !v)}
              aria-label={showSecret ? "Hide secret" : "Show secret"}
            >
              {showSecret ? <EyeOff /> : <Eye />}
            </Button>
          </div>
        </div>

        {meta.hasPassphrase && (
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cred-passphrase">Passphrase</Label>
            <div className="relative">
              <Input
                id="cred-passphrase"
                type={showPassphrase ? "text" : "password"}
                placeholder="Passphrase"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                autoComplete="off"
                className="pr-8"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="absolute top-1/2 right-1 -translate-y-1/2"
                onClick={() => setShowPassphrase((v) => !v)}
                aria-label={showPassphrase ? "Hide passphrase" : "Show passphrase"}
              >
                {showPassphrase ? <EyeOff /> : <Eye />}
              </Button>
            </div>
          </div>
        )}
      </div>

      <DialogFooter>
        <Button variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={handleSave} disabled={!isValid}>
          {credential ? "Save changes" : "Add credential"}
        </Button>
      </DialogFooter>
    </>
  );
}

interface CredentialDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  exchangeId: ExchangeId;
  credential?: ExchangeCredential | null;
  onSave: (data: Omit<ExchangeCredential, "id">) => void;
}

export function CredentialDialog({
  open,
  onOpenChange,
  exchangeId,
  credential,
  onSave,
}: CredentialDialogProps) {
  const meta = EXCHANGE_META[exchangeId];
  const isEditing = !!credential;

  // key forces form to remount with fresh state whenever dialog opens
  const formKey = open ? `${exchangeId}-${credential?.id ?? "new"}` : "closed";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isEditing ? "Edit" : "Add"} {meta.label} Credential
          </DialogTitle>
        </DialogHeader>
        <CredentialForm
          key={formKey}
          exchangeId={exchangeId}
          credential={credential}
          onSave={(data) => {
            onSave(data);
            onOpenChange(false);
          }}
          onCancel={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}
