import { NotificationChannelsSection } from "@/components/settings/notification-channels-section";
import { AlertRulesSection } from "@/components/settings/alert-rules-section";

export default function SettingsPage() {
  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-6 flex flex-col gap-6">
      <div>
        <h1 className="text-sm font-semibold">Settings</h1>
        <p className="text-xs text-muted-foreground mt-0.5">
          Configure notification channels and alert rules.
        </p>
      </div>
      <NotificationChannelsSection />
      <AlertRulesSection />
    </div>
  );
}
