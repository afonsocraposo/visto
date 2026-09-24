import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, Modal, Paper, Select, Text } from "@mantine/core";
import { useUserQueryKey } from "../auth/SessionContext";
import type { ShowEpisodeEntry } from "../../types";

type PendingSkippedEpisodes = { target: ShowEpisodeEntry; missing: ShowEpisodeEntry[] };

export function EpisodeBrowser({ showID, title, onClose }: { showID: string; title: string; onClose: () => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [season, setSeason] = useState<string | null>(null);
  const [confirmation, setConfirmation] = useState<PendingSkippedEpisodes | null>(null);
  const episodes = useQuery({
    queryKey: userQueryKey("episodes", showID),
    queryFn: async () => {
      const response = await fetch(`/api/v1/shows/${encodeURIComponent(showID)}/episodes`);
      if (!response.ok) throw new Error("Could not load episodes.");
      return response.json() as Promise<ShowEpisodeEntry[]>;
    },
  });
  const recordPlays = useMutation({
    mutationFn: async ({ episodeIDs, bulk }: { episodeIDs: string[]; bulk: boolean }) => {
      const response = await fetch(bulk ? "/api/v1/plays/bulk" : "/api/v1/plays", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(bulk ? { episode_ids: episodeIDs } : { episode_id: episodeIDs[0] }),
      });
      if (!response.ok) throw new Error("Could not record this watch.");
    },
    onSuccess: async () => {
      setConfirmation(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: userQueryKey("episodes", showID) }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      ]);
    },
  });

  const episodeEntries = episodes.data ?? [];
  const seasons = [...new Set(episodeEntries.map(entry => entry.episode.season_number))];
  const firstRegularSeason = seasons.find(number => number > 0);
  const defaultSeason = firstRegularSeason ?? seasons[0];
  const selectedSeason = season ?? (defaultSeason === undefined ? null : String(defaultSeason));
  const visibleEpisodes = episodeEntries.filter(entry => String(entry.episode.season_number) === selectedSeason);

  const selectEpisode = (target: ShowEpisodeEntry) => {
    const today = new Date().toISOString().slice(0, 10);
    const missing = episodeEntries.filter(entry => {
      const episode = entry.episode;
      const comesBefore = episode.season_number < target.episode.season_number
        || (episode.season_number === target.episode.season_number && episode.episode_number < target.episode.episode_number);
      const released = !episode.air_date || episode.air_date <= today;
      return comesBefore && episode.season_number > 0 && released && !entry.watched;
    });
    if (missing.length > 0) {
      setConfirmation({ target, missing });
      return;
    }
    recordPlays.mutate({ episodeIDs: [target.episode.id], bulk: false });
  };

  return (
    <>
      <Modal opened onClose={onClose} title={`Episodes · ${title}`} size="lg" centered>
        {episodes.isPending && <Group justify="center" py="xl"><Loader /></Group>}
        {episodes.isError && <Alert color="red">{episodes.error.message}</Alert>}
        {episodes.data && seasons.length === 0 && <Text c="dimmed">Episode details are not available yet.</Text>}
        {episodes.data && seasons.length > 0 && (
          <>
            <Select
              label="Season"
              value={selectedSeason}
              onChange={setSeason}
              data={seasons.map(number => ({ value: String(number), label: number === 0 ? "Specials" : `Season ${number}` }))}
            />
            {visibleEpisodes.map(entry => (
              <Paper key={entry.episode.id} withBorder p="sm" mt="sm">
                <Group justify="space-between">
                  <div>
                    <Text fw={600}>{`Episode ${entry.episode.episode_number}${entry.name ? ` · ${entry.name}` : ""}`}</Text>
                    <Text size="xs" c="dimmed">{entry.episode.air_date || "Air date not announced"}</Text>
                  </div>
                  {entry.watched ? (
                    <Text size="sm" c="dimmed">Watched</Text>
                  ) : (
                    <Button size="xs" loading={recordPlays.isPending} onClick={() => selectEpisode(entry)}>Mark watched</Button>
                  )}
                </Group>
              </Paper>
            ))}
            {recordPlays.isError && <Alert color="red" mt="md">{recordPlays.error.message}</Alert>}
          </>
        )}
      </Modal>
      <Modal opened={confirmation !== null} onClose={() => setConfirmation(null)} title="Skipped episodes" centered>
        <Text mb="md">There are {confirmation?.missing.length} earlier unplayed episodes of {title}. How would you like to continue?</Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={() => confirmation && recordPlays.mutate({ episodeIDs: [confirmation.target.episode.id], bulk: false })} loading={recordPlays.isPending}>
            Only this episode
          </Button>
          <Button onClick={() => confirmation && recordPlays.mutate({ episodeIDs: [...confirmation.missing.map(entry => entry.episode.id), confirmation.target.episode.id], bulk: true })} loading={recordPlays.isPending}>
            Mark all as watched
          </Button>
        </Group>
      </Modal>
    </>
  );
}
