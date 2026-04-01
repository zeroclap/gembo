import { PositionsTable } from "./positions-table";

export default function PositionsPage() {
  return (
    <div className="flex-1 px-4 py-4 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-screen-xl">
        <PositionsTable />
      </div>
    </div>
  );
}
