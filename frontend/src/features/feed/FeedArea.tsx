import { useState } from "react";
import { Tabs, Text, Title } from "@mantine/core";
import { HistoryPanel } from "../library/HistoryPanel";
import { FeedPanel } from "./FeedPanel";

type FeedSection = "history" | "community";

export function FeedArea() {
  const [section, setSection] = useState<FeedSection>("history");

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
        onChange={(value) => setSection((value || "history") as FeedSection)}
      >
        <Tabs.List>
          <Tabs.Tab value="history">History</Tabs.Tab>
          <Tabs.Tab value="community">Community</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="history" pt="md">
          <HistoryPanel />
        </Tabs.Panel>
        <Tabs.Panel value="community" pt="md">
          <FeedPanel />
        </Tabs.Panel>
      </Tabs>
    </section>
  );
}
