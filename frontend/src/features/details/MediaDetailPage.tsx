import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ActionIcon, Alert, Avatar, Badge, Button, Group, Image, Loader, Modal, Paper, Select, SimpleGrid, Stack, Switch, Text, Title, Tooltip } from "@mantine/core";
import { IconArrowLeft, IconCheck, IconClock, IconEye, IconEyeCheck } from "@tabler/icons-react";
import { api } from "../../lib/api";
import { backdropURL, posterURL } from "../../lib/artwork";
import { useUserQueryKey } from "../auth/SessionContext";
import { findMissingPriorEpisodes } from "../library/episodeSelection";
import { RatingStars } from "../../components/RatingStars";
import { MediaQuickActions } from "../../components/MediaQuickActions";
import { selectUnwatchedEpisodes } from "./watchSelection";
import type { EpisodeRating, HistoryEntry, LibraryEntry, MediaDetailTarget, SearchMedia, ShowEpisodeEntry, TemporaryEpisodeDetails, TemporaryMovieDetails, TemporaryShowDetails } from "../../types";

type Props = { target: MediaDetailTarget; onBack: () => void; onOpenDetail: (target: MediaDetailTarget) => void };
type PendingWatch = { target: ShowEpisodeEntry | null; episodes: ShowEpisodeEntry[] };

