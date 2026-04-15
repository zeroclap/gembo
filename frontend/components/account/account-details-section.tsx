"use client";

import { useState } from "react";
import { Pencil, Check, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useAccountStore } from "@/lib/account-store";

export function AccountDetailsSection() {
  const { email, setEmail } = useAccountStore();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");

  function startEdit() {
    setDraft(email);
    setEditing(true);
  }

  function cancelEdit() {
    setEditing(false);
  }

  function saveEdit() {
    setEmail(draft.trim());
    setEditing(false);
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter") saveEdit();
    if (e.key === "Escape") cancelEdit();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Account Details</CardTitle>
        <CardDescription>Your profile information.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground w-12 shrink-0">Email</span>
          {editing ? (
            <>
              <Input
                autoFocus
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="you@example.com"
                className="h-6 text-xs max-w-64"
              />
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={saveEdit}
                aria-label="Save email"
              >
                <Check />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={cancelEdit}
                aria-label="Cancel"
              >
                <X />
              </Button>
            </>
          ) : (
            <>
              <span className="text-xs font-medium flex-1">
                {email || <span className="text-muted-foreground italic">Not set</span>}
              </span>
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={startEdit}
                aria-label="Edit email"
              >
                <Pencil />
              </Button>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
