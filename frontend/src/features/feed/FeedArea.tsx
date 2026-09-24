import { useState } from "react";
import { Tabs } from "@mantine/core";
import { HistoryPanel } from "../library/HistoryPanel";
import { FeedPanel } from "./FeedPanel";

type FeedSection = "history" | "community";

export function FeedArea() {
  const [section, setSection] = useState<FeedSection>("history");

  return <Tabs value={section} keepMounted={false} onChange={value => setSection((value || "history") as FeedSection)}>
    <Tabs.List grow>
      <Tabs.Tab value="history">Watch history</Tabs.Tab>
      <Tabs.Tab value="community">Community feed</Tabs.Tab>
    </Tabs.List>
    <Tabs.Panel value="history" pt="md"><HistoryPanel /></Tabs.Panel>
    <Tabs.Panel value="community" pt="md"><FeedPanel /></Tabs.Panel>
  </Tabs>;
}
