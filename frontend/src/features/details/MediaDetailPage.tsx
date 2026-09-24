import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Badge, Button, Group, Image, Loader, Paper, Select, Stack, Text, Title } from "@mantine/core";
import { IconArrowLeft, IconCheck, IconClock, IconPlayerPlay, IconPlus } from "@tabler/icons-react";
import { api } from "../../lib/api";
import { backdropURL, posterURL } from "../../lib/artwork";
import { useUserQueryKey } from "../auth/SessionContext";
import type { HistoryEntry, LibraryEntry, MediaDetailTarget, SearchMedia, ShowEpisodeEntry } from "../../types";

type Props = { target: MediaDetailTarget; onBack: () => void };

export function MediaDetailPage({ target, onBack }: Props) {
  const userQueryKey = useUserQueryKey();
  const queryClient = useQueryClient();
  const [season, setSeason] = useState<string | null>(null);
  const library = useQuery({
    queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID),
    queryFn: () => api.get<LibraryEntry>(`/api/v1/${target.mediaType === "tv" ? "shows" : "movies"}/${target.tmdbID}`, "Could not load media details."),
    retry: false,
  });
  const seed = target.seed;
  const media = library.data?.media ?? seed;
  const showID = target.mediaID ?? library.data?.item.media_id;
  const episodes = useQuery({
    queryKey: userQueryKey("detail-episodes", showID),
    enabled: target.mediaType === "tv" && Boolean(showID),
    queryFn: () => api.get<ShowEpisodeEntry[]>(`/api/v1/shows/${encodeURIComponent(showID!)}/episodes`, "Could not load episodes."),
  });
  const history = useQuery({
    queryKey: userQueryKey("detail-history"),
    enabled: Boolean(library.data),
    queryFn: () => api.get<HistoryEntry[]>("/api/v1/plays?limit=500", "Could not load watch history."),
  });
  const update = useMutation({
    mutationFn: ({ status, rating }: { status: string; rating: number | null }) => api.patch(`/api/v1/library/${encodeURIComponent(showID!)}`, { status, rating }, "Could not update this title."),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID) }),
    ]),
  });
  const add = useMutation({
    mutationFn: (status: "watching" | "watchlist") => api.post("/api/v1/library", { media, status }, "Could not add this title."),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
  });
  const markMovieWatched = useMutation({
    mutationFn: () => api.post("/api/v1/plays", { media_id: showID }, "Could not record this watch."),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID) }),
    ]),
  });
  const markEpisodeWatched = useMutation({
    mutationFn: (episodeID: string) => api.post("/api/v1/plays", { episode_id: episodeID }, "Could not record this watch."),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("detail-episodes", showID) }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });
  const unwatch = useMutation({
    mutationFn: (playID: string) => api.delete(`/api/v1/plays/${encodeURIComponent(playID)}`, "Could not mark this item unwatched."),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("detail-history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("detail-episodes", showID) }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });

  if (library.isPending && !seed) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (library.isError && !seed) return <Alert color="red" mt="md">Media details are temporarily unavailable.</Alert>;
  if (!media) return <Alert color="yellow" mt="md">This title is no longer available.</Alert>;

  const art = posterURL(media.poster_path, "w500");
  const episodeEntries = episodes.data ?? [];
  const seasons = [...new Set(episodeEntries.map(entry => entry.episode.season_number))];
  const selectedSeason = season ?? (seasons.length ? String(seasons.find(number => number > 0) ?? seasons[0]) : null);
  const visibleEpisodes = episodeEntries.filter(entry => String(entry.episode.season_number) === selectedSeason);
  const selectedEpisode = target.episodeID ? episodeEntries.find(entry => entry.episode.id === target.episodeID) ?? { episode: target.episode!, name: `Episode ${target.episode?.episode_number ?? ""}`, watched: false } : null;
  const backdrop = (selectedEpisode?.still_path ? backdropURL(selectedEpisode.still_path, "w780") : null) ?? backdropURL((media as SearchMedia & { backdrop_path?: string }).backdrop_path, "w1280") ?? art;
  const watchedPlay = history.data?.find(item => selectedEpisode ? item.play.episode_id === selectedEpisode.episode.id : item.play.media_id === showID);
  const isSaved = Boolean(library.data);
  const status = library.data?.item.status;

  return <div className="detail-page">
    <Button className="detail-back" variant="subtle" leftSection={<IconArrowLeft size={17} />} onClick={onBack}>Back</Button>
    <section className="detail-hero" style={{ backgroundImage: `linear-gradient(90deg, rgba(9,13,18,.96) 0%, rgba(9,13,18,.84) 43%, rgba(9,13,18,.35) 100%), url(${backdrop})` }}>
      <div className="detail-hero-content">
        <Badge className="watch-kind" variant="filled">{media.type === "tv" ? "TV show" : "Movie"}</Badge>
        <Title order={1}>{selectedEpisode ? selectedEpisode.name : media.title}</Title>
        <Text className="detail-subtitle">{selectedEpisode ? `${media.title} · S${String(selectedEpisode.episode.season_number).padStart(2, "0")}E${String(selectedEpisode.episode.episode_number).padStart(2, "0")}` : `${media.release_date ? media.release_date.slice(0, 4) : ""}${media.original_language ? ` · ${media.original_language.toUpperCase()}` : ""}`}</Text>
        <Text className="detail-overview">{selectedEpisode ? (selectedEpisode.overview || "Episode details are shown from your local catalog.") : media.overview || "No description is available."}</Text>
        {selectedEpisode?.runtime ? <Text className="detail-runtime" mt="sm"><IconClock size={15} /> {selectedEpisode.runtime} min</Text> : null}
        {selectedEpisode ? <EpisodeActions entry={selectedEpisode} playID={watchedPlay?.play.id} onWatch={() => markEpisodeWatched.mutate(selectedEpisode.episode.id)} onUnwatch={playID => unwatch.mutate(playID)} pending={markEpisodeWatched.isPending || unwatch.isPending} /> : <MediaActions media={media} isSaved={isSaved} status={status} add={add} update={update} watched={Boolean(watchedPlay)} playID={watchedPlay?.play.id} onWatch={() => markMovieWatched.mutate()} onUnwatch={playID => unwatch.mutate(playID)} pending={markMovieWatched.isPending || unwatch.isPending} />}
      </div>
      {art && <Image className="detail-poster" src={art} alt={`${media.title} poster`} />}
    </section>
    {target.mediaType === "tv" && !selectedEpisode && isSaved && <section className="detail-section">
      <Group justify="space-between" align="end" mb="sm"><div><Text className="section-kicker">Your catalog</Text><Title order={2}>Seasons & episodes</Title></div>{seasons.length > 0 && <Select aria-label="Season" value={selectedSeason} onChange={setSeason} data={seasons.map(number => ({ value: String(number), label: number === 0 ? "Specials" : `Season ${number}` }))} w={150} />}</Group>
      {episodes.isPending && <Group justify="center" py="lg"><Loader /></Group>}
      {episodes.isError && <Alert color="red">Episodes are temporarily unavailable.</Alert>}
      {!episodes.isPending && !episodes.isError && !visibleEpisodes.length && <Text c="dimmed">Episode details are not available yet.</Text>}
      <Stack gap="xs">{visibleEpisodes.map(entry => <Paper key={entry.episode.id} className="episode-row" withBorder p="sm"><Group justify="space-between" wrap="nowrap"><div><Text fw={650}>{`Episode ${entry.episode.episode_number}${entry.name ? ` · ${entry.name}` : ""}`}</Text><Text size="xs" c="dimmed">{entry.episode.air_date || "Air date not announced"}</Text></div>{entry.watched ? <Badge color="teal" variant="light" leftSection={<IconCheck size={13} />}>Watched</Badge> : <Button size="xs" variant="light" leftSection={<IconPlayerPlay size={14} />} loading={markEpisodeWatched.isPending} onClick={() => markEpisodeWatched.mutate(entry.episode.id)}>Mark watched</Button>}</Group></Paper>)}</Stack>
    </section>}
    {library.isError && seed && <Text className="detail-hint" c="dimmed">Add this title to your library to track episodes and progress.</Text>}
  </div>;
}

function EpisodeActions({ entry, playID, onWatch, onUnwatch, pending }: { entry: ShowEpisodeEntry; playID?: string; onWatch: () => void; onUnwatch: (playID: string) => void; pending: boolean }) {
  return <Group className="detail-actions" mt="lg"><Badge color={entry.watched ? "teal" : "yellow"} variant="light">{entry.watched ? "Watched" : "Not watched"}</Badge>{entry.watched && playID ? <Button variant="default" leftSection={<IconCheck size={16} />} loading={pending} onClick={() => onUnwatch(playID)}>Mark unwatched</Button> : !entry.watched && <Button leftSection={<IconCheck size={16} />} loading={pending} onClick={onWatch}>Mark watched</Button>}</Group>;
}

type ActionMutation<T> = { isPending: boolean; mutate: (value: T) => void };
function MediaActions({ media, isSaved, status, add, update, watched, playID, onWatch, onUnwatch, pending }: { media: SearchMedia; isSaved: boolean; status?: string; add: ActionMutation<"watching" | "watchlist">; update: ActionMutation<{ status: string; rating: number | null }>; watched: boolean; playID?: string; onWatch: () => void; onUnwatch: (playID: string) => void; pending: boolean }) {
  if (!isSaved) return <Group className="detail-actions" mt="lg"><Button leftSection={<IconPlus size={16} />} loading={add.isPending} onClick={() => add.mutate(media.type === "tv" ? "watching" : "watchlist")}>{media.type === "tv" ? "Add to watching" : "Add to watchlist"}</Button>{media.type === "tv" && <Button variant="default" loading={add.isPending} onClick={() => add.mutate("watchlist")}>Watch later</Button>}</Group>;
  return <Group className="detail-actions" mt="lg"><Select aria-label="Current list" value={status} onChange={value => value && update.mutate({ status: value, rating: null })} data={[{ value: "watchlist", label: "Watchlist" }, { value: "watching", label: "Watching" }, { value: "paused", label: "Paused" }, { value: "dropped", label: "Dropped" }]} w={150} />{media.type === "movie" && (watched && playID ? <Button variant="default" leftSection={<IconCheck size={16} />} loading={pending} onClick={() => onUnwatch(playID)}>Mark unwatched</Button> : <Button variant="light" leftSection={<IconCheck size={16} />} loading={pending} onClick={onWatch}>Mark watched</Button>)}</Group>;
}
