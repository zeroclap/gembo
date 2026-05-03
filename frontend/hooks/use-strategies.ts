"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  fetchStrategies,
  createStrategy,
  patchStrategy,
  deleteStrategy,
  type Strategy,
  type CreateStrategyRequest,
  type PatchStrategyRequest,
} from "@/lib/api";

export const STRATEGIES_KEY = ["strategies"] as const;

export function useStrategies() {
  return useQuery<Strategy[]>({
    queryKey: STRATEGIES_KEY,
    queryFn: fetchStrategies,
  });
}

export function useCreateStrategy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateStrategyRequest) => createStrategy(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: STRATEGIES_KEY }),
  });
}

export function usePatchStrategy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, req }: { id: string; req: PatchStrategyRequest }) =>
      patchStrategy(id, req),
    onSuccess: () => qc.invalidateQueries({ queryKey: STRATEGIES_KEY }),
  });
}

export function useDeleteStrategy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteStrategy(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: STRATEGIES_KEY }),
  });
}
