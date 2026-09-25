import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ActionIcon, Alert, Badge, Button, Group, Image, Loader, Menu, Modal, Paper, Select, Stack, Switch, Text, Title, Tooltip } from "@mantine/core";
import { IconArrowLeft, IconClock, IconEye, IconEyeCheck, IconRefresh } from "@tabler/icons-react";
import { api } from "../../lib/api";
import { useInvalidateUserCache, userCache } from "../../lib/userCache";
import { backdropURL, posterURL } from "../../lib/artwork";
import { useUserQueryKey } from "../auth/SessionContext";
import { findMissingPriorEpisodes } from "../library/episodeSelection";
import { CastSection } from "../../components/CastSection";
import { MediaPosterCard } from "../../components/MediaPosterCard";
import { regularSeasonsThrough, selectUnwatchedEpisodes, selectWatchedEpisodes } from "./watchSelection";
import { resolveMediaID } from "./mediaIdentity";
import { heroArtworkLayers } from "./heroArtwork";
import { EpisodeActions, MediaActions } from "./MediaDetailActions";
import { chunk } from "./batch";
import { useDetailHistoryQuery, useEpisodeDetailsQuery, useEpisodeRatingQuery, useMediaDetailQueries, useShowEpisodesQuery, useTemporarySeasonEpisodesQuery } from "./queries";
import type { EpisodeRating, MediaDetailTarget, ShowEpisodeEntry } from "../../types";

type Props = { target: MediaDetailTarget; onBack: () => void; onOpenDetail: (target: MediaDetailTarget) => void; onOpenPerson: (personID: number) => void };
type PendingWatch = { target: ShowEpisodeEntry | null; episodes: ShowEpisodeEntry[]; seasonNumber?: number; action?: "watch" | "rewatch" | "unwatch" };

