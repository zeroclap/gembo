import { StrategiesSection } from "@/components/strategies/strategies-section";

export default function StrategiesPage() {
  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-6 flex flex-col gap-6">
      <div>
        <h1 className="text-sm font-semibold">Strategies</h1>
        <p className="text-xs text-muted-foreground mt-0.5">
          Automated arbitrage strategies that open positions when opportunities match your criteria.
        </p>
      </div>
      <StrategiesSection />
    </div>
  );
}
