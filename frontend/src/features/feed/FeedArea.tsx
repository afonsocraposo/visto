import { useState } from "react";
import { Button, Group, Title } from "@mantine/core";
import { IconUsers } from "@tabler/icons-react";
import { UserDirectory } from "./UserDirectory";
import { HistoryPanel } from "../library/HistoryPanel";
import { FeedPanel } from "./FeedPanel";
import type { MediaDetailTarget } from "../../types";
import { readStoredChoice, writeStoredChoice } from "../../lib/browserStorage";
import { SectionTabs } from "../../components/SectionTabs";

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
    readStoredChoice("local", tabStorageKey, ["history", "community"] as const, "history"),
  );
  const [directoryOpen, setDirectoryOpen] = useState(false);

  return (
    <section className="activity-page" aria-label="Activity">
      <Group className="page-heading" justify="space-between" align="center" wrap="nowrap">
        <Title order={1}>Activity</Title>
        <Button
          className="people-button"
          variant="default"
          leftSection={<IconUsers size={18} />}
          onClick={() => setDirectoryOpen(true)}
        >
          People
        </Button>
      </Group>
      <UserDirectory
        opened={directoryOpen}
        onClose={() => setDirectoryOpen(false)}
        onOpenUser={(id) => {
          setDirectoryOpen(false);
          onOpenUser?.(id);
        }}
      />
      <SectionTabs
        label="Activity views"
        value={section}
        onChange={(next) => {
          setSection(next);
          writeStoredChoice("local", tabStorageKey, next);
        }}
        options={[
          { value: "history", label: "History" },
          { value: "community", label: "Community" },
        ]}
      />
      <div key={section} className="section-panel">
        {section === "history" ? (
          <HistoryPanel onOpenDetail={onOpenDetail} groupByDay />
        ) : (
          <FeedPanel onOpenDetail={onOpenDetail} onOpenUser={onOpenUser} />
        )}
      </div>
    </section>
  );
}
