/**
 * Shared formatting utilities for the monitor UI.
 */

export function formatPct(value: string, decimals = 4): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(decimals)}%`;
}

export function formatRate(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  const pct = num * 100;
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(4)}%`;
}

export function formatPrice(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "-";
  if (num >= 1)
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  if (num >= 0.01)
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 4,
      maximumFractionDigits: 4,
    });
  return num.toLocaleString(undefined, {
    minimumFractionDigits: 6,
    maximumFractionDigits: 8,
  });
}

export function formatVolume(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  if (num >= 1_000_000_000) return `${(num / 1_000_000_000).toFixed(2)}B`;
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(2)}M`;
  if (num >= 1_000) return `${(num / 1_000).toFixed(2)}K`;
  return num.toFixed(2);
}

export function formatAPR(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num)) return "-";
  return `${num >= 0 ? "+" : ""}${num.toFixed(2)}%`;
}

export function formatUsd(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "$0.00";
  const abs = Math.abs(num);
  const sign = num >= 0 ? "+" : "-";
  if (abs >= 1000) {
    return `${sign}$${abs.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }
  if (abs >= 1) return `${sign}$${abs.toFixed(2)}`;
  return `${sign}$${abs.toFixed(4)}`;
}

export function formatUsdPlain(value: string): string {
  const num = parseFloat(value);
  if (!isFinite(num)) return "-";
  const abs = Math.abs(num);
  if (abs >= 1000) {
    return `$${abs.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }
  if (abs >= 1) return `$${abs.toFixed(2)}`;
  return `$${abs.toFixed(4)}`;
}

export function formatQty(value: string): string {
  const num = parseFloat(value);
  if (isNaN(num) || num === 0) return "-";
  if (num >= 1) return num.toFixed(4);
  return num.toFixed(8);
}

export function formatTimeShort(iso: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "-";
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatTimeRelative(iso: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "-";
  const diff = Date.now() - d.getTime();
  if (diff < 0) return "just now";
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function formatDuration(from: string, to?: string): string {
  if (!from) return "-";
  const start = new Date(from).getTime();
  if (isNaN(start)) return "-";
  const end = to ? new Date(to).getTime() : Date.now();
  if (isNaN(end)) return "-";
  const diffMs = end - start;
  if (diffMs < 0) return "-";
  const hours = Math.floor(diffMs / (1000 * 60 * 60));
  const mins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));
  if (hours >= 24) {
    const days = Math.floor(hours / 24);
    const remHours = hours % 24;
    return `${days}d ${remHours}h`;
  }
  return `${hours}h ${mins}m`;
}

export function formatCountdown(targetIso: string): string {
  if (!targetIso) return "-";
  const target = new Date(targetIso).getTime();
  if (isNaN(target)) return "-";
  const diff = target - Date.now();
  if (diff <= 0) return "now";
  const hours = Math.floor(diff / (1000 * 60 * 60));
  const mins = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
  const secs = Math.floor((diff % (1000 * 60)) / 1000);
  if (hours > 0) return `${hours}h ${mins}m`;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${secs}s`;
}
