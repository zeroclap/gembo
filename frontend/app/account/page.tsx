import { AccountDetailsSection } from "@/components/account/account-details-section";
import { ExchangeCredentialsSection } from "@/components/account/exchange-credentials-section";

export default function AccountPage() {
  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-6 flex flex-col gap-6">
      <div>
        <h1 className="text-sm font-semibold">Account</h1>
        <p className="text-xs text-muted-foreground mt-0.5">
          Manage your profile and exchange credentials.
        </p>
      </div>
      <AccountDetailsSection />
      <ExchangeCredentialsSection />
    </div>
  );
}
