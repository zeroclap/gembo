import { ArbitrageTable } from "./arbitrage-table";

export default function ArbitragePage() {
  return (
    <div className="flex-1 px-4 py-4 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-screen-xl">
        <ArbitrageTable />
      </div>
    </div>
  );
}