export function MediaDetailPage({ target, onBack, onOpenDetail, onOpenPerson }: Props) {
  const userQueryKey = useUserQueryKey();
  const queryClient = useQueryClient();
  const invalidate = useInvalidateUserCache();
  const [season, setSeason] = useState<string | null>(null);
  const [pendingWatch, setPendingWatch] = useState<PendingWatch | null>(null);
  const [showWatchModal, setShowWatchModal] = useState(false);
  const [showWatchAction, setShowWatchAction] = useState<"watch" | "rewatch" | "unwatch">("watch");
  const [selectedShowSeasons, setSelectedShowSeasons] = useState<Record<number, boolean>>({});
  const { library, show: temporary, movie: movieDetails, related } = useMediaDetailQueries(target);
  const seed = target.seed;
  const media = library.data?.media
    ? { ...library.data.media, backdrop_path: library.data.media.backdrop_path ?? temporary.data?.media.backdrop_path }
    : target.mediaType === "movie" ? movieDetails.data?.media ?? seed : temporary.data?.media ?? seed;
  const savedMediaID = library.data?.item.media_id || undefined;
  const showID = resolveMediaID(target.mediaID, savedMediaID, media);
  const detailScope = ["media-detail", target.mediaType, target.tmdbID];
  const episodeScopes = [["detail-episodes", showID], ["detail-history"], userCache.library, userCache.continue, userCache.history, userCache.feed, userCache.calendar];
  const invalidateEpisodeData = () => invalidate(...episodeScopes);
  const episodes = useShowEpisodesQuery(showID, target.mediaType === "tv" && Boolean(savedMediaID));
  const history = useDetailHistoryQuery(Boolean(library.data));
  const ensureTrackedEpisodes = async (): Promise<ShowEpisodeEntry[]> => {
    if (!media || !showID) throw new Error("This show is not available.");
    if (!savedMediaID) {
      await api.post("/api/v1/library", { media, status: "watching" }, "Could not add this show to Watching.");
    } else if (library.data?.item.status === "watchlist") {
      await api.patch(`/api/v1/library/${encodeURIComponent(showID)}`, { status: "watching", rating: library.data.item.rating }, "Could not move this show to Watching.");
    }
    const all = await api.get<ShowEpisodeEntry[]>(`/api/v1/shows/${encodeURIComponent(showID)}/episodes`, "Could not load episodes after adding this show.");
    queryClient.setQueryData(userQueryKey("detail-episodes", showID), all);
    await invalidate(userCache.library, ["media-detail", target.mediaType, target.tmdbID]);
    return all;
  };
  const update = useMutation({
    mutationFn: ({ status, rating }: { status: string; rating: number | null }) => api.patch(`/api/v1/library/${encodeURIComponent(showID!)}`, { status, rating }, "Could not update this title."),
    onSuccess: () => invalidate(userCache.library, userCache.continue, userCache.calendar, userCache.feed, detailScope),
  });
  const updateNotifications = useMutation({
    mutationFn: (enabled: boolean) => api.patch(`/api/v1/library/${encodeURIComponent(showID!)}/notifications`, { enabled }, "Could not update show notifications."),
    onSuccess: () => invalidate(userCache.library, detailScope),
  });
  const add = useMutation({
    mutationFn: (status: "watching" | "watchlist") => api.post("/api/v1/library", { media, status }, "Could not add this title."),
    onSuccess: () => invalidate(userCache.library, detailScope),
  });
  const markMovieWatched = useMutation({
    mutationFn: async () => {
      if (!savedMediaID) await api.post("/api/v1/library", { media, status: "watching" }, "Could not add this title.");
      return api.post("/api/v1/plays", { media_id: showID }, "Could not record this watch.");
    },
    onSuccess: () => invalidate(userCache.library, userCache.history, userCache.feed, detailScope),
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
    onSuccess: () => { setPendingWatch(null); setShowWatchModal(false); return invalidateEpisodeData(); },
  });
  const markEpisodesWatched = useMutation({
    mutationFn: async ({ episodeIDs, selectedSeasons, rewatch = false }: { episodeIDs?: string[]; selectedSeasons?: number[]; rewatch?: boolean }) => {
      const all = await ensureTrackedEpisodes();
      const today = new Date().toISOString().slice(0, 10);
      const selectedIDs = selectedSeasons
        ? rewatch ? selectWatchedEpisodes(all, selectedSeasons) : selectUnwatchedEpisodes(all, selectedSeasons, today)
        : (episodeIDs ?? []);
      for (const batch of chunk([...new Set(selectedIDs)], 100)) {
        await api.post("/api/v1/plays/bulk", { episode_ids: batch }, "Could not record these watches.");
      }
    },
    onSuccess: () => { setPendingWatch(null); setShowWatchModal(false); return invalidateEpisodeData(); },
  });
  const removeEpisodesWatched = useMutation({
    mutationFn: async (episodeIDs: string[]) => {
      for (const batch of chunk([...new Set(episodeIDs)], 100)) {
        await api.delete("/api/v1/plays/bulk", "Could not mark these episodes unwatched.", { episode_ids: batch });
      }
    },
    onSuccess: () => { setPendingWatch(null); setShowWatchModal(false); return invalidateEpisodeData(); },
  });
  const removeMovieWatches = useMutation({
    mutationFn: () => api.delete(`/api/v1/plays/media/${encodeURIComponent(showID!)}`, "Could not mark this movie unwatched."),
    onSuccess: () => invalidate(["detail-history"], userCache.library, userCache.history, userCache.feed, detailScope),
  });
  const isSaved = Boolean(savedMediaID);
  const availableSeasonNumbers = isSaved
    ? [...new Set((episodes.data ?? []).map(entry => entry.episode.season_number))]
    : (temporary.data?.seasons.map(season => season.season_number) ?? []);
  const selectedSeason = season ?? (target.seasonNumber !== undefined ? String(target.seasonNumber) : availableSeasonNumbers.length ? String(availableSeasonNumbers.find(number => number > 0) ?? availableSeasonNumbers[0]) : null);
  const temporarySeason = Number(selectedSeason);
  const temporaryEpisodes = useTemporarySeasonEpisodesQuery(target, !isSaved && Boolean(selectedSeason), temporarySeason);
  const candidateEpisodes = isSaved ? (episodes.data ?? []) : (temporaryEpisodes.data?.episodes ?? []);
  const candidateEpisode = target.episodeID ? candidateEpisodes.find(entry => entry.episode.id === target.episodeID) : null;
  const episodeSeasonNumber = candidateEpisode?.episode.season_number ?? target.seasonNumber;
  const episodeNumber = candidateEpisode?.episode.episode_number ?? target.episode?.episode_number;
  const episodeDetails = useEpisodeDetailsQuery(target, episodeSeasonNumber, episodeNumber);
  const episodeRating = useEpisodeRatingQuery(target.episodeID);
  const rateEpisode = useMutation({
    mutationFn: (rating: number | null) => api.put<EpisodeRating>(`/api/v1/episodes/${encodeURIComponent(target.episodeID!)}/rating`, { rating }, "Could not save episode rating."),
    onSuccess: () => invalidate(["episode-rating", target.episodeID], userCache.feed),
  });

  const fallbackDetails = target.mediaType === "tv" ? temporary : movieDetails;
  if (!media && (library.isPending || fallbackDetails.isPending)) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (!media) return <Alert color="yellow" mt="md" title="Could not load this title">
    {fallbackDetails.error instanceof Error ? fallbackDetails.error.message : library.error instanceof Error ? library.error.message : "The media details are temporarily unavailable."}
    <Button variant="subtle" size="compact-sm" ml="sm" onClick={() => { void library.refetch(); void fallbackDetails.refetch(); }}>Try again</Button>
  </Alert>;

  const art = posterURL(media.poster_path, "w500");
  const temporaryEpisodeEntries = temporaryEpisodes.data?.episodes ?? [];
  const episodeEntries = isSaved ? (episodes.data ?? []) : temporaryEpisodeEntries;
  const seasons = availableSeasonNumbers;
  const visibleEpisodes = episodeEntries.filter(entry => String(entry.episode.season_number) === selectedSeason);
  const selectedEpisode = target.episodeID ? episodeEntries.find(entry => entry.episode.id === target.episodeID) ?? (target.episode ? { episode: target.episode, name: `Episode ${target.episode.episode_number}`, overview: "", runtime: 0, still_path: "", watched: false } : null) : null;
  const episodeStill = episodeDetails.data?.still_path || selectedEpisode?.still_path;
  const episodeArtwork = episodeStill ? backdropURL(episodeStill, "w780") : null;
  const episodeArtworkPending = Boolean(target.episodeID) && !episodeArtwork && (episodeDetails.isFetching || (isSaved ? episodes.isFetching : temporaryEpisodes.isFetching));
  const artLayers = heroArtworkLayers(Boolean(target.episodeID), episodeArtwork, episodeArtworkPending, backdropURL(media.backdrop_path, "w1280") ?? art);
  const heroBackground = [
    "linear-gradient(0deg, rgba(9,13,18,.94) 0%, rgba(9,13,18,.52) 28%, rgba(9,13,18,.08) 72%)",
    "linear-gradient(90deg, rgba(9,13,18,.55) 0%, rgba(9,13,18,.2) 60%, rgba(9,13,18,.08) 100%)",
    ...artLayers.map(layer => `url(${layer})`),
  ].join(", ");
  const watchedPlay = history.data?.find(item => selectedEpisode ? item.play.episode_id === selectedEpisode.episode.id : item.play.media_id === showID);
  const status = library.data?.item.status;
  const today = new Date().toISOString().slice(0, 10);
  const releasedSeasonEpisodes = visibleEpisodes.filter(entry => !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today));
  const seasonGroups = [...new Set(isSaved ? episodeEntries.map(entry => entry.episode.season_number) : temporary.data?.seasons.map(season => season.season_number) ?? [])].sort((a, b) => a - b).map(number => ({ number, episodes: episodeEntries.filter(entry => entry.episode.season_number === number) }));
  const releasedShowEpisodes = episodeEntries.filter(entry => !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today));
  const selectedShowEpisodes = (showWatchAction === "rewatch" || showWatchAction === "unwatch"
    ? episodeEntries.filter(entry => entry.watched)
    : releasedShowEpisodes).filter(entry => selectedShowSeasons[entry.episode.season_number] === true);
  const openShowWatchModal = () => {
    setShowWatchAction("watch");
    setSelectedShowSeasons(Object.fromEntries(seasonGroups.map(group => [group.number, group.number > 0])));
    setShowWatchModal(true);
  };
  const openShowActionModal = (action: "rewatch" | "unwatch") => {
    setShowWatchAction(action);
    setSelectedShowSeasons(Object.fromEntries(seasonGroups.map(group => [group.number, group.number > 0])));
    setShowWatchModal(true);
  };
  const requestEpisodeWatch = (entry: ShowEpisodeEntry) => prepareEpisodeWatch.mutate(entry);
  const confirmWatch = (includeTarget: boolean, includePreviousSeasons = false) => {
    if (!pendingWatch) return;
    if (pendingWatch.seasonNumber !== undefined) {
      const seasonNumbers = includePreviousSeasons
        ? regularSeasonsThrough(seasonGroups.map(group => group.number), pendingWatch.seasonNumber)
        : [pendingWatch.seasonNumber];
      if (pendingWatch.action === "unwatch") {
        removeEpisodesWatched.mutate(selectWatchedEpisodes(episodeEntries, seasonNumbers));
      } else {
        markEpisodesWatched.mutate({ selectedSeasons: seasonNumbers, rewatch: pendingWatch.action === "rewatch" });
      }
      return;
    }
    const entries = includeTarget && pendingWatch.target ? [...pendingWatch.episodes, pendingWatch.target] : pendingWatch.episodes;
    markEpisodesWatched.mutate({ episodeIDs: entries.map(entry => entry.episode.id) });
  };
  const requestSeasonWatch = (action: "watch" | "rewatch" | "unwatch" = "watch") => setPendingWatch({ target: null, episodes: action === "watch" ? releasedSeasonEpisodes : visibleEpisodes.filter(entry => entry.watched), seasonNumber: Number(selectedSeason), action });
  const confirmShowAction = () => {
    const seasonNumbers = Object.entries(selectedShowSeasons).filter(([, selected]) => selected).map(([number]) => Number(number));
    if (showWatchAction === "unwatch") removeEpisodesWatched.mutate(selectWatchedEpisodes(episodeEntries, seasonNumbers));
    else markEpisodesWatched.mutate({ selectedSeasons: seasonNumbers, rewatch: showWatchAction === "rewatch" });
  };
  const hasWatchedShowEpisodes = episodeEntries.some(entry => entry.watched);
  const hasPreviousSeasons = pendingWatch?.seasonNumber !== undefined && seasonGroups.some(group => group.number > 0 && group.number < pendingWatch.seasonNumber!);

  return <div className="detail-page">
    <Modal opened={pendingWatch !== null} onClose={() => setPendingWatch(null)} title={pendingWatch?.target ? "Skipped episodes" : pendingWatch?.action === "unwatch" ? "Mark season unwatched" : pendingWatch?.action === "rewatch" ? "Rewatch season" : "Mark season watched"} centered>
      {pendingWatch?.target ? <Text mb="md">There are {pendingWatch.episodes.length} earlier unwatched episodes. How would you like to continue?</Text> : pendingWatch?.action === "unwatch" ? <Text mb="md">Remove watch history for {pendingWatch?.episodes.length ?? 0} watched episodes in this season?</Text> : pendingWatch?.action === "rewatch" ? <Text mb="md">Add a new watch for {pendingWatch?.episodes.length ?? 0} episodes in this season?</Text> : <Text mb="md">Mark {pendingWatch?.episodes.length ?? 0} released episodes in this season as watched?{hasPreviousSeasons ? " Would you also like to mark previous seasons? Specials will not be included." : ""}</Text>}
      {markEpisodesWatched.isError && <Alert color="red" mb="md">{markEpisodesWatched.error.message}</Alert>}
      {removeEpisodesWatched.isError && <Alert color="red" mb="md">{removeEpisodesWatched.error.message}</Alert>}
      {markEpisodeWatched.isError && <Alert color="red" mb="md">{markEpisodeWatched.error.message}</Alert>}
      <Group justify="flex-end">
        {pendingWatch?.target && <Button variant="default" onClick={() => markEpisodeWatched.mutate(pendingWatch.target!.episode.id)} loading={markEpisodeWatched.isPending || markEpisodesWatched.isPending}>Only this episode</Button>}
        <Button onClick={() => confirmWatch(Boolean(pendingWatch?.target))} loading={markEpisodeWatched.isPending || markEpisodesWatched.isPending || removeEpisodesWatched.isPending}>{pendingWatch?.target ? "Mark all as watched" : pendingWatch?.action === "unwatch" ? "Mark unwatched" : pendingWatch?.action === "rewatch" ? "Rewatch episodes" : hasPreviousSeasons ? "Only this season" : "Mark season watched"}</Button>
        {hasPreviousSeasons && pendingWatch?.action !== "rewatch" && pendingWatch?.action !== "unwatch" && <Button onClick={() => confirmWatch(false, true)} loading={markEpisodesWatched.isPending}>This and previous seasons</Button>}
      </Group>
    </Modal>
    <Modal opened={showWatchModal} onClose={() => setShowWatchModal(false)} title={`${showWatchAction === "unwatch" ? "Mark" : showWatchAction === "rewatch" ? "Rewatch" : "Mark"} ${media.title} ${showWatchAction === "unwatch" ? "unwatched" : "watched"}`} centered>
      <Text size="sm" c="dimmed" mb="md">{showWatchAction === "unwatch" ? "Choose which seasons to remove from watch history. Specials are off by default." : showWatchAction === "rewatch" ? "Choose which seasons to add to your watch history again. Specials are off by default." : "Choose the seasons to include. Specials are off by default."}</Text>
      <Stack gap="xs">
        {seasonGroups.map(group => {
          const remaining = group.episodes.filter(entry => showWatchAction === "rewatch" || showWatchAction === "unwatch" ? entry.watched : !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today)).length;
          return <Paper key={group.number} withBorder p="sm"><Group justify="space-between"><div><Text fw={650}>{group.number === 0 ? "Specials" : `Season ${group.number}`}</Text><Text size="xs" c="dimmed">{isSaved ? `${remaining} ${showWatchAction === "unwatch" ? "watched " : ""}episodes` : "Episodes load when you confirm"}</Text></div><Switch aria-label={`Include ${group.number === 0 ? "specials" : `season ${group.number}`}`} checked={selectedShowSeasons[group.number] === true} onChange={event => setSelectedShowSeasons(current => ({ ...current, [group.number]: event.currentTarget.checked }))} /></Group></Paper>;
        })}
      </Stack>
      {markEpisodesWatched.isError && <Alert color="red" mt="md">{markEpisodesWatched.error.message}</Alert>}
      {removeEpisodesWatched.isError && <Alert color="red" mt="md">{removeEpisodesWatched.error.message}</Alert>}
      <Group justify="flex-end" mt="lg"><Button variant="default" onClick={() => setShowWatchModal(false)}>Cancel</Button><Button disabled={!Object.values(selectedShowSeasons).some(Boolean) || (isSaved && selectedShowEpisodes.length === 0)} loading={markEpisodesWatched.isPending || removeEpisodesWatched.isPending} onClick={confirmShowAction}>{isSaved ? showWatchAction === "unwatch" ? `Mark ${selectedShowEpisodes.length} unwatched` : showWatchAction === "rewatch" ? `Rewatch ${selectedShowEpisodes.length} episodes` : `Mark ${selectedShowEpisodes.length} episodes watched` : "Mark selected seasons watched"}</Button></Group>
    </Modal>
    <Button className="detail-back" variant="subtle" leftSection={<IconArrowLeft size={17} />} onClick={onBack}>Back</Button>
    <section className="detail-hero" style={{ backgroundImage: heroBackground }}>
      <div className="detail-hero-content">
        <Badge className="watch-kind" variant="filled">{media.type === "tv" ? "TV show" : "Movie"}</Badge>
        <Title order={1}>{selectedEpisode ? selectedEpisode.name : media.title}</Title>
        {selectedEpisode ? <Button className="detail-season-link" variant="subtle" onClick={() => onOpenDetail({ mediaType: "tv", tmdbID: media.tmdb_id, mediaID: showID, seasonNumber: selectedEpisode.episode.season_number })}>{`${media.title} · Season ${selectedEpisode.episode.season_number}`}</Button> : <Text className="detail-subtitle">{`${media.release_date ? media.release_date.slice(0, 4) : ""}${media.original_language ? ` · ${media.original_language.toUpperCase()}` : ""}`}</Text>}
        <Text className="detail-overview">{selectedEpisode ? (episodeDetails.data?.overview || selectedEpisode.overview || "Episode details are shown from your local catalog.") : media.overview || "No description is available."}</Text>
        {(selectedEpisode ? (episodeDetails.data?.runtime || selectedEpisode.runtime) : movieDetails.data?.runtime) ? <Text className="detail-runtime" mt="sm"><IconClock size={15} /> {selectedEpisode ? (episodeDetails.data?.runtime || selectedEpisode.runtime) : movieDetails.data?.runtime} min</Text> : null}
        {selectedEpisode && episodeDetails.data?.vote_average ? <Text className="detail-runtime" mt="sm">TMDB {episodeDetails.data.vote_average.toFixed(1)} / 10</Text> : null}
        {selectedEpisode && episodeDetails.data?.air_date ? <Text className="detail-runtime" mt="sm">Aired {episodeDetails.data.air_date}{episodeDetails.data.production_code ? ` · ${episodeDetails.data.production_code}` : ""}</Text> : null}
        {selectedEpisode ? <EpisodeActions entry={selectedEpisode} canRate={isSaved} rating={episodeRating.data?.rating} onRate={rating => rateEpisode.mutate(rating)} onWatch={() => requestEpisodeWatch(selectedEpisode)} onRewatch={() => markEpisodeWatched.mutate(selectedEpisode.episode.id)} onUnwatch={() => removeEpisodesWatched.mutate([selectedEpisode.episode.id])} pending={prepareEpisodeWatch.isPending || markEpisodeWatched.isPending || removeEpisodesWatched.isPending || rateEpisode.isPending} /> : <MediaActions media={media} isSaved={isSaved} status={status} rating={library.data?.item.rating} notificationsEnabled={library.data?.item.notifications_enabled ?? true} add={add} update={update} updateNotifications={updateNotifications} watched={Boolean(watchedPlay) || library.data?.completed === true} onWatch={() => markMovieWatched.mutate()} onUnwatch={() => removeMovieWatches.mutate()} pending={markMovieWatched.isPending || removeMovieWatches.isPending} />}
      </div>
    </section>
    {updateNotifications.isError && <Alert color="red" mt="sm">{updateNotifications.error.message}</Alert>}
    {removeMovieWatches.isError && <Alert color="red" mt="sm">{removeMovieWatches.error.message}</Alert>}
    {markMovieWatched.isError && <Alert color="red" mt="sm">{markMovieWatched.error.message}</Alert>}
    {selectedEpisode && removeEpisodesWatched.isError && !pendingWatch && !showWatchModal && <Alert color="red" mt="sm">{removeEpisodesWatched.error.message}</Alert>}
    {selectedEpisode && prepareEpisodeWatch.isError && <Alert color="red" mt="sm">{prepareEpisodeWatch.error.message}</Alert>}
    {selectedEpisode && markEpisodeWatched.isError && <Alert color="red" mt="sm">{markEpisodeWatched.error.message}</Alert>}
    {selectedEpisode && markEpisodesWatched.isError && !pendingWatch && !showWatchModal && <Alert color="red" mt="sm">{markEpisodesWatched.error.message}</Alert>}
    {target.mediaType === "tv" && !selectedEpisode && <section className="detail-section">
      <Group justify="space-between" align="end" mb="sm"><div><Text className="section-kicker">{isSaved ? "Your catalog" : "From TMDB"}</Text><Title order={2}>Seasons & episodes</Title></div><Group gap="xs">{seasonGroups.length > 0 && (!isSaved || releasedShowEpisodes.length > 0) && <Button size="xs" onClick={openShowWatchModal}>Mark show watched</Button>}{releasedSeasonEpisodes.length > 0 && <Button size="xs" variant="light" onClick={() => requestSeasonWatch("watch")}>Mark season watched</Button>}{seasons.length > 0 && <Select aria-label="Season" value={selectedSeason} onChange={setSeason} data={seasons.map(number => ({ value: String(number), label: number === 0 ? "Specials" : `Season ${number}` }))} w={150} />}</Group></Group>
      {hasWatchedShowEpisodes && <Group gap="xs" mb="sm"><Button size="xs" variant="default" leftSection={<IconRefresh size={14} />} onClick={() => openShowActionModal("rewatch")}>Rewatch show</Button><Button size="xs" variant="default" onClick={() => openShowActionModal("unwatch")}>Mark show unwatched</Button></Group>}
      {((isSaved && episodes.isPending) || (!isSaved && (temporary.isPending || temporaryEpisodes.isPending))) && <Group justify="center" py="lg"><Loader /></Group>}
      {episodes.isError && isSaved && <Alert color="red">Episodes are temporarily unavailable.</Alert>}
      {temporary.isError && !isSaved && <Alert color="red">TV details are temporarily unavailable.</Alert>}
      {temporaryEpisodes.isError && !isSaved && <Alert color="red">Season episodes are temporarily unavailable.</Alert>}
      {markEpisodesWatched.isError && <Alert color="red">{markEpisodesWatched.error.message}</Alert>}
      {prepareEpisodeWatch.isError && <Alert color="red">{prepareEpisodeWatch.error.message}</Alert>}
      {markEpisodeWatched.isError && <Alert color="red">{markEpisodeWatched.error.message}</Alert>}
      {add.isError && <Alert color="red">{add.error.message}</Alert>}
      {!episodes.isPending && !temporary.isPending && !temporaryEpisodes.isPending && !episodes.isError && !temporary.isError && !temporaryEpisodes.isError && !visibleEpisodes.length && <Text c="dimmed">Episode details are not available yet.</Text>}
      <Stack gap="xs">{visibleEpisodes.map(entry => {
        const openEpisode = () => onOpenDetail({ mediaType: "tv", tmdbID: media.tmdb_id, mediaID: showID, episodeID: entry.episode.id, episode: entry.episode, seasonNumber: entry.episode.season_number });
        return <Paper key={entry.episode.id} className="episode-row" withBorder p={0} role="button" tabIndex={0} onClick={openEpisode} onKeyDown={event => { if ((event.key === "Enter" || event.key === " ") && event.target === event.currentTarget) { event.preventDefault(); openEpisode(); } }}>
          <Group className="episode-row-layout" justify="space-between" wrap="nowrap" gap={0}>
            <Group className="episode-row-main" wrap="nowrap" gap={0}>
              <div className="episode-art">{entry.still_path ? <Image src={backdropURL(entry.still_path, "w780")!} alt="" /> : <div className="artwork-fallback">{entry.episode.episode_number}</div>}</div>
              <div className="episode-row-copy"><Text fw={650}>{`Episode ${entry.episode.episode_number}${entry.name ? ` · ${entry.name}` : ""}`}</Text><Text size="xs" c="dimmed">{entry.episode.air_date || "Air date not announced"}</Text>{entry.overview && <Text className="episode-description" size="sm" c="dimmed" mt={5}>{entry.overview}</Text>}</div>
            </Group>
            {entry.watched ? <Menu withinPortal position="bottom-end"><Menu.Target><ActionIcon className="episode-row-action" aria-label={`Episode ${entry.episode.episode_number} watched actions`} size="lg" variant="light" color="teal" onClick={event => event.stopPropagation()}><IconEyeCheck size={18} /></ActionIcon></Menu.Target><Menu.Dropdown onClick={event => event.stopPropagation()}><Menu.Item leftSection={<IconRefresh size={15} />} onClick={() => markEpisodeWatched.mutate(entry.episode.id)}>Rewatch episode</Menu.Item><Menu.Item leftSection={<IconEye size={15} />} onClick={() => removeEpisodesWatched.mutate([entry.episode.id])}>Mark unwatched</Menu.Item></Menu.Dropdown></Menu> : <Tooltip label={`Mark episode ${entry.episode.episode_number} watched`} withArrow><ActionIcon className="episode-row-action" aria-label={`Mark episode ${entry.episode.episode_number} watched`} size="lg" variant="default" disabled={prepareEpisodeWatch.isPending || markEpisodeWatched.isPending} loading={prepareEpisodeWatch.isPending} onClick={event => { event.stopPropagation(); requestEpisodeWatch(entry); }}><IconEye size={18} /></ActionIcon></Tooltip>}
          </Group>
        </Paper>;
      })}</Stack>
    </section>}
    {target.mediaType === "movie" && !selectedEpisode && movieDetails.data?.genres?.length ? <section className="detail-section"><Text className="section-kicker">About this film</Text><Group gap="xs">{movieDetails.data.genres.map(genre => <Badge key={genre} variant="light">{genre}</Badge>)}{movieDetails.data.vote_average ? <Badge variant="light">TMDB {movieDetails.data.vote_average.toFixed(1)} / 10</Badge> : null}</Group></section> : null}
    {!selectedEpisode && <CastSection members={target.mediaType === "tv" ? temporary.data?.cast ?? [] : movieDetails.data?.cast ?? []} title="Cast" kicker="People" onOpenPerson={onOpenPerson} />}
    {!target.episodeID && (related.isPending || related.isError || Boolean(related.data?.length)) && <section className="detail-section">
      <Text className="section-kicker">More to explore</Text><Title order={2}>More like this</Title>
      {related.isPending && <Group justify="center" py="lg"><Loader size="sm" /></Group>}
      {related.isError && <Alert color="yellow" mt="sm">Related titles are temporarily unavailable.</Alert>}
      {related.data?.length ? <div className="related-media-grid">{related.data.map(item => <MediaPosterCard key={`${item.type}:${item.tmdb_id}`} media={item} onOpenDetail={onOpenDetail} />)}</div> : null}
    </section>}
    {selectedEpisode && <CastSection members={episodeDetails.data?.guest_stars ?? []} title="Guest stars" kicker="Episode cast" fallbackRole="Guest star" onOpenPerson={onOpenPerson} />}
    {selectedEpisode && episodeDetails.data?.crew?.length ? <section className="detail-section"><Text className="section-kicker">Episode crew</Text><Group gap="xs">{episodeDetails.data.crew.filter(member => ["Director", "Writer", "Screenplay"].includes(member.job)).slice(0, 8).map(member => <Badge key={`${member.id}-${member.job}`} variant="light">{member.job}: {member.name}</Badge>)}</Group></section> : null}
    {!isSaved && <Text className="detail-hint" c="dimmed">This is a temporary preview. Mark an episode watched to add the show to Watching.</Text>}
  </div>;
}
