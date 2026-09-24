import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Badge, Button, Group, Image, Paper, Select, Stack, Text } from "@mantine/core";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { EpisodeBrowser } from "./EpisodeBrowser";
import { hasCaughtUpDisplayState } from "./showStatus";
import type { LibraryEntry, MediaDetailTarget, ShowProgress } from "../../types";
import { posterURL } from "../../lib/artwork";

export function LibraryCard({ entry, onOpenDetail }: { entry: LibraryEntry; onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [episodesOpen, setEpisodesOpen] = useState(false);
  const progress = useQuery({
    queryKey: userQueryKey("show-progress", entry.item.media_id),
    enabled: entry.media.type === "tv" && entry.item.status === "watching",
    queryFn: () => api.get<ShowProgress>(`/api/v1/shows/${encodeURIComponent(entry.item.media_id)}/progress`, "Could not load show progress."),
  });
  const update = useMutation({
    mutationFn: ({ status, rating }: { status: string; rating: number | null }) => api.patch(`/api/v1/library/${encodeURIComponent(entry.item.media_id)}`, { status, rating }, "Could not update this title."),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });
  const watched = useMutation({
    mutationFn: () => api.post("/api/v1/plays", { media_id: entry.item.media_id }, "Could not record this watch."), onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });

  const art = posterURL(entry.media.poster_path, "w342");
  return <Paper className="library-card" withBorder p="md" mt="sm">
    <Group align="flex-start" wrap="nowrap" gap="md">
      <div className="library-poster library-clickable" role={onOpenDetail ? "button" : undefined} tabIndex={onOpenDetail ? 0 : undefined} onClick={() => onOpenDetail?.({ mediaType: entry.media.type, tmdbID: entry.media.tmdb_id, mediaID: entry.item.media_id })}>
        {art ? <Image src={art} alt={`${entry.media.title} poster`} /> : <div className="artwork-fallback">{entry.media.title.slice(0, 1)}</div>}
      </div>
      <div className="library-card-body">
        <Group justify="space-between" align="flex-start" gap="xs">
          <div>
            <Text className="library-title library-clickable" fw={750} role={onOpenDetail ? "link" : undefined} onClick={() => onOpenDetail?.({ mediaType: entry.media.type, tmdbID: entry.media.tmdb_id, mediaID: entry.item.media_id })}>{entry.media.title}</Text>
            <Text size="xs" c="dimmed">{entry.media.type === "tv" ? "TV show" : "Movie"}{entry.media.release_date ? ` · ${entry.media.release_date.slice(0, 4)}` : ""}</Text>
          </div>
          <Group gap={4}>
            {entry.completed && <Badge color="teal" variant="light">Completed</Badge>}
            {entry.media.type === "tv" && hasCaughtUpDisplayState(progress.data) && <Badge color="blue" variant="light">Caught up</Badge>}
          </Group>
        </Group>
        <Stack className="library-controls" mt="md" gap="xs">
          <Select aria-label={`Status for ${entry.media.title}`} value={entry.item.status} onChange={status => {
        if (status) update.mutate({ status, rating: entry.item.rating });
      }} data={[
        { value: "watchlist", label: "Watchlist" }, { value: "watching", label: "Watching" },
        { value: "paused", label: "Paused" }, { value: "dropped", label: "Dropped" },
      ]} />
          <Select aria-label={`Rating for ${entry.media.title}`} value={entry.item.rating ? String(entry.item.rating) : "none"} onChange={value => {
        update.mutate({ status: entry.item.status, rating: value && value !== "none" ? Number(value) : null });
      }} data={[
        { value: "none", label: "Not rated" },
        ...[1, 2, 3, 4, 5].map(value => ({ value: String(value), label: `${value} / 5 stars` })),
          ]} />
        </Stack>
        {entry.media.type === "tv" && <Button mt="sm" size="xs" variant="default" onClick={() => setEpisodesOpen(true)}>Browse episodes</Button>}
        {entry.media.type === "movie" && <Button mt="sm" size="xs" loading={watched.isPending} onClick={() => watched.mutate()}>{entry.completed ? "Rewatch" : "Watched"}</Button>}
        {update.isError && <Alert color="red" mt="sm">{update.error.message}</Alert>}
        {watched.isError && <Alert color="red" mt="sm">{watched.error.message}</Alert>}
      </div>
    </Group>
    {episodesOpen && <EpisodeBrowser showID={entry.item.media_id} title={entry.media.title} onClose={() => setEpisodesOpen(false)} />}
  </Paper>;
}
