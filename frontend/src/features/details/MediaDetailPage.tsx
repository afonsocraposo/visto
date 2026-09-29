import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Checkbox,
  Drawer,
  Group,
  Image,
  Menu,
  Modal,
  Paper,
  Radio,
  Select,
  Skeleton,
  Stack,
  Switch,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import {
  IconArrowLeft,
  IconCheck,
  IconChevronDown,
  IconClock,
  IconEye,
  IconRefresh,
} from "@tabler/icons-react";
import { fetchAllPages } from "../../lib/pagination";
import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { PosterGridSkeleton } from "../../components/PosterGridSkeleton";
import { api } from "../../lib/api";
import { postPlaysBulk } from "../../generated/api";
import { showActionFeedback } from "../../lib/actionFeedback";
import type { Play } from "../../generated/models/play";
import { useInvalidateUserCache, userCache } from "../../lib/userCache";
import { backdropURL, posterURL } from "../../lib/artwork";
import { useUserQueryKey } from "../auth/SessionContext";
import { findMissingPriorEpisodes } from "../library/episodeSelection";
import { CastSection } from "../../components/CastSection";
import { HistoryPanel } from "../library/HistoryPanel";
import { MediaPosterCard } from "../../components/MediaPosterCard";
import {
  regularSeasonsThrough,
  selectUnwatchedEpisodes,
  selectWatchedEpisodes,
} from "./watchSelection";
import { resolveMediaID } from "./mediaIdentity";
import { heroArtworkLayers, resolveMediaArtwork } from "./heroArtwork";
import { EpisodeActions, MediaActions } from "./MediaDetailActions";
import { chunk } from "./batch";
import {
  useDetailHistoryQuery,
  useEpisodeDetailsQuery,
  useEpisodeRatingQuery,
  useMediaDetailQueries,
  useShowEpisodesQuery,
  useTemporarySeasonEpisodesQuery,
} from "./queries";
import type { EpisodeRating, MediaDetailTarget, ShowEpisodeEntry } from "../../types";

type Props = {
  target: MediaDetailTarget;
  returnTo?: string;
  onBack: () => void;
  onOpenDetail: (target: MediaDetailTarget, options?: { from?: string; replace?: boolean }) => void;
  onOpenPerson: (personID: number) => void;
};
type PendingWatch = {
  target: ShowEpisodeEntry | null;
  episodes: ShowEpisodeEntry[];
  seasonNumber?: number;
  action?: "watch" | "unwatch";
};

function showStatusLabel(status: string | undefined): string | null {
  const labels: Record<string, string> = {
    "Returning Series": "Ongoing",
    Ended: "Ended",
    Canceled: "Canceled",
    Cancelled: "Canceled",
    Planned: "Planned",
    "In Production": "In Production",
    Pilot: "Pilot",
  };
  return status ? (labels[status] ?? status) : null;
}