export function MediaDetailPage({ target, onBack, onOpenDetail }: Props) {
  const userQueryKey = useUserQueryKey();
  const queryClient = useQueryClient();
  const [season, setSeason] = useState<string | null>(null);
  const [pendingWatch, setPendingWatch] = useState<PendingWatch | null>(null);
  const [showWatchModal, setShowWatchModal] = useState(false);
  const [selectedShowSeasons, setSelectedShowSeasons] = useState<Record<number, boolean>>({});
  const library = useQuery({
    queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID),
    queryFn: () => api.get<LibraryEntry>(`/api/v1/${target.mediaType === "tv" ? "shows" : "movies"}/${target.tmdbID}`, "Could not load media details."),
    retry: false,
  });
  const temporary = useQuery({
    queryKey: userQueryKey("temporary-show-detail", target.tmdbID),
    enabled: target.mediaType === "tv",
    queryFn: () => api.get<TemporaryShowDetails>(`/api/v1/discover/shows/${target.tmdbID}`, "Could not load TV show details."),
  });
  const movieDetails = useQuery({
    queryKey: userQueryKey("temporary-movie-detail", target.tmdbID),
    enabled: target.mediaType === "movie",
    queryFn: () => api.get<TemporaryMovieDetails>(`/api/v1/discover/movies/${target.tmdbID}`, "Could not load movie details."),
  });
  const seed = target.seed;
  const media = library.data?.media
    ? { ...library.data.media, backdrop_path: library.data.media.backdrop_path ?? temporary.data?.media.backdrop_path }
    : target.mediaType === "movie" ? movieDetails.data?.media ?? seed : temporary.data?.media ?? seed;
  const savedMediaID = library.data?.item.media_id;
  const showID = target.mediaID ?? savedMediaID ?? (media ? `${media.type}:${media.tmdb_id}` : undefined);
  const episodes = useQuery({
    queryKey: userQueryKey("detail-episodes", showID),
    enabled: target.mediaType === "tv" && Boolean(savedMediaID),
    queryFn: () => api.get<ShowEpisodeEntry[]>(`/api/v1/shows/${encodeURIComponent(showID!)}/episodes`, "Could not load episodes."),
  });
  const history = useQuery({
    queryKey: userQueryKey("detail-history"),
    enabled: Boolean(library.data),
    queryFn: () => api.get<HistoryEntry[]>("/api/v1/plays?limit=500", "Could not load watch history."),
  });
  const ensureTrackedEpisodes = async (): Promise<ShowEpisodeEntry[]> => {
    if (!media || !showID) throw new Error("This show is not available.");
    if (!savedMediaID) {
      await api.post("/api/v1/library", { media, status: "watching" }, "Could not add this show to Watching.");
    } else if (library.data?.item.status === "watchlist") {
      await api.patch(`/api/v1/library/${encodeURIComponent(showID)}`, { status: "watching", rating: library.data.item.rating }, "Could not move this show to Watching.");
    }
    const all = await api.get<ShowEpisodeEntry[]>(`/api/v1/shows/${encodeURIComponent(showID)}/episodes`, "Could not load episodes after adding this show.");
    queryClient.setQueryData(userQueryKey("detail-episodes", showID), all);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID) }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
    ]);
    return all;
  };
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
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID) }),
    ]),
  });
  const markMovieWatched = useMutation({
    mutationFn: async () => {
      if (!savedMediaID) await api.post("/api/v1/library", { media, status: "watching" }, "Could not add this title.");
      return api.post("/api/v1/plays", { media_id: showID }, "Could not record this watch.");
    },
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID) }),
    ]),
  });
  const prepareEpisodeWatch = useMutation({
    mutationFn: async (entry: ShowEpisodeEntry) => {
      const all = await ensureTrackedEpisodes();
      const tracked = all.find(candidate => candidate.episode.id === entry.episode.id);
      if (!tracked) throw new Error("This episode is not available in the show catalog.");
      return { tracked, missing: findMissingPriorEpisodes(all, tracked, new Date().toISOString().slice(0, 10)) };
    },
    onSuccess: ({ tracked, missing }) => {
      if (tracked.watched) return;
      if (missing.length) setPendingWatch({ target: tracked, episodes: missing });
      else markEpisodeWatched.mutate(tracked.episode.id);
    },
  });
  const markEpisodeWatched = useMutation({
    mutationFn: (episodeID: string) => api.post("/api/v1/plays", { episode_id: episodeID }, "Could not record this watch."),
    onSuccess: async () => { setPendingWatch(null); setShowWatchModal(false); return Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("detail-episodes", showID) }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("detail-history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
    ]); },
  });
  const markEpisodesWatched = useMutation({
    mutationFn: async ({ episodeIDs, selectedSeasons }: { episodeIDs?: string[]; selectedSeasons?: number[] }) => {
      const all = await ensureTrackedEpisodes();
      const today = new Date().toISOString().slice(0, 10);
      const selectedIDs = selectedSeasons
        ? selectUnwatchedEpisodes(all, selectedSeasons, today)
        : (episodeIDs ?? []);
      for (const batch of chunk([...new Set(selectedIDs)], 100)) {
        await api.post("/api/v1/plays/bulk", { episode_ids: batch }, "Could not record these watches.");
      }
    },
    onSuccess: async () => { setPendingWatch(null); setShowWatchModal(false); return Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("detail-episodes", showID) }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("detail-history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
    ]); },
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

  const isSaved = Boolean(savedMediaID);
  const availableSeasonNumbers = isSaved
    ? [...new Set((episodes.data ?? []).map(entry => entry.episode.season_number))]
    : (temporary.data?.seasons.map(season => season.season_number) ?? []);
  const selectedSeason = season ?? (target.seasonNumber !== undefined ? String(target.seasonNumber) : availableSeasonNumbers.length ? String(availableSeasonNumbers.find(number => number > 0) ?? availableSeasonNumbers[0]) : null);
  const temporarySeason = Number(selectedSeason);
  const temporaryEpisodes = useQuery({
    queryKey: userQueryKey("temporary-season-episodes", target.tmdbID, temporarySeason),
    enabled: target.mediaType === "tv" && !isSaved && Boolean(selectedSeason) && Number.isInteger(temporarySeason) && temporarySeason >= 0,
    queryFn: () => api.get<{ episodes: ShowEpisodeEntry[] }>(`/api/v1/discover/shows/${target.tmdbID}/seasons/${temporarySeason}`, "Could not load season episodes."),
  });
  const candidateEpisodes = isSaved ? (episodes.data ?? []) : (temporaryEpisodes.data?.episodes ?? []);
  const candidateEpisode = target.episodeID ? candidateEpisodes.find(entry => entry.episode.id === target.episodeID) : null;
  const episodeSeasonNumber = candidateEpisode?.episode.season_number ?? target.seasonNumber;
  const episodeNumber = candidateEpisode?.episode.episode_number ?? target.episode?.episode_number;
  const episodeDetails = useQuery({
    queryKey: userQueryKey("temporary-episode-detail", target.tmdbID, episodeSeasonNumber, episodeNumber),
    enabled: target.mediaType === "tv" && Boolean(target.episodeID) && episodeSeasonNumber !== undefined && episodeNumber !== undefined,
    queryFn: () => api.get<TemporaryEpisodeDetails>(`/api/v1/discover/shows/${target.tmdbID}/seasons/${episodeSeasonNumber}/episodes/${episodeNumber}`, "Could not load episode details."),
  });
  const episodeRating = useQuery({
    queryKey: userQueryKey("episode-rating", target.episodeID),
    enabled: Boolean(target.episodeID),
    queryFn: () => api.get<EpisodeRating>(`/api/v1/episodes/${encodeURIComponent(target.episodeID!)}/rating`, "Could not load episode rating."),
  });
  const rateEpisode = useMutation({
    mutationFn: (rating: number | null) => api.put<EpisodeRating>(`/api/v1/episodes/${encodeURIComponent(target.episodeID!)}/rating`, { rating }, "Could not save episode rating."),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("episode-rating", target.episodeID) }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });

  if (library.isPending && !seed) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (library.isError && !seed) return <Alert color="yellow" mt="md">This title is not in your library. Open it from Discover or add it to your library first.</Alert>;
  if (!media) return <Alert color="yellow" mt="md">This title is no longer available.</Alert>;

  const art = posterURL(media.poster_path, "w500");
  const temporaryEpisodeEntries = temporaryEpisodes.data?.episodes ?? [];
  const episodeEntries = isSaved ? (episodes.data ?? []) : temporaryEpisodeEntries;
  const seasons = availableSeasonNumbers;
  const visibleEpisodes = episodeEntries.filter(entry => String(entry.episode.season_number) === selectedSeason);
  const selectedEpisode = target.episodeID ? episodeEntries.find(entry => entry.episode.id === target.episodeID) ?? (target.episode ? { episode: target.episode, name: `Episode ${target.episode.episode_number}`, watched: false } : null) : null;
  const backdrop = (selectedEpisode?.still_path ? backdropURL(selectedEpisode.still_path, "w780") : null) ?? backdropURL(media.backdrop_path, "w1280") ?? art;
  const watchedPlay = history.data?.find(item => selectedEpisode ? item.play.episode_id === selectedEpisode.episode.id : item.play.media_id === showID);
  const status = library.data?.item.status;
  const today = new Date().toISOString().slice(0, 10);
  const releasedSeasonEpisodes = visibleEpisodes.filter(entry => !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today));
  const seasonGroups = [...new Set(isSaved ? episodeEntries.map(entry => entry.episode.season_number) : temporary.data?.seasons.map(season => season.season_number) ?? [])].sort((a, b) => a - b).map(number => ({ number, episodes: episodeEntries.filter(entry => entry.episode.season_number === number) }));
  const releasedShowEpisodes = episodeEntries.filter(entry => !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today));
  const selectedShowEpisodes = releasedShowEpisodes.filter(entry => selectedShowSeasons[entry.episode.season_number] === true);
  const openShowWatchModal = () => {
    setSelectedShowSeasons(Object.fromEntries(seasonGroups.map(group => [group.number, group.number > 0])));
    setShowWatchModal(true);
  };
  const requestEpisodeWatch = (entry: ShowEpisodeEntry) => prepareEpisodeWatch.mutate(entry);
  const confirmWatch = (includeTarget: boolean) => {
    if (!pendingWatch) return;
    const entries = includeTarget && pendingWatch.target ? [...pendingWatch.episodes, pendingWatch.target] : pendingWatch.episodes;
    markEpisodesWatched.mutate({ episodeIDs: entries.map(entry => entry.episode.id) });
  };

  return <div className="detail-page">
    <Modal opened={pendingWatch !== null} onClose={() => setPendingWatch(null)} title={pendingWatch?.target ? "Skipped episodes" : "Mark season watched"} centered>
      {pendingWatch?.target ? <Text mb="md">There are {pendingWatch.episodes.length} earlier unwatched episodes. How would you like to continue?</Text> : <Text mb="md">Mark {pendingWatch?.episodes.length ?? 0} released episodes in this season as watched?</Text>}
      {markEpisodesWatched.isError && <Alert color="red" mb="md">{markEpisodesWatched.error.message}</Alert>}
      {markEpisodeWatched.isError && <Alert color="red" mb="md">{markEpisodeWatched.error.message}</Alert>}
      <Group justify="flex-end">
        {pendingWatch?.target && <Button variant="default" onClick={() => markEpisodeWatched.mutate(pendingWatch.target!.episode.id)} loading={markEpisodeWatched.isPending || markEpisodesWatched.isPending}>Only this episode</Button>}
        <Button onClick={() => confirmWatch(Boolean(pendingWatch?.target))} loading={markEpisodeWatched.isPending || markEpisodesWatched.isPending}>{pendingWatch?.target ? "Mark all as watched" : "Mark season watched"}</Button>
      </Group>
    </Modal>
    <Modal opened={showWatchModal} onClose={() => setShowWatchModal(false)} title={`Mark ${media.title} watched`} centered>
      <Text size="sm" c="dimmed" mb="md">Choose the seasons to include. Specials are off by default.</Text>
      <Stack gap="xs">
        {seasonGroups.map(group => {
          const remaining = group.episodes.filter(entry => !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today)).length;
          return <Paper key={group.number} withBorder p="sm"><Group justify="space-between"><div><Text fw={650}>{group.number === 0 ? "Specials" : `Season ${group.number}`}</Text><Text size="xs" c="dimmed">{isSaved ? `${remaining} episodes remaining` : "Episodes load when you confirm"}</Text></div><Switch aria-label={`Include ${group.number === 0 ? "specials" : `season ${group.number}`}`} checked={selectedShowSeasons[group.number] === true} onChange={event => setSelectedShowSeasons(current => ({ ...current, [group.number]: event.currentTarget.checked }))} /></Group></Paper>;
        })}
      </Stack>
      {markEpisodesWatched.isError && <Alert color="red" mt="md">{markEpisodesWatched.error.message}</Alert>}
      <Group justify="flex-end" mt="lg"><Button variant="default" onClick={() => setShowWatchModal(false)}>Cancel</Button><Button disabled={!Object.values(selectedShowSeasons).some(Boolean) || (isSaved && selectedShowEpisodes.length === 0)} loading={markEpisodesWatched.isPending} onClick={() => markEpisodesWatched.mutate({ selectedSeasons: Object.entries(selectedShowSeasons).filter(([, selected]) => selected).map(([number]) => Number(number)) })}>{isSaved ? `Mark ${selectedShowEpisodes.length} episodes watched` : "Mark selected seasons watched"}</Button></Group>
    </Modal>
    <Button className="detail-back" variant="subtle" leftSection={<IconArrowLeft size={17} />} onClick={onBack}>Back</Button>
    <section className="detail-hero" style={{ backgroundImage: `linear-gradient(90deg, rgba(9,13,18,.96) 0%, rgba(9,13,18,.84) 43%, rgba(9,13,18,.35) 100%), url(${backdrop})` }}>
      <div className="detail-hero-content">
        <Badge className="watch-kind" variant="filled">{media.type === "tv" ? "TV show" : "Movie"}</Badge>
        <Title order={1}>{selectedEpisode ? selectedEpisode.name : media.title}</Title>
        {selectedEpisode ? <Button className="detail-season-link" variant="subtle" onClick={() => onOpenDetail({ mediaType: "tv", tmdbID: media.tmdb_id, mediaID: showID, seasonNumber: selectedEpisode.episode.season_number })}>{`${media.title} · Season ${selectedEpisode.episode.season_number}`}</Button> : <Text className="detail-subtitle">{`${media.release_date ? media.release_date.slice(0, 4) : ""}${media.original_language ? ` · ${media.original_language.toUpperCase()}` : ""}`}</Text>}
        <Text className="detail-overview">{selectedEpisode ? (episodeDetails.data?.overview || selectedEpisode.overview || "Episode details are shown from your local catalog.") : media.overview || "No description is available."}</Text>
        {(selectedEpisode ? (episodeDetails.data?.runtime || selectedEpisode.runtime) : movieDetails.data?.runtime) ? <Text className="detail-runtime" mt="sm"><IconClock size={15} /> {selectedEpisode ? (episodeDetails.data?.runtime || selectedEpisode.runtime) : movieDetails.data?.runtime} min</Text> : null}
        {selectedEpisode && episodeDetails.data?.vote_average ? <Text className="detail-runtime" mt="sm">TMDB {episodeDetails.data.vote_average.toFixed(1)} / 10</Text> : null}
        {selectedEpisode && episodeDetails.data?.air_date ? <Text className="detail-runtime" mt="sm">Aired {episodeDetails.data.air_date}{episodeDetails.data.production_code ? ` · ${episodeDetails.data.production_code}` : ""}</Text> : null}
        {selectedEpisode ? <EpisodeActions entry={selectedEpisode} canRate={isSaved} rating={episodeRating.data?.rating} onRate={rating => rateEpisode.mutate(rating)} playID={watchedPlay?.play.id} onWatch={() => requestEpisodeWatch(selectedEpisode)} onUnwatch={playID => unwatch.mutate(playID)} pending={prepareEpisodeWatch.isPending || markEpisodeWatched.isPending || unwatch.isPending || rateEpisode.isPending} /> : <MediaActions media={media} isSaved={isSaved} status={status} rating={library.data?.item.rating} add={add} update={update} watched={Boolean(watchedPlay)} playID={watchedPlay?.play.id} onWatch={() => markMovieWatched.mutate()} onUnwatch={playID => unwatch.mutate(playID)} pending={markMovieWatched.isPending || unwatch.isPending} />}
      </div>
      {art && <Image className="detail-poster" src={art} alt={`${media.title} poster`} />}
    </section>
    {selectedEpisode && <Group className="detail-actions" mt="md" gap="xs">
      {releasedSeasonEpisodes.length > 0 && <Button size="xs" variant="light" onClick={() => setPendingWatch({ target: null, episodes: releasedSeasonEpisodes })}>Mark season watched</Button>}
      {seasonGroups.length > 0 && (!isSaved || releasedShowEpisodes.length > 0) && <Button size="xs" variant="light" onClick={openShowWatchModal}>Mark show watched</Button>}
    </Group>}
    {selectedEpisode && prepareEpisodeWatch.isError && <Alert color="red" mt="sm">{prepareEpisodeWatch.error.message}</Alert>}
    {selectedEpisode && markEpisodeWatched.isError && <Alert color="red" mt="sm">{markEpisodeWatched.error.message}</Alert>}
    {selectedEpisode && markEpisodesWatched.isError && !pendingWatch && !showWatchModal && <Alert color="red" mt="sm">{markEpisodesWatched.error.message}</Alert>}
    {target.mediaType === "tv" && !selectedEpisode && <section className="detail-section">
      <Group justify="space-between" align="end" mb="sm"><div><Text className="section-kicker">{isSaved ? "Your catalog" : "From TMDB"}</Text><Title order={2}>Seasons & episodes</Title></div><Group gap="xs">{seasonGroups.length > 0 && (!isSaved || releasedShowEpisodes.length > 0) && <Button size="xs" onClick={openShowWatchModal}>Mark show watched</Button>}{releasedSeasonEpisodes.length > 0 && <Button size="xs" variant="light" onClick={() => setPendingWatch({ target: null, episodes: releasedSeasonEpisodes })}>Mark season watched</Button>}{seasons.length > 0 && <Select aria-label="Season" value={selectedSeason} onChange={setSeason} data={seasons.map(number => ({ value: String(number), label: number === 0 ? "Specials" : `Season ${number}` }))} w={150} />}</Group></Group>
      {((isSaved && episodes.isPending) || (!isSaved && (temporary.isPending || temporaryEpisodes.isPending))) && <Group justify="center" py="lg"><Loader /></Group>}
      {episodes.isError && isSaved && <Alert color="red">Episodes are temporarily unavailable.</Alert>}
      {temporary.isError && !isSaved && <Alert color="red">TV details are temporarily unavailable.</Alert>}
      {temporaryEpisodes.isError && !isSaved && <Alert color="red">Season episodes are temporarily unavailable.</Alert>}
      {markEpisodesWatched.isError && <Alert color="red">{markEpisodesWatched.error.message}</Alert>}
      {prepareEpisodeWatch.isError && <Alert color="red">{prepareEpisodeWatch.error.message}</Alert>}
      {markEpisodeWatched.isError && <Alert color="red">{markEpisodeWatched.error.message}</Alert>}
      {add.isError && <Alert color="red">{add.error.message}</Alert>}
      {!episodes.isPending && !temporary.isPending && !temporaryEpisodes.isPending && !episodes.isError && !temporary.isError && !temporaryEpisodes.isError && !visibleEpisodes.length && <Text c="dimmed">Episode details are not available yet.</Text>}
      <Stack gap="xs">{visibleEpisodes.map(entry => <Paper key={entry.episode.id} className="episode-row" withBorder p="sm" role="button" tabIndex={0} onClick={() => onOpenDetail({ mediaType: "tv", tmdbID: media.tmdb_id, mediaID: showID, episodeID: entry.episode.id, episode: entry.episode, seasonNumber: entry.episode.season_number })} onKeyDown={event => { if ((event.key === "Enter" || event.key === " ") && event.target === event.currentTarget) { event.preventDefault(); onOpenDetail({ mediaType: "tv", tmdbID: media.tmdb_id, mediaID: showID, episodeID: entry.episode.id, episode: entry.episode, seasonNumber: entry.episode.season_number }); } }}><Group justify="space-between" wrap="nowrap" align="flex-start"><Group wrap="nowrap" gap="sm" align="flex-start"><div className="episode-art">{entry.still_path ? <Image src={backdropURL(entry.still_path, "w780")!} alt="" /> : <div className="artwork-fallback">{entry.episode.episode_number}</div>}</div><div><Text fw={650}>{`Episode ${entry.episode.episode_number}${entry.name ? ` · ${entry.name}` : ""}`}</Text><Text size="xs" c="dimmed">{entry.episode.air_date || "Air date not announced"}</Text>{entry.overview && <Text className="episode-description" size="sm" c="dimmed" mt={5}>{entry.overview}</Text>}</div></Group><Tooltip label={entry.watched ? "Already watched" : `Mark episode ${entry.episode.episode_number} watched`} withArrow><ActionIcon aria-label={entry.watched ? `Episode ${entry.episode.episode_number} already watched` : `Mark episode ${entry.episode.episode_number} watched`} size="lg" variant={entry.watched ? "light" : "default"} color={entry.watched ? "teal" : undefined} disabled={entry.watched || prepareEpisodeWatch.isPending || markEpisodeWatched.isPending} loading={!entry.watched && prepareEpisodeWatch.isPending} onClick={event => { event.stopPropagation(); requestEpisodeWatch(entry); }}>{entry.watched ? <IconEyeCheck size={18} /> : <IconEye size={18} />}</ActionIcon></Tooltip></Group></Paper>)}</Stack>
    </section>}
    {target.mediaType === "movie" && !selectedEpisode && movieDetails.data?.genres?.length ? <section className="detail-section"><Text className="section-kicker">About this film</Text><Group gap="xs">{movieDetails.data.genres.map(genre => <Badge key={genre} variant="light">{genre}</Badge>)}{movieDetails.data.vote_average ? <Badge variant="light">TMDB {movieDetails.data.vote_average.toFixed(1)} / 10</Badge> : null}</Group></section> : null}
    {!selectedEpisode && ((target.mediaType === "tv" ? temporary.data?.cast : movieDetails.data?.cast)?.length ?? 0) > 0 && <section className="detail-section"><Text className="section-kicker">People</Text><Title order={2} mb="sm">Cast</Title><SimpleGrid cols={{ base: 2, sm: 4, md: 6 }} spacing="sm">{(target.mediaType === "tv" ? temporary.data!.cast : movieDetails.data!.cast).slice(0, 12).map(member => <Paper key={`${member.id}-${member.character}`} className="cast-card" withBorder p="sm"><Avatar src={member.profile_path ? posterURL(member.profile_path, "w185") : undefined} radius="xl" size="lg" mb="xs">{member.name.slice(0, 1)}</Avatar><Text fw={650} size="sm" lineClamp={2}>{member.name}</Text><Text size="xs" c="dimmed" lineClamp={2}>{member.character || "Cast"}</Text></Paper>)}</SimpleGrid></section>}
    {selectedEpisode && episodeDetails.data?.guest_stars?.length ? <section className="detail-section"><Text className="section-kicker">Episode guest stars</Text><SimpleGrid cols={{ base: 2, sm: 4, md: 6 }} spacing="sm">{episodeDetails.data.guest_stars.slice(0, 12).map(member => <Paper key={`${member.id}-${member.character}`} className="cast-card" withBorder p="sm"><Avatar src={member.profile_path ? posterURL(member.profile_path, "w185") : undefined} radius="xl" size="lg" mb="xs">{member.name.slice(0, 1)}</Avatar><Text fw={650} size="sm" lineClamp={2}>{member.name}</Text><Text size="xs" c="dimmed" lineClamp={2}>{member.character || "Guest star"}</Text></Paper>)}</SimpleGrid></section> : null}
    {selectedEpisode && episodeDetails.data?.crew?.length ? <section className="detail-section"><Text className="section-kicker">Episode crew</Text><Group gap="xs">{episodeDetails.data.crew.filter(member => ["Director", "Writer", "Screenplay"].includes(member.job)).slice(0, 8).map(member => <Badge key={`${member.id}-${member.job}`} variant="light">{member.job}: {member.name}</Badge>)}</Group></section> : null}
    {!isSaved && <Text className="detail-hint" c="dimmed">This is a temporary preview. Mark an episode watched to add the show to Watching.</Text>}
  </div>;
}

