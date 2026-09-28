import { useState } from "react";
import { useMutation, useQuery, useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import {
  ActionIcon,
  Alert,
  Button,
  Group,
  Loader,
  Modal,
  Paper,
  Select,
  Text,
  Tooltip,
} from "@mantine/core";
import { IconEye, IconEyeCheck } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { fetchAllPages, pageURL } from "../../lib/pagination";
import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { formatActivityTime } from "../../lib/time";
import type { CursorPage } from "../../types";
import { findMissingPriorEpisodes } from "./episodeSelection";
import type { ShowEpisodeEntry } from "../../types";

type PendingSkippedEpisodes = { target: ShowEpisodeEntry; missing: ShowEpisodeEntry[] };

export function EpisodeBrowser({
  showID,
  title,
  onClose,
}: {
  showID: string;
  title: string;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [season, setSeason] = useState<string | null>(null);
  const [confirmation, setConfirmation] = useState<PendingSkippedEpisodes | null>(null);
  const [prepareError, setPrepareError] = useState<string | null>(null);
  const seasonsQuery = useQuery({
    queryKey: userQueryKey("show-seasons", showID),
    queryFn: () =>
      api.get<Array<{ id: string; season_number: number }>>(
        `/api/v1/shows/${encodeURIComponent(showID)}/seasons`,
      ),
  });
  const seasons = seasonsQuery.data?.map((item) => item.season_number) ?? [];
  const firstRegularSeason = seasons.find((number) => number > 0);
  const defaultSeason = firstRegularSeason ?? seasons[0];
  const selectedSeason = season ?? (defaultSeason === undefined ? null : String(defaultSeason));
  const selectedSeasonID = seasonsQuery.data?.find(
    (item) => String(item.season_number) === selectedSeason,
  )?.id;
  const episodes = useInfiniteQuery({
    queryKey: userQueryKey("episodes", showID, selectedSeasonID),
    enabled: Boolean(selectedSeasonID),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<ShowEpisodeEntry>>(
        pageURL(`/api/v1/seasons/${encodeURIComponent(selectedSeasonID!)}/episodes`, pageParam),
        "Could not load episodes.",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const recordPlays = useMutation({
    mutationFn: ({ episodeIDs, bulk }: { episodeIDs: string[]; bulk: boolean }) =>
      api.post(
        bulk ? "/api/v1/plays/bulk" : "/api/v1/plays",
        bulk ? { episode_ids: episodeIDs } : { episode_id: episodeIDs[0] },
        "Could not record this watch.",
      ),
    onSuccess: async () => {
      setConfirmation(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: userQueryKey("episodes", showID) }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("show-progress") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      ]);
    },
  });

  const visibleEpisodes = episodes.data?.pages.flatMap((page) => page.items) ?? [];

  const selectEpisode = async (target: ShowEpisodeEntry) => {
    setPrepareError(null);
    try {
      const allEpisodes = await fetchAllPages<ShowEpisodeEntry>(
        `/api/v1/shows/${encodeURIComponent(showID)}/episodes`,
        "Could not check earlier episodes.",
      );
      const today = new Date().toISOString().slice(0, 10);
      const missing = findMissingPriorEpisodes(allEpisodes, target, today);
      if (missing.length > 0) {
        setConfirmation({ target, missing });
        return;
      }
      recordPlays.mutate({ episodeIDs: [target.episode.id], bulk: false });
    } catch (error) {
      setPrepareError(error instanceof Error ? error.message : "Could not check earlier episodes.");
    }
  };

  return (
    <>
      <Modal opened onClose={onClose} title={`Episodes · ${title}`} size="lg" centered>
        {(episodes.isPending || seasonsQuery.isPending) && (
          <Group justify="center" py="xl">
            <Loader />
          </Group>
        )}
        {(episodes.isError || seasonsQuery.isError) && (
          <Alert color="red">Could not load episodes.</Alert>
        )}
        {seasonsQuery.data && seasons.length === 0 && (
          <Text c="dimmed">Episode details are not available yet.</Text>
        )}
        {seasonsQuery.data && seasons.length > 0 && (
          <>
            <Select
              label="Season"
              value={selectedSeason}
              onChange={setSeason}
              data={seasons.map((number) => ({
                value: String(number),
                label: number === 0 ? "Specials" : `Season ${number}`,
              }))}
            />
            {visibleEpisodes.map((entry) => (
              <Paper key={entry.episode.id} withBorder p="sm" mt="sm">
                <Group justify="space-between">
                  <div>
                    <Text
                      fw={600}
                    >{`Episode ${entry.episode.episode_number}${entry.name ? ` · ${entry.name}` : ""}`}</Text>
                    <Text size="xs" c="dimmed">
                      {entry.episode.air_date || "Air date not announced"}
                    </Text>
                  </div>
                  <Tooltip
                    label={
                      entry.watched
                        ? entry.last_watched_at
                          ? `Last watched ${formatActivityTime(entry.last_watched_at)}`
                          : "Already watched"
                        : `Mark episode ${entry.episode.episode_number} watched`
                    }
                    withArrow
                  >
                    <ActionIcon
                      size="lg"
                      variant={entry.watched ? "light" : "default"}
                      color={entry.watched ? "teal" : undefined}
                      aria-label={
                        entry.watched
                          ? `Episode ${entry.episode.episode_number} already watched`
                          : `Mark episode ${entry.episode.episode_number} watched`
                      }
                      disabled={entry.watched || recordPlays.isPending}
                      loading={!entry.watched && recordPlays.isPending}
                      onClick={() => void selectEpisode(entry)}
                    >
                      {entry.watched ? <IconEyeCheck size={18} /> : <IconEye size={18} />}
                    </ActionIcon>
                  </Tooltip>
                </Group>
              </Paper>
            ))}
            <InfiniteScrollTrigger
              hasNextPage={!!episodes.hasNextPage}
              isFetchingNextPage={episodes.isFetchingNextPage}
              isFetchNextPageError={episodes.isFetchNextPageError}
              fetchNextPage={() => void episodes.fetchNextPage()}
            />
            {recordPlays.isError && (
              <Alert color="red" mt="md">
                {recordPlays.error.message}
              </Alert>
            )}
            {prepareError && (
              <Alert color="red" mt="md">
                {prepareError}
              </Alert>
            )}
          </>
        )}
      </Modal>
      <Modal
        opened={confirmation !== null}
        onClose={() => setConfirmation(null)}
        title="Skipped episodes"
        centered
      >
        <Text mb="md">
          There are {confirmation?.missing.length} earlier unplayed episodes of {title}. How would
          you like to continue?
        </Text>
        <Group justify="flex-end">
          <Button
            variant="default"
            onClick={() =>
              confirmation &&
              recordPlays.mutate({ episodeIDs: [confirmation.target.episode.id], bulk: false })
            }
            loading={recordPlays.isPending}
          >
            Only this episode
          </Button>
          <Button
            onClick={() =>
              confirmation &&
              recordPlays.mutate({
                episodeIDs: [
                  ...confirmation.missing.map((entry) => entry.episode.id),
                  confirmation.target.episode.id,
                ],
                bulk: true,
              })
            }
            loading={recordPlays.isPending}
          >
            Mark all as watched
          </Button>
        </Group>
      </Modal>
    </>
  );
}
