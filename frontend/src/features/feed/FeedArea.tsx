import { useState } from "react";
import { Tabs, Text, Title } from "@mantine/core";
import { HistoryPanel } from "../library/HistoryPanel";
import { FeedPanel } from "./FeedPanel";
import type { MediaDetailTarget } from "../../types";
import { readStoredChoice, writeStoredChoice } from "../../lib/browserStorage";

type FeedSection = "history" | "community";

export function FeedArea({
  onOpenDetail,
  userID,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
  userID: string;
}) {
  const tabStorageKey = `visto:tab:${userID}:feed`;
  const [section, setSection] = useState<FeedSection>(() =>
    readStoredChoice("session", tabStorageKey, ["history", "community"] as const, "history"),
  );

  return (
    <section className="activity-page" aria-label="Activity">
      <div className="page-heading">
        <Text className="section-kicker">Your watchroom</Text>
        <Title order={1}>Activity</Title>
        <Text c="dimmed" mt={6}>
          Your watch history and what others have shared.
        </Text>
      </div>
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
          <FeedPanel onOpenDetail={onOpenDetail} />
        </Tabs.Panel>
      </Tabs>
    </section>
  );
}