function chunk<T>(values: T[], size: number): T[][] {
  const batches: T[][] = [];
  for (let index = 0; index < values.length; index += size) batches.push(values.slice(index, index + size));
  return batches;
}

function EpisodeActions({ entry, canRate, rating, onRate, playID, onWatch, onUnwatch, pending }: { entry: ShowEpisodeEntry; canRate: boolean; rating?: number | null; onRate: (rating: number | null) => void; playID?: string; onWatch: () => void; onUnwatch: (playID: string) => void; pending: boolean }) {
  return <Group className="detail-actions" mt="lg"><Badge color={entry.watched ? "teal" : "yellow"} variant="light">{entry.watched ? "Watched" : "Not watched"}</Badge><RatingStars value={rating} onChange={onRate} label="Episode rating" disabled={!canRate || pending} size="md" />{!canRate && <Text size="xs" c="dimmed">Add to library to rate</Text>}{entry.watched && playID ? <Button variant="default" leftSection={<IconEyeCheck size={16} />} loading={pending} onClick={() => onUnwatch(playID)}>Mark unwatched</Button> : !entry.watched && <Button leftSection={<IconEye size={16} />} loading={pending} onClick={onWatch}>Mark watched</Button>}</Group>;
}

type ActionMutation<T> = { isPending: boolean; mutate: (value: T) => void };
function MediaActions({ media, isSaved, status, rating, add, update, watched, playID, onWatch, onUnwatch, pending }: { media: SearchMedia; isSaved: boolean; status?: string; rating?: number | null; add: ActionMutation<"watching" | "watchlist">; update: ActionMutation<{ status: string; rating: number | null }>; watched: boolean; playID?: string; onWatch: () => void; onUnwatch: (playID: string) => void; pending: boolean }) {
  if (!isSaved) return <Group className="detail-actions" mt="lg"><MediaQuickActions media={media} busy={add.isPending || pending} onWatch={() => media.type === "tv" ? add.mutate("watching") : onWatch()} onWatchlist={() => add.mutate("watchlist")} /></Group>;
  return <Group className="detail-actions" mt="lg"><Select aria-label="Current list" value={status} onChange={value => value && update.mutate({ status: value, rating: rating ?? null })} data={[{ value: "watchlist", label: "Watchlist" }, { value: "watching", label: "Watching" }, { value: "paused", label: "Paused" }, { value: "dropped", label: "Dropped" }]} w={150} /><RatingStars value={rating} onChange={value => update.mutate({ status: status!, rating: value })} label="Media rating" disabled={pending} size="md" />{media.type === "movie" && (watched && playID ? <Button variant="default" leftSection={<IconCheck size={16} />} loading={pending} onClick={() => onUnwatch(playID)}>Mark unwatched</Button> : <Button variant="light" leftSection={<IconCheck size={16} />} loading={pending} onClick={onWatch}>Mark watched</Button>)}</Group>;
}
