import { useState } from "react";
import { Button, Text, Title } from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { ProfilePanel } from "./ProfilePanel";
import { AdminPanel } from "./AdminPanel";
import { SectionTabs } from "../../components/SectionTabs";
import type { Theme, User } from "../../types";
import { readStoredChoice, writeStoredChoice } from "../../lib/browserStorage";

type SettingsSection = "account" | "admin";

export function SettingsPage({
  user,
  theme,
  onThemeChange,
  onSignOut,
  signingOut,
  signOutError,
  onBack,
}: {
  user: User;
  theme: Theme;
  onThemeChange: (theme: Theme) => void;
  onSignOut: () => void;
  signingOut: boolean;
  signOutError?: string;
  onBack: () => void;
}) {
  const isAdmin = user.role === "admin";
  const storageKey = `visto:tab:${user.id}:settings`;
  const [section, setSection] = useState<SettingsSection>(() =>
    isAdmin
      ? readStoredChoice("session", storageKey, ["account", "admin"] as const, "account")
      : "account",
  );

  return (
    <div className="settings-page">
      <Button
        className="detail-back"
        variant="subtle"
        leftSection={<IconArrowLeft size={16} />}
        onClick={onBack}
      >
        Library
      </Button>
      <div className="page-heading">
        <Text className="section-kicker">{user.name || "Your account"}</Text>
        <Title order={1}>Settings</Title>
      </div>
      {isAdmin && (
        <SectionTabs
          label="Settings sections"
          value={section}
          onChange={(next) => {
            setSection(next);
            writeStoredChoice("session", storageKey, next);
          }}
          options={[
            { value: "account", label: "Account" },
            { value: "admin", label: "Admin" },
          ]}
        />
      )}
      <div key={section} className="section-panel">
        {section === "admin" && isAdmin ? (
          <AdminPanel currentUser={user} />
        ) : (
          <ProfilePanel
            theme={theme}
            onThemeChange={onThemeChange}
            onSignOut={onSignOut}
            signingOut={signingOut}
            signOutError={signOutError}
          />
        )}
      </div>
    </div>
  );
}