export function MediaDetailPage({ target, returnTo, onBack, onOpenDetail, onOpenPerson }: Props) {
  const userQueryKey = useUserQueryKey();
  const queryClient = useQueryClient();
  const invalidate = useInvalidateUserCache();
  const [pendingWatch, setPendingWatch] = useState<PendingWatch | null>(null);
  const [showWatchModal, setShowWatchModal] = useState(false);
  const [showNotificationsOpen, setShowNotificationsOpen] = useState(false);
  const [movieHistoryMode, setMovieHistoryMode] = useState<"view" | "edit" | null>(null);
  const [showWatchAction, setShowWatchAction] = useState<"watch" | "unwatch">("watch");
  const [selectedShowSeasons, setSelectedShowSeasons] = useState<Record<number, boolean>>({});
  const { library, show: temporary, movie: movieDetails, related } = useMediaDetailQueries(target);
  const savedMediaID = library.data?.item.media_id || undefined;
  const seed = target.seed;
  const media = library.data?.media
    ? library.data.media
    : target.mediaType === "movie"
      ? (movieDetails.data?.media ?? seed)
      : (temporary.data?.media ?? seed);
  const movieRuntime = savedMediaID ? library.data?.runtime : movieDetails.data?.runtime;
  const movieGenres = savedMediaID ? library.data?.genres : movieDetails.data?.genres;
  const movieScore = savedMediaID ? library.data?.vote_average : movieDetails.data?.vote_average;
  const showID = resolveMediaID(target.mediaID, savedMediaID, media);
  const isSaved = Boolean(savedMediaID);
  const savedSeasons = useQuery({
    queryKey: userQueryKey("show-seasons", showID),
    enabled: target.mediaType === "tv" && isSaved && Boolean(showID),
    queryFn: () =>
      api.get<Array<{ id: string; season_number: number; episode_count: number }>>(
        `/api/v1/shows/${encodeURIComponent(showID!)}/seasons`,
      ),
  });
  const savedProgress = useQuery({
    queryKey: userQueryKey("show-progress", showID),
    enabled: target.mediaType === "tv" && isSaved && Boolean(showID),
    queryFn: () =>
      api.get<{ is_fully_watched: boolean; watched_episodes: number }>(
        `/api/v1/shows/${encodeURIComponent(showID!)}/progress`,
      ),
  });
  const availableSeasonNumbers = isSaved
    ? (savedSeasons.data?.map((item) => item.season_number) ?? [])
    : (temporary.data?.seasons.map((item) => item.season_number) ?? []);
  const selectedSeason =
    target.seasonNumber !== undefined
      ? String(target.seasonNumber)
      : availableSeasonNumbers.length
        ? String(availableSeasonNumbers.find((number) => number > 0) ?? availableSeasonNumbers[0])
        : null;
  const selectedSeasonID = savedSeasons.data?.find(
    (item) => String(item.season_number) === selectedSeason,
  )?.id;
  const showNotificationMode = library.data?.item.season_alerts_enabled
    ? "season"
    : library.data?.item.notifications_enabled
      ? "episode"
      : "";
  const updateShowNotificationMode = useMutation({
    mutationFn: (mode: "episode" | "season") =>
      api.patch(
        `/api/v1/library/${encodeURIComponent(showID!)}/notifications`,
        { mode },
        "Could not update show notifications.",
      ),
    onSuccess: (_result, mode) => {
      setShowNotificationsOpen(false);
      void invalidate(userCache.library, detailScope);
      showActionFeedback(
        mode === "season"
          ? "You'll be notified when each season is fully available."
          : "You'll be notified about new episodes.",
      );
    },
  });
  const detailScope = ["media-detail", target.mediaType, target.tmdbID];
  const episodeScopes = [
    ["detail-episodes", showID],
    ["detail-history"],
    userCache.library,
    userCache.continue,
    userCache.history,
    userCache.feed,
    userCache.calendar,
  ];
  const invalidateEpisodeData = () => invalidate(...episodeScopes);
  const episodes = useShowEpisodesQuery(
    showID,
    selectedSeasonID,
    target.mediaType === "tv" && isSaved,
  );
  const loadedSelectedEpisode = episodes.data?.pages.some((page) =>
    page.items.some((entry) => entry.episode.id === target.episodeID),
  );
  useEffect(() => {
    if (
      target.episodeID &&
      isSaved &&
      episodes.hasNextPage &&
      !episodes.isFetchingNextPage &&
      !loadedSelectedEpisode
    ) {
      void episodes.fetchNextPage();
    }
  }, [
    target.episodeID,
    isSaved,
    episodes.hasNextPage,
    episodes.isFetchingNextPage,
    loadedSelectedEpisode,
    episodes.fetchNextPage,
  ]);
  const history = useDetailHistoryQuery(
    Boolean(library.data),
    target.episodeID,
    target.mediaType === "movie" ? showID : undefined,
  );
  const ensureTrackedEpisodes = async (): Promise<ShowEpisodeEntry[]> => {
    if (!media || !showID) throw new Error("This show is not available.");
    if (!savedMediaID) {
      await api.post(
        "/api/v1/library",
        { media, status: "watching" },
        "Could not add this show to Watching.",
      );
    } else if (library.data?.item.status === "watchlist") {
      await api.patch(
        `/api/v1/library/${encodeURIComponent(showID)}`,
        { status: "watching", rating: library.data.item.rating },
        "Could not move this show to Watching.",
      );
    }
    const all = await fetchAllPages<ShowEpisodeEntry>(
      `/api/v1/shows/${encodeURIComponent(showID)}/episodes`,
      "Could not load episodes after adding this show.",
    );
    await queryClient.invalidateQueries({ queryKey: userQueryKey("detail-episodes", showID) });
    await invalidate(userCache.library, ["media-detail", target.mediaType, target.tmdbID]);
    return all;
  };
  const update = useMutation({
    mutationFn: ({
      status,
      rating,
      confirm_all_episodes,
    }: {
      status: string;
      rating: number | null;
      confirm_all_episodes?: boolean;
    }) =>
      api.patch<{ created_episode_ids?: string[] }>(
        `/api/v1/library/${encodeURIComponent(showID!)}`,
        { status, rating, confirm_all_episodes },
        "Could not update this title.",
      ),
    onSuccess: (result, changed) => {
      void invalidate(
        userCache.library,
        userCache.continue,
        userCache.calendar,
        userCache.feed,
        detailScope,
      );
      const previousStatus = library.data?.item.status;
      const previousRating = library.data?.item.rating ?? null;
      if (changed.status === "completed" && previousStatus !== "completed") {
        const createdEpisodeIDs = result.created_episode_ids ?? [];
        showActionFeedback(
          "Show completed.",
          createdEpisodeIDs.length && previousStatus
            ? async () => {
                for (const batch of chunk(createdEpisodeIDs, 100)) {
                  await api.delete("/api/v1/plays/bulk", "Could not undo show completion.", {
                    episode_ids: batch,
                  });
                }
                await api.patch(`/api/v1/library/${encodeURIComponent(showID!)}`, {
                  status: previousStatus,
                  rating: previousRating,
                });
                await invalidate(
                  userCache.library,
                  userCache.continue,
                  userCache.calendar,
                  userCache.feed,
                  detailScope,
                );
              }
            : undefined,
        );
      } else if (
        previousStatus &&
        (previousStatus !== changed.status || previousRating !== changed.rating)
      ) {
        showActionFeedback(
          previousStatus !== changed.status ? "List updated." : "Rating saved.",
          async () => {
            await api.patch(`/api/v1/library/${encodeURIComponent(showID!)}`, {
              status: previousStatus,
              rating: previousRating,
            });
            await invalidate(
              userCache.library,
              userCache.continue,
              userCache.calendar,
              userCache.feed,
              detailScope,
            );
          },
        );
      }
    },
  });
  const updateNotifications = useMutation({
    mutationFn: (enabled: boolean) =>
      api.patch(
        `/api/v1/library/${encodeURIComponent(showID!)}/notifications`,
        { enabled },
        "Could not update show notifications.",
      ),
    onSuccess: () => invalidate(userCache.library, detailScope),
  });
  const add = useMutation({
    mutationFn: (status: "watching" | "watchlist" | "paused" | "dropped" | "completed") =>
      api.post<{ created_episode_ids?: string[] }>(
        "/api/v1/library",
        { media, status, confirm_all_episodes: status === "completed" },
        "Could not add this title.",
      ),
    onSuccess: (result, status) => {
      void invalidate(userCache.library, detailScope);
      if (status === "completed") {
        const createdEpisodeIDs = result.created_episode_ids ?? [];
        showActionFeedback(
          `${media?.title} completed.`,
          target.mediaType === "tv" && createdEpisodeIDs.length
            ? async () => {
                for (const batch of chunk(createdEpisodeIDs, 100)) {
                  await api.delete("/api/v1/plays/bulk", "Could not undo show completion.", {
                    episode_ids: batch,
                  });
                }
                await api.delete(
                  `/api/v1/library/${encodeURIComponent(showID!)}?status=watching`,
                  "Could not remove the show from your library.",
                );
                await invalidate(userCache.library, detailScope);
              }
            : undefined,
        );
        return;
      }
      showActionFeedback(
        `${media?.title} added to ${status[0].toUpperCase() + status.slice(1)}.`,
        async () => {
          await api.delete(`/api/v1/library/${encodeURIComponent(showID!)}?status=${status}`);
          await invalidate(userCache.library, detailScope);
        },
      );
    },
  });
  const removeWatchlist = useMutation({
    mutationFn: () =>
      api.delete(
        `/api/v1/library/${encodeURIComponent(showID!)}`,
        "Could not remove this title from your watchlist.",
      ),
    onSuccess: () => {
      void invalidate(userCache.library, detailScope);
      showActionFeedback(`${media?.title} removed from Watchlist.`, async () => {
        await api.post("/api/v1/library", {
          media,
          status: "watchlist",
          rating: library.data?.item.rating ?? null,
        });
        await invalidate(userCache.library, detailScope);
      });
    },
  });
  const removeCurrentList = useMutation({
    mutationFn: (status: "watching" | "paused" | "dropped") =>
      api.delete(
        `/api/v1/library/${encodeURIComponent(showID!)}?status=${status}`,
        "Could not remove this show from its list.",
      ),
    onSuccess: (_result, status) => {
      const label = status[0].toUpperCase() + status.slice(1);
      void invalidate(userCache.library, userCache.continue, detailScope);
      showActionFeedback(`${media?.title} removed from ${label}.`, async () => {
        await api.post("/api/v1/library", {
          media,
          status,
          rating: library.data?.item.rating ?? null,
        });
        await invalidate(userCache.library, userCache.continue, detailScope);
      });
    },
  });
  const markMovieWatched = useMutation({
    mutationFn: async () => {
      if (!savedMediaID)
        await api.post(
          "/api/v1/library",
          { media, status: "watchlist" },
          "Could not add this title.",
        );
      let play: Play;
      try {
        play = await api.post<Play>(
          "/api/v1/plays",
          { media_id: showID },
          "Could not record this watch.",
        );
      } catch (error) {
        if (!savedMediaID)
          await api.delete(`/api/v1/library/${encodeURIComponent(showID!)}`).catch(() => undefined);
        throw error;
      }
      return {
        play,
        wasSaved: Boolean(savedMediaID),
        previousStatus: library.data?.item.status,
        previousRating: library.data?.item.rating ?? null,
      };
    },
    onSuccess: ({ play, wasSaved, previousStatus, previousRating }) => {
      void invalidate(userCache.library, userCache.history, userCache.feed, detailScope);
      showActionFeedback(`${media?.title} marked watched.`, async () => {
        await api.delete(`/api/v1/plays/${encodeURIComponent(play.id)}`);
        if (wasSaved && previousStatus === "watchlist")
          await api.post("/api/v1/library", { media, status: "watchlist", rating: previousRating });
        await invalidate(userCache.library, userCache.history, userCache.feed, detailScope);
      });
    },
  });
  const prepareEpisodeWatch = useMutation({
    mutationFn: async (entry: ShowEpisodeEntry) => {
      const all = await ensureTrackedEpisodes();
      const tracked = all.find((candidate) => candidate.episode.id === entry.episode.id);
      if (!tracked) throw new Error("This episode is not available in the show catalog.");
      return {
        tracked,
        missing: findMissingPriorEpisodes(all, tracked, new Date().toISOString().slice(0, 10)),
      };
    },
    onSuccess: ({ tracked, missing }) => {
      if (tracked.watched) return;
      if (missing.length) setPendingWatch({ target: tracked, episodes: missing });
      else markEpisodeWatched.mutate(tracked.episode.id);
    },
  });
  const markEpisodeWatched = useMutation({
    mutationFn: (episodeID: string) =>
      api.post<Play>("/api/v1/plays", { episode_id: episodeID }, "Could not record this watch."),
    onSuccess: (play) => {
      setPendingWatch(null);
      setShowWatchModal(false);
      showActionFeedback("Episode marked watched.", async () => {
        await api.delete(`/api/v1/plays/${encodeURIComponent(play.id)}`);
        await invalidateEpisodeData();
      });
      return invalidateEpisodeData();
    },
  });
  const markEpisodesWatched = useMutation({
    mutationFn: async ({
      episodeIDs,
      selectedSeasons,
    }: {
      episodeIDs?: string[];
      selectedSeasons?: number[];
    }) => {
      const all = await ensureTrackedEpisodes();
      const today = new Date().toISOString().slice(0, 10);
      const selectedIDs = selectedSeasons
        ? selectUnwatchedEpisodes(all, selectedSeasons, today)
        : (episodeIDs ?? []);
      const created: Play[] = [];
      for (const batch of chunk([...new Set(selectedIDs)], 100)) {
        created.push(...(await postPlaysBulk({ episode_ids: batch })));
      }
      return created;
    },
    onSuccess: (plays) => {
      setPendingWatch(null);
      setShowWatchModal(false);
      if (plays.length)
        showActionFeedback(
          `${plays.length} ${plays.length === 1 ? "episode" : "episodes"} marked watched.`,
          async () => {
            for (const play of plays)
              await api.delete(`/api/v1/plays/${encodeURIComponent(play.id)}`);
            await invalidateEpisodeData();
          },
        );
      return invalidateEpisodeData();
    },
  });
  const removeEpisodesWatched = useMutation({
    mutationFn: async (selection: string[] | { selectedSeasons: number[] }) => {
      const episodeIDs = Array.isArray(selection)
        ? selection
        : selectWatchedEpisodes(
            await fetchAllPages<ShowEpisodeEntry>(
              `/api/v1/shows/${encodeURIComponent(showID!)}/episodes`,
              "Could not load episodes.",
            ),
            selection.selectedSeasons,
          );
      const removed: Play[] = [];
      for (const batch of chunk([...new Set(episodeIDs)], 100)) {
        removed.push(
          ...(await api.delete<Play[]>(
            "/api/v1/plays/bulk",
            "Could not mark these episodes unwatched.",
            { episode_ids: batch },
          )),
        );
      }
      return removed;
    },
    onSuccess: (removed) => {
      setPendingWatch(null);
      setShowWatchModal(false);
      showActionFeedback(
        "Episodes marked unwatched.",
        removed.length
          ? async () => {
              for (const batch of chunk(removed, 10)) {
                await Promise.all(
                  batch.map((play) =>
                    api.post("/api/v1/plays", {
                      episode_id: play.episode_id,
                      watched_at: play.watched_at,
                      source: play.source,
                    }),
                  ),
                );
              }
              await invalidateEpisodeData();
            }
          : undefined,
      );
      return invalidateEpisodeData();
    },
  });
  const removeMovieWatches = useMutation({
    mutationFn: () =>
      api.delete(
        `/api/v1/plays/media/${encodeURIComponent(showID!)}`,
        "Could not mark this movie unwatched.",
      ),
    onSuccess: () => {
      showActionFeedback(`${media?.title} marked unwatched.`);
      return invalidate(
        ["detail-history"],
        userCache.library,
        userCache.history,
        userCache.feed,
        detailScope,
      );
    },
  });
  const temporarySeason = Number(selectedSeason);
  const temporaryEpisodes = useTemporarySeasonEpisodesQuery(
    target,
    !isSaved && Boolean(selectedSeason),
    temporarySeason,
  );
  const candidateEpisodes = isSaved
    ? (episodes.data?.pages.flatMap((page) => page.items) ?? [])
    : (temporaryEpisodes.data?.episodes ?? []);
  const candidateEpisode = target.episodeID
    ? candidateEpisodes.find((entry) => entry.episode.id === target.episodeID)
    : null;
  const episodeSeasonNumber = candidateEpisode?.episode.season_number ?? target.seasonNumber;
  const episodeNumber = candidateEpisode?.episode.episode_number ?? target.episode?.episode_number;
  const episodeDetails = useEpisodeDetailsQuery(target, episodeSeasonNumber, episodeNumber);
  const episodeRating = useEpisodeRatingQuery(target.episodeID);
  const rateEpisode = useMutation({
    mutationFn: (rating: number | null) =>
      api.put<EpisodeRating>(
        `/api/v1/episodes/${encodeURIComponent(target.episodeID!)}/rating`,
        { rating },
        "Could not save episode rating.",
      ),
    onSuccess: () => invalidate(["episode-rating", target.episodeID], userCache.feed),
  });

  const fallbackDetails = target.mediaType === "tv" ? temporary : movieDetails;
  if (!media && (library.isPending || fallbackDetails.isPending))
    return (
      <div className="detail-page">
        <section className="detail-hero">
          <Skeleton height={36} width={90} radius="xl" className="detail-hero-back" />
          <Skeleton
            height="100%"
            width="100%"
            radius={0}
            animate={false}
            style={{ position: "absolute", inset: 0, opacity: 0.4 }}
          />
          <div className="detail-hero-content">
            <div className="detail-poster">
              <Skeleton height="100%" width="100%" radius={0} />
            </div>
            <div className="detail-hero-body">
              <Skeleton height={22} width={90} radius="xl" mb="md" />
              <Skeleton height={48} width="55%" mb="sm" />
              <Skeleton height={16} width="25%" mb="lg" />
              <Skeleton height={14} width="90%" mb={6} />
              <Skeleton height={14} width="80%" mb={6} />
              <Skeleton height={14} width="65%" mb="lg" />
              <Group gap="sm">
                <Skeleton height={42} width={150} radius="sm" />
                <Skeleton height={42} width={42} circle />
              </Group>
            </div>
          </div>
        </section>
      </div>
    );
  if (!media)
    return (
      <Alert color="yellow" mt="md" title="Could not load this title">
        {fallbackDetails.error instanceof Error
          ? fallbackDetails.error.message
          : library.error instanceof Error
            ? library.error.message
            : "The media details are temporarily unavailable."}
        <Button
          variant="subtle"
          size="compact-sm"
          ml="sm"
          onClick={() => {
            void library.refetch();
            void fallbackDetails.refetch();
          }}
        >
          Try again
        </Button>
      </Alert>
    );

  const art = posterURL(media.poster_path, "w500");
  const temporaryEpisodeEntries = temporaryEpisodes.data?.episodes ?? [];
  const episodeEntries = isSaved
    ? (episodes.data?.pages.flatMap((page) => page.items) ?? [])
    : temporaryEpisodeEntries;
  const seasons = availableSeasonNumbers;
  const visibleEpisodes = episodeEntries.filter(
    (entry) => String(entry.episode.season_number) === selectedSeason,
  );
  const selectedEpisode = target.episodeID
    ? (episodeEntries.find((entry) => entry.episode.id === target.episodeID) ??
      (target.episode
        ? {
            episode: target.episode,
            name: `Episode ${target.episode.episode_number}`,
            overview: "",
            runtime: 0,
            still_path: "",
            watched: false,
          }
        : null))
    : null;
  const episodeStill = episodeDetails.data?.still_path || selectedEpisode?.still_path;
  const episodeArtwork = episodeStill ? backdropURL(episodeStill, "w780") : null;
  const episodeArtworkPending =
    Boolean(target.episodeID) &&
    !episodeArtwork &&
    (episodeDetails.isFetching || (isSaved ? episodes.isFetching : temporaryEpisodes.isFetching));
  const mediaArtwork = resolveMediaArtwork(
    backdropURL(media.backdrop_path, "w1280"),
    art,
    !isSaved && fallbackDetails.isPending,
  );
  const artLayers = heroArtworkLayers(
    Boolean(target.episodeID),
    episodeArtwork,
    episodeArtworkPending,
    mediaArtwork,
  );
  const heroBackground = [
    "linear-gradient(0deg, rgba(9,13,18,.94) 0%, rgba(9,13,18,.52) 28%, rgba(9,13,18,.08) 72%)",
    "linear-gradient(90deg, rgba(9,13,18,.55) 0%, rgba(9,13,18,.2) 60%, rgba(9,13,18,.08) 100%)",
    ...artLayers.map((layer) => `url(${layer})`),
  ].join(", ");
  const watchedPlay = history.data?.items.find((item) =>
    selectedEpisode
      ? item.play.episode_id === selectedEpisode.episode.id
      : item.play.media_id === showID,
  );
  const status = library.data?.item.status;
  const today = new Date().toISOString().slice(0, 10);
  const releasedSeasonEpisodes = visibleEpisodes.filter(
    (entry) => !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today),
  );
  const watchedSeasonEpisodes = visibleEpisodes.filter((entry) => entry.watched);
  const seasonGroups = [
    ...new Set(
      isSaved
        ? availableSeasonNumbers
        : (temporary.data?.seasons.map((season) => season.season_number) ?? []),
    ),
  ]
    .sort((a, b) => a - b)
    .map((number) => ({
      number,
      episodes: episodeEntries.filter((entry) => entry.episode.season_number === number),
    }));
  const releasedShowEpisodes = episodeEntries.filter(
    (entry) => !entry.watched && (!entry.episode.air_date || entry.episode.air_date <= today),
  );
  const openShowWatchModal = () => {
    setShowWatchAction("watch");
    setSelectedShowSeasons(
      Object.fromEntries(seasonGroups.map((group) => [group.number, group.number > 0])),
    );
    setShowWatchModal(true);
  };
  const openShowActionModal = (action: "unwatch") => {
    setShowWatchAction(action);
    setSelectedShowSeasons(
      Object.fromEntries(seasonGroups.map((group) => [group.number, group.number > 0])),
    );
    setShowWatchModal(true);
  };
  const requestEpisodeWatch = (entry: ShowEpisodeEntry) => prepareEpisodeWatch.mutate(entry);
  const confirmWatch = (includeTarget: boolean, includePreviousSeasons = false) => {
    if (!pendingWatch) return;
    if (pendingWatch.seasonNumber !== undefined) {
      const seasonNumbers = includePreviousSeasons
        ? regularSeasonsThrough(
            seasonGroups.map((group) => group.number),
            pendingWatch.seasonNumber,
          )
        : [pendingWatch.seasonNumber];
      if (pendingWatch.action === "unwatch") {
        removeEpisodesWatched.mutate({ selectedSeasons: seasonNumbers });
      } else {
        markEpisodesWatched.mutate({
          selectedSeasons: seasonNumbers,
        });
      }
      return;
    }
    const entries =
      includeTarget && pendingWatch.target
        ? [...pendingWatch.episodes, pendingWatch.target]
        : pendingWatch.episodes;
    markEpisodesWatched.mutate({ episodeIDs: entries.map((entry) => entry.episode.id) });
  };
  const requestSeasonWatch = (action: "watch" | "unwatch" = "watch") =>
    setPendingWatch({
      target: null,
      episodes:
        action === "watch"
          ? releasedSeasonEpisodes
          : visibleEpisodes.filter((entry) => entry.watched),
      seasonNumber: Number(selectedSeason),
      action,
    });
  const confirmShowAction = () => {
    const seasonNumbers = Object.entries(selectedShowSeasons)
      .filter(([, selected]) => selected)
      .map(([number]) => Number(number));
    if (showWatchAction === "unwatch")
      removeEpisodesWatched.mutate({ selectedSeasons: seasonNumbers });
    else
      markEpisodesWatched.mutate({
        selectedSeasons: seasonNumbers,
      });
  };
  const hasWatchedShowEpisodes =
    (library.data?.progress?.watched_episodes ?? 0) > 0 ||
    episodeEntries.some((entry) => entry.watched);
  const showBulkAction = isSaved
    ? savedProgress.data?.is_fully_watched
      ? "unwatch"
      : "watch"
    : releasedShowEpisodes.length > 0
      ? "watch"
      : hasWatchedShowEpisodes
        ? "unwatch"
        : null;
  const seasonBulkAction =
    releasedSeasonEpisodes.length > 0 || (isSaved && episodes.hasNextPage)
      ? "watch"
      : watchedSeasonEpisodes.length > 0
        ? "unwatch"
        : null;
  const hasPreviousSeasons =
    pendingWatch?.seasonNumber !== undefined &&
    seasonGroups.some((group) => group.number > 0 && group.number < pendingWatch.seasonNumber!);

  return (
    <div className="detail-page">
      <Modal
        opened={movieHistoryMode !== null}
        onClose={() => setMovieHistoryMode(null)}
        title={movieHistoryMode === "edit" ? "Change watch date" : "Watch history"}
        size="lg"
        centered
      >
        {movieHistoryMode && showID && (
          <HistoryPanel
            key={movieHistoryMode}
            mediaID={showID}
            editLatest={movieHistoryMode === "edit"}
          />
        )}
      </Modal>
      <Modal
        opened={pendingWatch !== null}
        onClose={() => setPendingWatch(null)}
        title={
          pendingWatch?.target
            ? "Skipped episodes"
            : pendingWatch?.action === "unwatch"
              ? "Mark season unwatched"
              : "Mark season watched"
        }
        centered
      >
        {pendingWatch?.target ? (
          <Text mb="md">
            There are {pendingWatch.episodes.length} earlier unwatched episodes. How would you like
            to continue?
          </Text>
        ) : pendingWatch?.action === "unwatch" ? (
          <Text mb="md">
            Remove watch history for {pendingWatch?.episodes.length ?? 0} watched episodes in this
            season?
          </Text>
        ) : (
          <Text mb="md">
            Mark {pendingWatch?.episodes.length ?? 0} released episodes in this season as watched?
            {hasPreviousSeasons
              ? " Would you also like to mark previous seasons? Specials will not be included."
              : ""}
          </Text>
        )}
        {markEpisodesWatched.isError && (
          <Alert color="red" mb="md">
            {markEpisodesWatched.error.message}
          </Alert>
        )}
        {removeEpisodesWatched.isError && (
          <Alert color="red" mb="md">
            {removeEpisodesWatched.error.message}
          </Alert>
        )}
        {markEpisodeWatched.isError && (
          <Alert color="red" mb="md">
            {markEpisodeWatched.error.message}
          </Alert>
        )}
        <Group justify="flex-end">
          {pendingWatch?.target && (
            <Button
              variant="default"
              onClick={() => markEpisodeWatched.mutate(pendingWatch.target!.episode.id)}
              loading={markEpisodeWatched.isPending || markEpisodesWatched.isPending}
            >
              Only this episode
            </Button>
          )}
          <Button
            onClick={() => confirmWatch(Boolean(pendingWatch?.target))}
            loading={
              markEpisodeWatched.isPending ||
              markEpisodesWatched.isPending ||
              removeEpisodesWatched.isPending
            }
          >
            {pendingWatch?.target
              ? "Mark all as watched"
              : pendingWatch?.action === "unwatch"
                ? "Mark unwatched"
                : hasPreviousSeasons
                  ? "Only this season"
                  : "Mark season watched"}
          </Button>
          {hasPreviousSeasons && pendingWatch?.action !== "unwatch" && (
            <Button
              onClick={() => confirmWatch(false, true)}
              loading={markEpisodesWatched.isPending}
            >
              This and previous seasons
            </Button>
          )}
        </Group>
      </Modal>
      <Modal
        opened={showWatchModal}
        onClose={() => setShowWatchModal(false)}
        title={`${"Mark"} ${media.title} ${showWatchAction === "unwatch" ? "unwatched" : "watched"}`}
        centered
      >
        <Text size="sm" c="dimmed" mb="md">
          {showWatchAction === "unwatch"
            ? "Choose which seasons to remove from watch history. Specials are off by default."
            : "Choose the seasons to include. Specials are off by default."}
        </Text>
        <Stack gap="xs">
          {seasonGroups.map((group) => {
            return (
              <Paper key={group.number} withBorder p="sm">
                <Group justify="space-between">
                  <div>
                    <Text fw={650}>
                      {group.number === 0 ? "Specials" : `Season ${group.number}`}
                    </Text>
                    <Text size="xs" c="dimmed">
                      {isSaved
                        ? `${savedSeasons.data?.find((item) => item.season_number === group.number)?.episode_count ?? 0} episodes`
                        : "Episodes load when you confirm"}
                    </Text>
                  </div>
                  <Switch
                    aria-label={`Include ${group.number === 0 ? "specials" : `season ${group.number}`}`}
                    checked={selectedShowSeasons[group.number] === true}
                    onChange={(event) =>
                      setSelectedShowSeasons((current) => ({
                        ...current,
                        [group.number]: event.currentTarget.checked,
                      }))
                    }
                  />
                </Group>
              </Paper>
            );
          })}
        </Stack>
        {markEpisodesWatched.isError && (
          <Alert color="red" mt="md">
            {markEpisodesWatched.error.message}
          </Alert>
        )}
        {removeEpisodesWatched.isError && (
          <Alert color="red" mt="md">
            {removeEpisodesWatched.error.message}
          </Alert>
        )}
        <Group justify="flex-end" mt="lg">
          <Button variant="default" onClick={() => setShowWatchModal(false)}>
            Cancel
          </Button>
          <Button
            disabled={!Object.values(selectedShowSeasons).some(Boolean)}
            loading={markEpisodesWatched.isPending || removeEpisodesWatched.isPending}
            onClick={confirmShowAction}
          >
            {isSaved
              ? showWatchAction === "unwatch"
                ? "Mark selected seasons unwatched"
                : "Mark selected seasons watched"
              : "Mark selected seasons watched"}
          </Button>
        </Group>
      </Modal>
      <section className="detail-hero" style={{ backgroundImage: heroBackground }}>
        <Button
          className="detail-hero-back"
          variant="subtle"
          leftSection={<IconArrowLeft size={17} />}
          onClick={onBack}
        >
          Back
        </Button>
        <div className="detail-hero-content">
          <div className="detail-poster">
            {art ? (
              <Image src={art} alt={`${media.title} poster`} />
            ) : (
              <div className="artwork-fallback">{media.title.slice(0, 1)}</div>
            )}
          </div>
          <div className="detail-hero-body">
            <Group gap="xs">
              <Badge className="watch-kind" variant="filled">
                {media.type === "tv" ? "TV show" : "Movie"}
              </Badge>
              {media.type === "tv" && showStatusLabel(media.status) && (
                <Badge variant="light" color="gray">
                  {showStatusLabel(media.status)}
                </Badge>
              )}
            </Group>
            <Title order={1}>
              {selectedEpisode
                ? `S${String(selectedEpisode.episode.season_number).padStart(2, "0")}E${String(selectedEpisode.episode.episode_number).padStart(2, "0")} · ${selectedEpisode.name}`
                : media.title}
            </Title>
            {selectedEpisode ? (
              <Button
                className="detail-season-link"
                variant="subtle"
                onClick={() =>
                  onOpenDetail(
                    {
                      mediaType: "tv",
                      tmdbID: media.tmdb_id,
                      mediaID: showID,
                      seasonNumber: selectedEpisode.episode.season_number,
                    },
                    { from: returnTo, replace: true },
                  )
                }
              >{`${media.title} · Season ${selectedEpisode.episode.season_number}`}</Button>
            ) : (
              <Text className="detail-subtitle">{`${media.release_date ? media.release_date.slice(0, 4) : ""}${media.original_language ? ` · ${media.original_language.toUpperCase()}` : ""}`}</Text>
            )}
            <Text className="detail-overview">
              {selectedEpisode
                ? episodeDetails.data?.overview ||
                  selectedEpisode.overview ||
                  "Episode details are shown from your local catalog."
                : media.overview || "No description is available."}
            </Text>
            {(
              selectedEpisode
                ? episodeDetails.data?.runtime || selectedEpisode.runtime
                : movieRuntime
            ) ? (
              <Text className="detail-runtime" mt="sm">
                <IconClock size={15} />{" "}
                {selectedEpisode
                  ? episodeDetails.data?.runtime || selectedEpisode.runtime
                  : movieRuntime}{" "}
                min
              </Text>
            ) : null}
            {selectedEpisode && episodeDetails.data?.vote_average ? (
              <Text className="detail-runtime" mt="sm">
                TMDB {episodeDetails.data.vote_average.toFixed(1)} / 10
              </Text>
            ) : null}
            {selectedEpisode && episodeDetails.data?.air_date ? (
              <Text className="detail-runtime" mt="sm">
                Aired {episodeDetails.data.air_date}
                {episodeDetails.data.production_code
                  ? ` · ${episodeDetails.data.production_code}`
                  : ""}
              </Text>
            ) : null}
            {selectedEpisode ? (
              <EpisodeActions
                entry={selectedEpisode}
                canRate={isSaved}
                rating={episodeRating.data?.rating}
                onRate={(rating) => rateEpisode.mutate(rating)}
                onWatch={() => requestEpisodeWatch(selectedEpisode)}
                onRewatch={() => markEpisodeWatched.mutate(selectedEpisode.episode.id)}
                onUnwatch={() => removeEpisodesWatched.mutate([selectedEpisode.episode.id])}
                pending={
                  prepareEpisodeWatch.isPending ||
                  markEpisodeWatched.isPending ||
                  removeEpisodesWatched.isPending ||
                  rateEpisode.isPending
                }
              />
            ) : (
              <MediaActions
                media={media}
                isSaved={isSaved}
                status={status}
                rating={library.data?.item.rating}
                notificationsEnabled={library.data?.item.notifications_enabled ?? true}
                add={add}
                update={update}
                updateNotifications={updateNotifications}
                notificationMode={showNotificationMode}
                onOpenNotifications={() => setShowNotificationsOpen(true)}
                watched={Boolean(watchedPlay) || library.data?.item.status === "completed"}
                onWatch={() => markMovieWatched.mutate()}
                onUnwatch={() => removeMovieWatches.mutate()}
                onRemoveWatchlist={() => removeWatchlist.mutate()}
                onRemoveCurrentList={() =>
                  removeCurrentList.mutate(status as "watching" | "paused" | "dropped")
                }
                onViewWatchHistory={() => setMovieHistoryMode("view")}
                onChangeWatchDate={() => setMovieHistoryMode("edit")}
                showBulkAction={target.mediaType === "tv" ? showBulkAction : null}
                onShowBulkAction={() => {
                  if (showBulkAction === "watch") openShowWatchModal();
                  else if (showBulkAction === "unwatch") openShowActionModal("unwatch");
                }}
                showBulkPending={markEpisodesWatched.isPending || removeEpisodesWatched.isPending}
                pending={
                  markMovieWatched.isPending ||
                  removeMovieWatches.isPending ||
                  removeWatchlist.isPending ||
                  removeCurrentList.isPending
                }
              />
            )}
          </div>
        </div>
      </section>
      <Drawer
        opened={showNotificationsOpen}
        onClose={() => setShowNotificationsOpen(false)}
        position="bottom"
        title="Show notifications"
      >
        {updateShowNotificationMode.isError && (
          <Alert color="red" mb="sm">
            {updateShowNotificationMode.error.message}
          </Alert>
        )}
        <Radio.Group
          value={showNotificationMode}
          onChange={(value) => {
            if (value !== showNotificationMode) {
              updateShowNotificationMode.mutate(value as "episode" | "season");
            }
          }}
        >
          <Stack gap="md">
            <Radio
              value="episode"
              label="Every new episode"
              description="Notify me as new episodes of this show become available."
              disabled={updateShowNotificationMode.isPending}
            />
            <Radio
              value="season"
              label="Every full season"
              description="Notify me each time a season of this show is fully available."
              disabled={updateShowNotificationMode.isPending}
            />
          </Stack>
        </Radio.Group>
      </Drawer>
      {updateNotifications.isError && (
        <Alert color="red" mt="sm">
          {updateNotifications.error.message}
        </Alert>
      )}
      {update.isError && (
        <Alert color="red" mt="sm">
          {update.error.message}
        </Alert>
      )}
      {removeMovieWatches.isError && (
        <Alert color="red" mt="sm">
          {removeMovieWatches.error.message}
        </Alert>
      )}
      {removeWatchlist.isError && (
        <Alert color="red" mt="sm">
          {removeWatchlist.error.message}
        </Alert>
      )}
      {removeCurrentList.isError && (
        <Alert color="red" mt="sm">
          {removeCurrentList.error.message}
        </Alert>
      )}
      {markMovieWatched.isError && (
        <Alert color="red" mt="sm">
          {markMovieWatched.error.message}
        </Alert>
      )}
      {selectedEpisode && removeEpisodesWatched.isError && !pendingWatch && !showWatchModal && (
        <Alert color="red" mt="sm">
          {removeEpisodesWatched.error.message}
        </Alert>
      )}
      {selectedEpisode && prepareEpisodeWatch.isError && (
        <Alert color="red" mt="sm">
          {prepareEpisodeWatch.error.message}
        </Alert>
      )}
      {selectedEpisode && markEpisodeWatched.isError && (
        <Alert color="red" mt="sm">
          {markEpisodeWatched.error.message}
        </Alert>
      )}
      {selectedEpisode && markEpisodesWatched.isError && !pendingWatch && !showWatchModal && (
        <Alert color="red" mt="sm">
          {markEpisodesWatched.error.message}
        </Alert>
      )}
      {target.mediaType === "tv" && !selectedEpisode && (
        <section className="detail-section">
          <Group justify="space-between" align="end" mb="sm">
            <div>
              <Text className="section-kicker">{isSaved ? "Your catalog" : "From TMDB"}</Text>
              <Title order={2}>Seasons & episodes</Title>
            </div>
            <Group gap="xs">
              <Group gap="xs" wrap="nowrap">
                {seasons.length > 0 && (
                  <Select
                    aria-label="Season"
                    value={selectedSeason}
                    onChange={(value) => {
                      if (value !== null) {
                        onOpenDetail(
                          {
                            mediaType: "tv",
                            tmdbID: media.tmdb_id,
                            mediaID: showID,
                            seasonNumber: Number(value),
                          },
                          { from: returnTo, replace: true },
                        );
                      }
                    }}
                    data={seasons.map((number) => ({
                      value: String(number),
                      label: number === 0 ? "Specials" : `Season ${number}`,
                    }))}
                    w={150}
                  />
                )}
                {seasonBulkAction && (
                  <Tooltip
                    label={
                      seasonBulkAction === "watch" ? "Mark season watched" : "Mark season unwatched"
                    }
                    withArrow
                  >
                    <ActionIcon
                      size={44}
                      variant="subtle"
                      color="yellow"
                      aria-label={`Mark ${Number(selectedSeason) === 0 ? "specials" : `season ${selectedSeason}`} ${seasonBulkAction === "watch" ? "watched" : "unwatched"}`}
                      onClick={() => requestSeasonWatch(seasonBulkAction)}
                      disabled={markEpisodesWatched.isPending || removeEpisodesWatched.isPending}
                    >
                      {seasonBulkAction === "watch" ? (
                        <IconEye size={19} />
                      ) : (
                        <IconCheck size={19} />
                      )}
                    </ActionIcon>
                  </Tooltip>
                )}
              </Group>
            </Group>
          </Group>
          {((isSaved && episodes.isPending) ||
            (!isSaved && (temporary.isPending || temporaryEpisodes.isPending))) && (
            <Stack gap="xs">
              {Array.from({ length: 4 }).map((_, i) => (
                <Paper key={i} className="episode-row" withBorder p={0}>
                  <Group
                    className="episode-row-layout"
                    justify="space-between"
                    wrap="nowrap"
                    gap={0}
                  >
                    <Group className="episode-row-main" wrap="nowrap" gap={0}>
                      <div className="episode-art">
                        <Skeleton height="100%" width="100%" radius={0} />
                      </div>
                      <div className="episode-row-copy">
                        <Skeleton height={14} width={180} mb={8} />
                        <Skeleton height={12} width={110} mb={6} />
                        <Skeleton height={12} width={230} />
                      </div>
                    </Group>
                    <div className="episode-row-controls">
                      <Skeleton height={32} width={32} circle />
                    </div>
                  </Group>
                </Paper>
              ))}
            </Stack>
          )}
          {episodes.isError && isSaved && (
            <Alert color="red">Episodes are temporarily unavailable.</Alert>
          )}
          {temporary.isError && !isSaved && (
            <Alert color="red">TV details are temporarily unavailable.</Alert>
          )}
          {temporaryEpisodes.isError && !isSaved && (
            <Alert color="red">Season episodes are temporarily unavailable.</Alert>
          )}
          {markEpisodesWatched.isError && (
            <Alert color="red">{markEpisodesWatched.error.message}</Alert>
          )}
          {prepareEpisodeWatch.isError && (
            <Alert color="red">{prepareEpisodeWatch.error.message}</Alert>
          )}
          {markEpisodeWatched.isError && (
            <Alert color="red">{markEpisodeWatched.error.message}</Alert>
          )}
          {add.isError && <Alert color="red">{add.error.message}</Alert>}
          {(isSaved
            ? !episodes.isPending && !episodes.isError
            : !temporary.isPending &&
              !temporaryEpisodes.isPending &&
              !temporary.isError &&
              !temporaryEpisodes.isError) &&
            !visibleEpisodes.length && (
              <Text c="dimmed">Episode details are not available yet.</Text>
            )}
          <Stack gap="xs">
            {visibleEpisodes.map((entry) => {
              const openEpisode = () =>
                onOpenDetail({
                  mediaType: "tv",
                  tmdbID: media.tmdb_id,
                  mediaID: showID,
                  episodeID: entry.episode.id,
                  episode: entry.episode,
                  seasonNumber: entry.episode.season_number,
                });
              return (
                <Paper
                  key={entry.episode.id}
                  className="episode-row"
                  withBorder
                  p={0}
                  role="button"
                  tabIndex={0}
                  onClick={openEpisode}
                  onKeyDown={(event) => {
                    if (
                      (event.key === "Enter" || event.key === " ") &&
                      event.target === event.currentTarget
                    ) {
                      event.preventDefault();
                      openEpisode();
                    }
                  }}
                >
                  <Group
                    className="episode-row-layout"
                    justify="space-between"
                    wrap="nowrap"
                    gap={0}
                  >
                    <Group className="episode-row-main" wrap="nowrap" gap={0}>
                      <div className="episode-art">
                        {entry.still_path ? (
                          <Image src={backdropURL(entry.still_path, "w780")!} alt="" />
                        ) : (
                          <div className="artwork-fallback">{entry.episode.episode_number}</div>
                        )}
                      </div>
                      <div className="episode-row-copy">
                        <Text
                          fw={650}
                        >{`Episode ${entry.episode.episode_number}${entry.name ? ` · ${entry.name}` : ""}`}</Text>
                        <Text size="xs" c="dimmed">
                          {entry.episode.air_date || "Air date not announced"}
                        </Text>
                        {entry.overview && (
                          <Text className="episode-description" size="sm" c="dimmed" mt={5}>
                            {entry.overview}
                          </Text>
                        )}
                      </div>
                    </Group>
                    <Group
                      className="episode-row-controls"
                      gap="xs"
                      wrap="nowrap"
                      onClick={(event) => event.stopPropagation()}
                    >
                      <Tooltip
                        label={`Mark episode ${entry.episode.episode_number} ${entry.watched ? "unwatched" : "watched"}`}
                        withArrow
                      >
                        <Checkbox
                          aria-label={`Episode ${entry.episode.episode_number} watched`}
                          checked={entry.watched}
                          color="yellow"
                          size="md"
                          disabled={
                            prepareEpisodeWatch.isPending ||
                            markEpisodeWatched.isPending ||
                            removeEpisodesWatched.isPending
                          }
                          onChange={() => {
                            if (entry.watched) removeEpisodesWatched.mutate([entry.episode.id]);
                            else requestEpisodeWatch(entry);
                          }}
                        />
                      </Tooltip>
                      {entry.watched && (
                        <Menu withinPortal position="bottom-end">
                          <Menu.Target>
                            <ActionIcon
                              aria-label={`More actions for episode ${entry.episode.episode_number}`}
                              size="sm"
                              variant="subtle"
                            >
                              <IconChevronDown size={16} />
                            </ActionIcon>
                          </Menu.Target>
                          <Menu.Dropdown>
                            <Menu.Item
                              leftSection={<IconRefresh size={15} />}
                              onClick={() => markEpisodeWatched.mutate(entry.episode.id)}
                            >
                              Rewatch episode
                            </Menu.Item>
                          </Menu.Dropdown>
                        </Menu>
                      )}
                    </Group>
                  </Group>
                </Paper>
              );
            })}
            {isSaved && (
              <InfiniteScrollTrigger
                hasNextPage={!!episodes.hasNextPage}
                isFetchingNextPage={episodes.isFetchingNextPage}
                isFetchNextPageError={episodes.isFetchNextPageError}
                fetchNextPage={() => void episodes.fetchNextPage()}
              />
            )}
          </Stack>
        </section>
      )}
      {target.mediaType === "movie" && !selectedEpisode && movieGenres?.length ? (
        <section className="detail-section">
          <Text className="section-kicker">About this film</Text>
          <Group gap="xs">
            {movieGenres.map((genre) => (
              <Badge key={genre} variant="light">
                {genre}
              </Badge>
            ))}
            {movieScore ? <Badge variant="light">TMDB {movieScore.toFixed(1)} / 10</Badge> : null}
          </Group>
        </section>
      ) : null}
      {!selectedEpisode && (
        <CastSection
          members={
            target.mediaType === "tv"
              ? isSaved
                ? (library.data?.cast ?? [])
                : (temporary.data?.cast ?? [])
              : isSaved
                ? (library.data?.cast ?? [])
                : (movieDetails.data?.cast ?? [])
          }
          title="Cast"
          kicker="People"
          onOpenPerson={onOpenPerson}
        />
      )}
      {!target.episodeID &&
        (related.isPending || related.isError || Boolean(related.data?.length)) && (
          <section className="detail-section">
            <Text className="section-kicker">More to explore</Text>
            <Title order={2}>More like this</Title>
            {related.isPending && (
              <PosterGridSkeleton count={6} className="poster-grid related-media-grid" />
            )}
            {related.isError && (
              <Alert color="yellow" mt="sm">
                Related titles are temporarily unavailable.
              </Alert>
            )}
            {related.data?.length ? (
              <div className="poster-grid related-media-grid">
                {related.data.map((item) => (
                  <MediaPosterCard
                    key={`${item.type}:${item.tmdb_id}`}
                    media={item}
                    onOpenDetail={onOpenDetail}
                  />
                ))}
              </div>
            ) : null}
          </section>
        )}
      {selectedEpisode && (
        <CastSection
          members={episodeDetails.data?.guest_stars ?? []}
          title="Guest stars"
          kicker="Episode cast"
          fallbackRole="Guest star"
          onOpenPerson={onOpenPerson}
        />
      )}
      {selectedEpisode && episodeDetails.data?.crew?.length ? (
        <section className="detail-section">
          <Text className="section-kicker">Episode crew</Text>
          <Group gap="xs">
            {episodeDetails.data.crew
              .filter((member) => ["Director", "Writer", "Screenplay"].includes(member.job))
              .slice(0, 8)
              .map((member) => (
                <Badge key={`${member.id}-${member.job}`} variant="light">
                  {member.job}: {member.name}
                </Badge>
              ))}
          </Group>
        </section>
      ) : null}
      {!isSaved && (
        <Text className="detail-hint" c="dimmed">
          This is a temporary preview. Mark an episode watched to add the show to Watching.
        </Text>
      )}
    </div>
  );
}
