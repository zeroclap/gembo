"use client";

import { useState } from "react";
import { Plus, Pencil, Trash2, Power } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
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
import { type Strategy } from "@/lib/api";
import {
  useStrategies,
  useCreateStrategy,
  usePatchStrategy,
  useDeleteStrategy,
} from "@/hooks/use-strategies";
import { StrategyDialog } from "@/components/strategies/strategy-dialog";

export function StrategiesSection() {
  const { data: strategies, isLoading } = useStrategies();
  const createMutation = useCreateStrategy();
  const patchMutation = usePatchStrategy();
  const deleteMutation = useDeleteStrategy();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Strategy | null>(null);

  function openCreate() {
    setEditing(null);
    setDialogOpen(true);
  }

  function openEdit(s: Strategy) {
    setEditing(s);
    setDialogOpen(true);
  }

  function handleClose() {
    setDialogOpen(false);
    setEditing(null);
  }

  function handleSave(data: Parameters<typeof createMutation.mutate>[0]) {
    if (editing) {
      patchMutation.mutate(
        { id: editing.id, req: data },
        { onSuccess: handleClose }
      );
    } else {
      createMutation.mutate(data, { onSuccess: handleClose });
    }
  }

  function handleToggle(s: Strategy) {
    patchMutation.mutate({ id: s.id, req: { enabled: !s.enabled } });
  }

  const isPending = createMutation.isPending || patchMutation.isPending;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-semibold">Strategies</CardTitle>
          <Button size="sm" variant="outline" className="h-7 text-xs gap-1" onClick={openCreate}>
            <Plus className="h-3 w-3" />
            New Strategy
          </Button>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {isLoading && (
          <p className="text-xs text-muted-foreground py-4 text-center">Loading…</p>
        )}
        {!isLoading && (!strategies || strategies.length === 0) && (
          <p className="text-xs text-muted-foreground py-4 text-center">
            No strategies yet. Create one to automate position opening.
          </p>
        )}
        {strategies && strategies.length > 0 && (
          <div className="flex flex-col gap-1">
            {strategies.map((s, i) => (
              <div key={s.id}>
                {i > 0 && <Separator className="my-1" />}
                <div className="flex items-center gap-2 py-1">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-medium truncate">{s.name}</span>
                      <Badge
                        variant={s.enabled ? "default" : "secondary"}
                        className="text-[10px] h-4 px-1.5"
                      >
                        {s.enabled ? "enabled" : "disabled"}
                      </Badge>
                      <Badge variant="outline" className="text-[10px] h-4 px-1.5">
                        {s.arbitrage_type}
                      </Badge>
                    </div>
                    <div className="text-[10px] text-muted-foreground mt-0.5 flex gap-2 flex-wrap">
                      <span>${s.size_usd} / pos</span>
                      {s.leverage > 1 && <span>{s.leverage}x leverage</span>}
                      {s.arbitrage_type === "funding" && parseFloat(s.min_apr) > 0 && (
                        <span>min APR {s.min_apr}%</span>
                      )}
                      {s.arbitrage_type === "spread" && parseFloat(s.min_net_spread_pct) > 0 && (
                        <span>min spread {s.min_net_spread_pct}%</span>
                      )}
                      {s.max_concurrent_positions > 0 && (
                        <span>max {s.max_concurrent_positions} concurrent</span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => handleToggle(s)}
                      aria-label={s.enabled ? "Disable strategy" : "Enable strategy"}
                    >
                      <Power className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => openEdit(s)}
                      aria-label="Edit strategy"
                    >
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-destructive hover:text-destructive"
                          aria-label="Delete strategy"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle className="text-sm">Delete strategy?</AlertDialogTitle>
                          <AlertDialogDescription className="text-xs">
                            This will permanently delete &ldquo;{s.name}&rdquo;. This cannot be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel className="h-8 text-xs">Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            className="h-8 text-xs bg-destructive hover:bg-destructive/90"
                            onClick={() => deleteMutation.mutate(s.id)}
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

      <StrategyDialog
        open={dialogOpen}
        onClose={handleClose}
        initial={editing}
        onSave={handleSave}
        isPending={isPending}
      />
    </Card>
  );
}
