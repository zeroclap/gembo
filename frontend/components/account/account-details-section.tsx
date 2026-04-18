"use client";

import { useQuery } from "@tanstack/react-query";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { fetchAccount } from "@/lib/api";

export function AccountDetailsSection() {
  const accountQuery = useQuery({
    queryKey: ["account"],
    queryFn: fetchAccount,
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Account Details</CardTitle>
        <CardDescription>Your authenticated profile information.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground w-12 shrink-0">Email</span>
          {accountQuery.isLoading ? (
            <span className="text-xs text-muted-foreground">Loading...</span>
          ) : accountQuery.isError ? (
            <span className="text-xs text-destructive">Failed to load account details.</span>
          ) : (
            <span className="text-xs font-medium flex-1">
              {accountQuery.data?.email ?? <span className="text-muted-foreground italic">Not set</span>}
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
