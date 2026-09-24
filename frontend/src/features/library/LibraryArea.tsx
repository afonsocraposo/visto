import { useState } from "react";
import { Tabs } from "@mantine/core";
import { LibraryPanel } from "./Library";
import { ProfilePanel } from "./ProfilePanel";
import type { MediaDetailTarget, User } from "../../types";

type LibrarySection = "library" | "settings";

export function LibraryArea({ user, onOpenDetail }: { user: User; onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const [section, setSection] = useState<LibrarySection>("library");

  return <Tabs value={section} keepMounted={false} onChange={value => setSection((value || "library") as LibrarySection)}>
    <Tabs.List grow>
      <Tabs.Tab value="library">Library</Tabs.Tab>
      <Tabs.Tab value="settings">Settings</Tabs.Tab>
    </Tabs.List>
    <Tabs.Panel value="library" pt="md"><LibraryPanel onOpenDetail={onOpenDetail} /></Tabs.Panel>
    <Tabs.Panel value="settings" pt="md"><ProfilePanel user={user} /></Tabs.Panel>
  </Tabs>;
}
