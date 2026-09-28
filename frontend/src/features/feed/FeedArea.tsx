import { useState } from "react";
import { ActionIcon, Group, Tabs, Text, Title, Tooltip } from "@mantine/core";
import { IconUsers } from "@tabler/icons-react";
import { UserDirectory } from "./UserDirectory";
import { HistoryPanel } from "../library/HistoryPanel";
import { FeedPanel } from "./FeedPanel";
import type { MediaDetailTarget } from "../../types";
import { readStoredChoice, writeStoredChoice } from "../../lib/browserStorage";

type FeedSection = "history" | "community";

export function FeedArea({
  onOpenDetail,
  onOpenUser,
  userID,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
  onOpenUser?: (userID: string) => void;
  userID: string;
}) {
  const tabStorageKey = `visto:tab:${userID}:feed`;
  const [section, setSection] = useState<FeedSection>(() =>
    readStoredChoice("session", tabStorageKey, ["history", "community"] as const, "history"),
  );
  const [directoryOpen, setDirectoryOpen] = useState(false);

  return (
    <section className="activity-page" aria-label="Activity">
      <Group className="page-heading" justify="space-between" align="start" wrap="nowrap">
        <div>
          <Text className="section-kicker">Your watchroom</Text>
          <Title order={1}>Activity</Title>
          <Text c="dimmed" mt={6}>
            Your watch history and what others have shared.
          </Text>
        </div>
        <Tooltip label="Users">
          <ActionIcon
            variant="light"
            aria-label="Users"
            onClick={() => setDirectoryOpen(true)}
          >
            <IconUsers size={18} />
          </ActionIcon>
        </Tooltip>
      </Group>
      <UserDirectory
        opened={directoryOpen}
        onClose={() => setDirectoryOpen(false)}
        onOpenUser={(id) => {
          setDirectoryOpen(false);
          onOpenUser?.(id);
        }}
      />
      <Tabs
        className="section-tabs"
        value={section}
        keepMounted={false}
        onChange={(value) => {
          const next = value === "community" ? "community" : "history";
          setSection(next);
          writeStoredChoice("session", tabStorageKey, next);
        }}
      >
        <Tabs.List>
          <Tabs.Tab value="history">History</Tabs.Tab>
          <Tabs.Tab value="community">Community</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="history" pt="md">
          <HistoryPanel onOpenDetail={onOpenDetail} />
        </Tabs.Panel>
        <Tabs.Panel value="community" pt="md">
          <FeedPanel onOpenDetail={onOpenDetail} onOpenUser={onOpenUser} />
        </Tabs.Panel>
      </Tabs>
    </section>
  );
}
