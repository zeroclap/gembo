import { TickersTable } from "./tickers-table";

export default function TickersPage() {
  return (
    <div className="flex-1 px-4 py-6 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-screen-xl space-y-6">
        <div>
          <h1 className="text-lg font-heading font-semibold tracking-tight">Tickers</h1>
          <p className="text-xs text-muted-foreground">
            Real-time price data across all exchanges
          </p>
        </div>
        <TickersTable />
      </div>
    </div>
  );
}
