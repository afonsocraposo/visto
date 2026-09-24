import { useState } from "react";
import { Tabs } from "@mantine/core";
import { HistoryPanel } from "./HistoryPanel";
import { LibraryPanel } from "./Library";
import { ProfilePanel } from "./ProfilePanel";
import type { MediaDetailTarget, User } from "../../types";

type LibrarySection = "library" | "history" | "profile";

export function LibraryArea({ user, onOpenDetail }: { user: User; onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const [section, setSection] = useState<LibrarySection>("library");

  return <Tabs value={section} keepMounted={false} onChange={value => setSection((value || "library") as LibrarySection)}>
    <Tabs.List grow>
      <Tabs.Tab value="library">Library</Tabs.Tab>
      <Tabs.Tab value="history">Watch history</Tabs.Tab>
      <Tabs.Tab value="profile">Profile & settings</Tabs.Tab>
    </Tabs.List>
    <Tabs.Panel value="library" pt="md"><LibraryPanel onOpenDetail={onOpenDetail} /></Tabs.Panel>
    <Tabs.Panel value="history" pt="md"><HistoryPanel /></Tabs.Panel>
    <Tabs.Panel value="profile" pt="md"><ProfilePanel user={user} /></Tabs.Panel>
  </Tabs>;
}
