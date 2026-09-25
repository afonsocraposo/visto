import { useState } from "react";
import { useDebouncedValue } from "@mantine/hooks";
import { Alert, Group, Image, Loader, Paper, Text, TextInput, Title } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { MediaQuickActions } from "../../components/MediaQuickActions";
import { MediaPosterCard } from "../../components/MediaPosterCard";
import { useDiscoverMutations, useDiscoverQueries } from "./queries";
import type { MediaDetailTarget } from "../../types";
import { posterURL } from "../../lib/artwork";

export function SearchPanel({
  onOpenDetail,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const [query, setQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(query.trim(), 300);
  const { library, results, trending } = useDiscoverQueries(debouncedQuery);
  const { addToLibrary, addMovieAsWatched } = useDiscoverMutations();
  const libraryIDs = new Set(library.data?.map((entry) => entry.item.media_id) ?? []);

  return (
    <>
      <div className="page-heading search-heading">
        <Text className="section-kicker">Find your next thing</Text>
        <Title order={1}>Search</Title>
      </div>
      <TextInput
        className="search-input"
        mt="md"
        label="Search TMDB"
        placeholder="Try a show, movie, actor…"
        value={query}
        onChange={(event) => setQuery(event.currentTarget.value)}
        leftSection={<IconSearch size={18} />}
      />
      {!debouncedQuery && trending.isPending && (
        <Group justify="center" py="xl">
          <Loader size="sm" />
        </Group>
      )}
      {!debouncedQuery && trending.isError && (
        <Alert color="yellow" mt="md">
          Trending titles are temporarily unavailable. You can still search TMDB.
        </Alert>
      )}
      {!debouncedQuery && trending.data && (
        <div className="trending-sections">
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
                  <div className="trending-grid">
                    {section.items.slice(0, 10).map((item) => (
                      <MediaPosterCard
                        key={`${item.type}-${item.tmdb_id}`}
                        media={item}
                        onOpenDetail={onOpenDetail}
                      />
                    ))}
                  </div>
                </section>
              ),
          )}
        </div>
      )}
      {results.isError && (
        <Alert color="red" mt="md">
          Search is temporarily unavailable.
        </Alert>
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
      {results.data?.map((item) => {
        const mediaID = `${item.type}:${item.tmdb_id}`;
        const saved = libraryIDs.has(mediaID);
        return (
          <Paper
            className="search-result-card search-result-clickable"
            key={`${item.type}-${item.tmdb_id}`}
            withBorder
            p="sm"
            mt="sm"
            role={onOpenDetail ? "link" : undefined}
            tabIndex={onOpenDetail ? 0 : undefined}
            aria-label={onOpenDetail ? `Open details for ${item.title}` : undefined}
            onClick={() =>
              onOpenDetail?.({ mediaType: item.type, tmdbID: item.tmdb_id, seed: item })
            }
            onKeyDown={(event) => {
              if (onOpenDetail && (event.key === "Enter" || event.key === " "))
                onOpenDetail({ mediaType: item.type, tmdbID: item.tmdb_id, seed: item });
            }}
          >
            <Group justify="space-between" align="start" wrap="nowrap">
              <Group align="flex-start" wrap="nowrap" gap="sm">
                <div className="search-poster">
                  {posterURL(item.poster_path, "w185") ? (
                    <Image
                      src={posterURL(item.poster_path, "w185")!}
                      alt={`${item.title} poster`}
                    />
                  ) : (
                    <div className="artwork-fallback">{item.title.slice(0, 1)}</div>
                  )}
                </div>
                <div>
                  <Text fw={750}>{item.title}</Text>
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
              </Group>
              <MediaQuickActions
                media={item}
                saved={saved}
                busy={addToLibrary.isPending || addMovieAsWatched.isPending}
                onWatch={() =>
                  item.type === "tv"
                    ? addToLibrary.mutate({ media: item, status: "watching" })
                    : addMovieAsWatched.mutate(item)
                }
                onWatchlist={() => addToLibrary.mutate({ media: item, status: "watchlist" })}
              />
            </Group>
          </Paper>
        );
      })}
    </>
  );
}
