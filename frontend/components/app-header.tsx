"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTheme } from "next-themes";
import { useState, useSyncExternalStore } from "react";
import { Sun, Moon, RefreshCw, Activity, User } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

const emptySubscribe = () => () => {};
const getTrue = () => true;
const getFalse = () => false;

const navTabs = [
  { label: "Arbitrage", href: "/arbitrage", icon: Activity },
];

const mobileTabs = [
  { label: "Arbitrage", href: "/arbitrage", icon: Activity },
  { label: "Account", href: "/account", icon: User },
];

export function AppHeader() {
  const pathname = usePathname();
  const { theme, setTheme } = useTheme();
  const queryClient = useQueryClient();
  const mounted = useSyncExternalStore(emptySubscribe, getTrue, getFalse);
  const [isRefreshing, setIsRefreshing] = useState(false);

  return (
    <>
      {/* ------------------------------------------------------------------ */}
      {/* Desktop top bar — hidden on mobile                                  */}
      {/* ------------------------------------------------------------------ */}
      <header className="hidden md:block sticky top-0 z-50 border-b bg-background/80 backdrop-blur-sm">
        <div className="mx-auto flex h-11 max-w-screen-xl items-center justify-between px-4 sm:px-6 lg:px-8">
          <nav className="flex items-center gap-1">
            {navTabs.map((tab) => {
              const isActive =
                pathname === tab.href || pathname.startsWith(tab.href + "/");
              const Icon = tab.icon;
              return (
                <Link
                  key={tab.href}
                  href={tab.href}
                  className={cn(
                    "flex items-center gap-1.5 relative px-3 py-1.5 text-xs font-medium transition-colors rounded-md",
                    isActive
                      ? "text-foreground bg-muted"
                      : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
                  )}
                >
                  <Icon className="h-3.5 w-3.5" />
                  {tab.label}
                </Link>
              );
            })}
          </nav>

          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={() => {
                setIsRefreshing(true);
                queryClient.invalidateQueries().then(() => {
                  setIsRefreshing(false);
                });
              }}
              aria-label="Refresh data"
            >
              <RefreshCw
                className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")}
              />
            </Button>
            {mounted && (
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
                aria-label="Toggle theme"
              >
                {theme === "dark" ? (
                  <Sun className="h-3.5 w-3.5" />
                ) : (
                  <Moon className="h-3.5 w-3.5" />
                )}
              </Button>
            )}
            <Button
              variant="ghost"
              size="icon"
              className={cn(
                "h-8 w-8",
                (pathname === "/account" || pathname.startsWith("/account/")) &&
                  "bg-muted text-foreground"
              )}
              asChild
              aria-label="Account"
            >
              <Link href="/account">
                <User className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </div>
        </div>
      </header>

      {/* ------------------------------------------------------------------ */}
      {/* Mobile bottom nav — fixed, icon + label, shown only on mobile       */}
      {/* ------------------------------------------------------------------ */}
      <nav className="md:hidden fixed bottom-0 inset-x-0 z-50 flex border-t bg-background/95 backdrop-blur-sm">
        {mobileTabs.map((tab) => {
          const isActive =
            pathname === tab.href || pathname.startsWith(tab.href + "/");
          const Icon = tab.icon;
          return (
            <Link
              key={tab.href}
              href={tab.href}
              className={cn(
                "flex flex-1 flex-col items-center justify-center gap-1 py-2 text-[0.625rem] font-medium transition-colors",
                isActive
                  ? "text-primary"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              <Icon className={cn("h-5 w-5", isActive && "stroke-[2.5]")} />
              {tab.label}
            </Link>
          );
        })}
      </nav>
    </>
  );
}
