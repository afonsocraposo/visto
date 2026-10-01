import { useDebouncedValue } from "@mantine/hooks";
import { FadeImage } from "../../components/FadeImage";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { Alert, CloseButton, Group, Skeleton, Text, TextInput, Title } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { MediaQuickActions } from "../../components/MediaQuickActions";
import { MediaPosterCard } from "../../components/MediaPosterCard";
import { PosterGridSkeleton } from "../../components/PosterGridSkeleton";
import { EmptyState } from "../../components/EmptyState";
import { QueryError } from "../../components/QueryError";
import { DetailLink } from "../../components/DetailLink";
import { useDiscoverMutations, useDiscoverQueries } from "./queries";
import type { MediaDetailTarget } from "../../types";
import { posterURL } from "../../lib/artwork";

export function SearchPanel({
  onOpenDetail,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const query = useSearch({ from: "/discover" }).q ?? "";
  const navigate = useNavigate({ from: "/discover" });
  const trimmed = query.trim();
  const [debouncedQuery] = useDebouncedValue(trimmed, 350);
  const { library, results, trending } = useDiscoverQueries(debouncedQuery);
  const searchResults = results.data ?? [];
  const { addToLibrary, addMovieAsWatched, markWatchlistMovieWatched } = useDiscoverMutations();
  const libraryEntries = new Map(library.data?.map((entry) => [entry.item.media_id, entry]) ?? []);
  // Trending leaves as soon as there is a query; results show a skeleton until they arrive.
  const searching = trimmed.length > 0;
  const waitingForResults =
    searching &&
    trimmed.length > 1 &&
    (debouncedQuery !== trimmed || (results.isFetching && !results.isSuccess) || results.isPending);

  return (
    <>
      <div className="page-heading search-heading">
        <Title order={1}>Discover</Title>
      </div>
      <TextInput
        className="search-input"
        size="lg"
        radius="md"
        aria-label="Search TMDB"
        placeholder="Search shows, movies, people…"
        value={query}
        onChange={(event) =>
          void navigate({
            to: "/discover",
            search: { q: event.currentTarget.value || undefined },
            replace: true,
            resetScroll: false,
          })
        }
        leftSection={<IconSearch size={20} />}
        rightSection={
          query ? (
            <CloseButton
              aria-label="Clear search"
              onClick={() =>
                void navigate({ to: "/discover", search: {}, replace: true, resetScroll: false })
              }
            />
          ) : undefined
        }
      />
      {!searching && trending.isPending && (
        <div className="trending-sections">
          {[0, 1].map((i) => (
            <section className="trending-section" key={i}>
              <Group justify="space-between" mb="sm">
                <Skeleton height={22} width={160} />
                <Skeleton height={14} width={70} />
              </Group>
              <PosterGridSkeleton count={10} caption />
            </section>
          ))}
        </div>
      )}
      {!searching && trending.isError && (
        <QueryError
          message="Trending titles are temporarily unavailable. You can still search TMDB."
          onRetry={() => trending.refetch()}
        />
      )}
      {!searching && trending.data && (
        <div className="trending-sections content-ready">
          {[
            { title: "Trending TV shows", items: trending.data.tv },
            { title: "Trending movies", items: trending.data.movies },
          ].map(
            (section) =>
              section.items.length > 0 && (
                <section
                  key={section.title}
                  className="trending-section"
                  aria-labelledby={`heading-${section.title}`}
                >
                  <Group justify="space-between" align="baseline" mb="sm">
                    <Title id={`heading-${section.title}`} order={2} size="h3">
                      {section.title}
                    </Title>
                    <Text size="sm" c="dimmed">
                      This week
                    </Text>
                  </Group>
                  <div className="poster-grid">
                    {section.items.slice(0, 12).map((item, index) => (
                      <MediaPosterCard
                        key={`${item.type}-${item.tmdb_id}`}
                        media={item}
                        variant="caption"
                        eager={index < 6}
                        onOpenDetail={onOpenDetail}
                      />
                    ))}
                  </div>
                </section>
              ),
          )}
        </div>
      )}
      {searching && trimmed.length === 1 && (
        <Text c="dimmed" size="sm" mt="md">
          Keep typing to search.
        </Text>
      )}
      {waitingForResults && (
        <div className="search-results" aria-busy="true" aria-label="Loading results">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="search-result-card">
              <Skeleton className="search-poster" radius="sm" />
              <div style={{ flex: 1 }}>
                <Skeleton height={16} width="45%" mb={8} />
                <Skeleton height={12} width="30%" />
              </div>
            </div>
          ))}
        </div>
      )}
      {searching && !waitingForResults && results.isError && (
        <QueryError
          message="Search is temporarily unavailable."
          onRetry={() => results.refetch()}
        />
      )}
      {searching && !waitingForResults && results.isSuccess && searchResults.length === 0 && (
        <EmptyState
          icon={<IconSearch size={20} />}
          title={`No results for “${debouncedQuery}”`}
          detail="Check the spelling or try the original title."
        />
      )}
      {library.isError && (
        <Alert color="red" mt="md">
          Could not check your library. You can still view search results.
        </Alert>
      )}
      {addToLibrary.isError && (
        <Alert color="red" mt="md">
          {addToLibrary.error.message}
        </Alert>
      )}
      {addMovieAsWatched.isError && (
        <Alert color="red" mt="md">
          {addMovieAsWatched.error.message}
        </Alert>
      )}
      {markWatchlistMovieWatched.isError && (
        <Alert color="red" mt="md">
          {markWatchlistMovieWatched.error.message}
        </Alert>
      )}
      {searching && !waitingForResults && (
        <div key={debouncedQuery} className="search-results content-ready">
          {searchResults.map((item) => {
            const mediaID = `${item.type}:${item.tmdb_id}`;
            const savedEntry = libraryEntries.get(mediaID);
            const savedLabel =
              savedEntry?.item.status === "completed"
                ? item.type === "movie"
                  ? "Watched"
                  : "Completed"
                : savedEntry?.item.status === "watchlist"
                  ? "In Watchlist"
                  : savedEntry?.item.status === "watching"
                    ? item.type === "movie"
                      ? "Watched"
                      : "In Watching"
                    : savedEntry?.item.status === "paused"
                      ? "Paused"
                      : savedEntry?.item.status === "dropped"
                        ? "Dropped"
                        : "In your library";
            const art = posterURL(item.poster_path, "w185");
            return (
              <article className="search-result-card" key={`${item.type}-${item.tmdb_id}`}>
                <div className="search-poster" aria-hidden="true">
                  {art ? (
                    <FadeImage src={art} alt="" />
                  ) : (
                    <div className="artwork-fallback">{item.title.slice(0, 1)}</div>
                  )}
                </div>
                <div className="search-result-copy">
                  <DetailLink
                    className="search-result-title"
                    aria-label={`Open details for ${item.title}`}
                    to={{ mediaType: item.type, tmdbID: item.tmdb_id, seed: item }}
                    onOpen={onOpenDetail}
                  >
                    {item.title}
                  </DetailLink>
                  <Text size="sm" c="dimmed">
                    {item.type === "tv" ? "TV show" : "Movie"}
                    {item.release_date ? ` · ${item.release_date.slice(0, 4)}` : ""}
                  </Text>
                  {item.overview && (
                    <Text className="search-overview" size="sm" c="dimmed" mt={6}>
                      {item.overview}
                    </Text>
                  )}
                </div>
                <div className="search-result-actions">
                  <MediaQuickActions
                    media={item}
                    saved={Boolean(savedEntry)}
                    savedLabel={savedLabel}
                    canMarkSavedWatched={
                      item.type === "movie" && savedEntry?.item.status === "watchlist"
                    }
                    loadingAction={
                      (addToLibrary.isPending &&
                        addToLibrary.variables?.media.type === item.type &&
                        addToLibrary.variables?.media.tmdb_id === item.tmdb_id &&
                        addToLibrary.variables.status === "watching") ||
                      (addMovieAsWatched.isPending &&
                        addMovieAsWatched.variables?.tmdb_id === item.tmdb_id)
                        ? "watch"
                        : addToLibrary.isPending &&
                            addToLibrary.variables?.media.type === item.type &&
                            addToLibrary.variables?.media.tmdb_id === item.tmdb_id &&
                            addToLibrary.variables.status === "watchlist"
                          ? "watchlist"
                          : markWatchlistMovieWatched.isPending &&
                              markWatchlistMovieWatched.variables?.media.tmdb_id === item.tmdb_id
                            ? "mark-watched"
                            : undefined
                    }
                    disabled={
                      addToLibrary.isPending ||
                      addMovieAsWatched.isPending ||
                      markWatchlistMovieWatched.isPending
                    }
                    onWatch={() =>
                      item.type === "tv"
                        ? addToLibrary.mutate({ media: item, status: "watching" })
                        : addMovieAsWatched.mutate(item)
                    }
                    onWatchlist={() => addToLibrary.mutate({ media: item, status: "watchlist" })}
                    onMarkSavedWatched={() =>
                      markWatchlistMovieWatched.mutate({
                        media: item,
                        rating: savedEntry?.item.rating ?? null,
                      })
                    }
                  />
                </div>
              </article>
            );
          })}
        </div>
      )}
    </>
  );
}
