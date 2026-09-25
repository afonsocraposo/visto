import { useState } from "react";
import { Tabs } from "@mantine/core";
import { LibraryPanel } from "./Library";
import { ProfilePanel } from "./ProfilePanel";
import { AdminPanel } from "./AdminPanel";
import type { LibraryStatus, MediaDetailTarget, Theme, User } from "../../types";

type LibrarySection = "library" | "settings" | "admin";

export function LibraryArea({
  user,
  theme,
  onThemeChange,
  onSignOut,
  signingOut,
  signOutError,
  onOpenDetail,
  onOpenList,
}: {
  user: User;
  theme: Theme;
  onThemeChange: (theme: Theme) => void;
  onSignOut: () => void;
  signingOut: boolean;
  signOutError?: string;
  onOpenDetail?: (target: MediaDetailTarget) => void;
  onOpenList?: (status: LibraryStatus) => void;
}) {
  const [section, setSection] = useState<LibrarySection>("library");

  return (
    <Tabs
      className="section-tabs"
      value={section}
      keepMounted={false}
      onChange={(value) => setSection((value || "library") as LibrarySection)}
    >
      <Tabs.List>
        <Tabs.Tab value="library">Library</Tabs.Tab>
        <Tabs.Tab value="settings">Settings</Tabs.Tab>
        {user.role === "admin" && <Tabs.Tab value="admin">Admin</Tabs.Tab>}
      </Tabs.List>
      <Tabs.Panel value="library" pt="md">
        <LibraryPanel onOpenDetail={onOpenDetail} onOpenList={onOpenList} />
      </Tabs.Panel>
      <Tabs.Panel value="settings" pt="md">
        <ProfilePanel
          theme={theme}
          onThemeChange={onThemeChange}
          onSignOut={onSignOut}
          signingOut={signingOut}
          signOutError={signOutError}
        />
      </Tabs.Panel>
      {user.role === "admin" && (
        <Tabs.Panel value="admin" pt="md">
          <AdminPanel currentUser={user} />
        </Tabs.Panel>
      )}
    </Tabs>
  );
}
