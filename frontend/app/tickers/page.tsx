import { TickersTable } from "./tickers-table";

export default function TickersPage() {
  return (
    <div className="flex-1 px-4 py-4 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-screen-xl">
        <TickersTable />
      </div>
    </div>
  );
}
