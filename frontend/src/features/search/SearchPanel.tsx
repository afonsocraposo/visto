import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDebouncedValue } from "@mantine/hooks";
import { Alert, Button, Group, Image, Modal, Paper, Text, TextInput, Title } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { LibraryEntry, MediaDetailTarget, SearchMedia } from "../../types";
import { posterURL } from "../../lib/artwork";

export function SearchPanel({ onOpenDetail }: { onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [query, setQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(query.trim(), 300);
  const [selectedMedia, setSelectedMedia] = useState<SearchMedia | null>(null);
  const library = useQuery({
    queryKey: userQueryKey("library"),
    queryFn: () => api.get<LibraryEntry[]>("/api/v1/library", "Could not load your library."),
  });
  const results = useQuery({
    queryKey: userQueryKey("search", debouncedQuery),
    enabled: debouncedQuery.length > 1,
    queryFn: () => api.get<SearchMedia[]>(`/api/v1/search?q=${encodeURIComponent(debouncedQuery)}`, "Search is temporarily unavailable."),
  });
  const addToLibrary = useMutation({
    mutationFn: ({ media, status }: { media: SearchMedia; status: "watching" | "watchlist" }) =>
      api.post("/api/v1/library", { media, status }, "Could not add this title."),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
    ]),
  });
  const addMovieAsWatched = useMutation({
    mutationFn: async (media: SearchMedia) => {
      await api.post("/api/v1/library", { media, status: "watching" }, "Could not add this title.");
      await api.post("/api/v1/plays", { media_id: `${media.type}:${media.tmdb_id}` }, "Could not record this watch.");
    },
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });
  const libraryIDs = new Set(library.data?.map(entry => entry.item.media_id) ?? []);

  return (
    <>
      <div className="page-heading search-heading"><Text className="section-kicker">Find your next thing</Text><Title order={1}>Search</Title></div>
      <TextInput className="search-input" mt="md" label="Search TMDB" placeholder="Try a show, movie, actor…" value={query} onChange={event => setQuery(event.currentTarget.value)} leftSection={<IconSearch size={18} />} />
      {results.isError && <Alert color="red" mt="md">Search is temporarily unavailable.</Alert>}
      {library.isError && <Alert color="red" mt="md">Could not check your library. You can still view search results.</Alert>}
      {addToLibrary.isError && <Alert color="red" mt="md">{addToLibrary.error.message}</Alert>}
      <Modal opened={selectedMedia !== null} onClose={() => setSelectedMedia(null)} title={selectedMedia?.title} centered>
        {selectedMedia && (
          <Group className="media-detail" align="start" wrap="nowrap">
            {selectedMedia.poster_path && (
              <Image src={posterURL(selectedMedia.poster_path, "w342")!} alt={`${selectedMedia.title} poster`} w={128} radius="md" />
            )}
            <div>
              <Text size="sm" c="dimmed">
                {selectedMedia.type === "tv" ? "TV show" : "Movie"}
                {selectedMedia.release_date ? ` · ${selectedMedia.release_date.slice(0, 4)}` : ""}
                {selectedMedia.original_language ? ` · ${selectedMedia.original_language.toUpperCase()}` : ""}
              </Text>
              {selectedMedia.original_title && selectedMedia.original_title !== selectedMedia.title && (
                <Text size="sm" mt="xs">Original title: {selectedMedia.original_title}</Text>
              )}
              <Text mt="md">{selectedMedia.overview || "No description is available."}</Text>
              {libraryIDs.has(`${selectedMedia.type}:${selectedMedia.tmdb_id}`) ? (
                <Button mt="lg" disabled>In library</Button>
              ) : selectedMedia.type === "tv" ? (
                <Group mt="lg" gap="xs">
                  <Button loading={addToLibrary.isPending} onClick={() => addToLibrary.mutate({ media: selectedMedia, status: "watching" })}>
                    Add to watching
                  </Button>
                  <Button variant="default" loading={addToLibrary.isPending} onClick={() => addToLibrary.mutate({ media: selectedMedia, status: "watchlist" })}>
                    Watch later
                  </Button>
                </Group>
              ) : (
                <Group mt="lg" gap="xs"><Button loading={addMovieAsWatched.isPending} onClick={() => addMovieAsWatched.mutate(selectedMedia)}>Mark watched</Button><Button variant="default" loading={addToLibrary.isPending} onClick={() => addToLibrary.mutate({ media: selectedMedia, status: "watchlist" })}>Watch later</Button></Group>
              )}
            </div>
          </Group>
        )}
      </Modal>
      {results.data?.map(item => {
        const mediaID = `${item.type}:${item.tmdb_id}`;
        const saved = libraryIDs.has(mediaID);
        return (
          <Paper className="search-result-card" key={`${item.type}-${item.tmdb_id}`} withBorder p="sm" mt="sm">
            <Group justify="space-between" align="start" wrap="nowrap">
              <Group align="flex-start" wrap="nowrap" gap="sm">
                <div className="search-poster">
                  {posterURL(item.poster_path, "w185") ? <Image src={posterURL(item.poster_path, "w185")!} alt={`${item.title} poster`} /> : <div className="artwork-fallback">{item.title.slice(0, 1)}</div>}
                </div>
                <div>
                  <Text fw={750}>{item.title}</Text>
                  <Text size="sm" c="dimmed">{item.type === "tv" ? "TV show" : "Movie"}{item.release_date ? ` · ${item.release_date.slice(0, 4)}` : ""}</Text>
                  {item.overview && <Text className="search-overview" size="sm" c="dimmed" mt={6}>{item.overview}</Text>}
                </div>
              </Group>
              <Group className="search-result-actions" gap="xs">
                <Button size="xs" variant="default" onClick={() => onOpenDetail ? onOpenDetail({ mediaType: item.type, tmdbID: item.tmdb_id, seed: item }) : setSelectedMedia(item)}>Details</Button>
                {saved ? <Button size="xs" disabled>In library</Button> : item.type === "tv" ? (
                  <>
                    <Button size="xs" onClick={() => addToLibrary.mutate({ media: item, status: "watching" })} loading={addToLibrary.isPending}>
                      Add to watching
                    </Button>
                    <Button size="xs" variant="default" onClick={() => addToLibrary.mutate({ media: item, status: "watchlist" })} loading={addToLibrary.isPending}>
                      Watch later
                    </Button>
                  </>
                ) : (
                  <>
                    <Button size="xs" onClick={() => addMovieAsWatched.mutate(item)} loading={addMovieAsWatched.isPending}>Mark watched</Button>
                    <Button size="xs" variant="default" onClick={() => addToLibrary.mutate({ media: item, status: "watchlist" })} loading={addToLibrary.isPending}>Watch later</Button>
                  </>
                )}
              </Group>
            </Group>
          </Paper>
        );
      })}
    </>
  );
}
