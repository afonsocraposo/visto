import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDebouncedValue } from "@mantine/hooks";
import { ActionIcon, Alert, Group, Image, Paper, Text, TextInput, Title, Tooltip } from "@mantine/core";
import { IconBookmark, IconEye, IconEyeCheck, IconSearch } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { LibraryEntry, MediaDetailTarget, SearchMedia } from "../../types";
import { posterURL } from "../../lib/artwork";

export function SearchPanel({ onOpenDetail }: { onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [query, setQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(query.trim(), 300);
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
      {results.data?.map(item => {
        const mediaID = `${item.type}:${item.tmdb_id}`;
        const saved = libraryIDs.has(mediaID);
        return (
          <Paper className="search-result-card search-result-clickable" key={`${item.type}-${item.tmdb_id}`} withBorder p="sm" mt="sm" role={onOpenDetail ? "link" : undefined} tabIndex={onOpenDetail ? 0 : undefined} aria-label={onOpenDetail ? `Open details for ${item.title}` : undefined} onClick={() => onOpenDetail?.({ mediaType: item.type, tmdbID: item.tmdb_id, seed: item })} onKeyDown={event => { if (onOpenDetail && (event.key === "Enter" || event.key === " ")) onOpenDetail({ mediaType: item.type, tmdbID: item.tmdb_id, seed: item }); }}>
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
              <Group className="search-result-actions" gap="xs" onClick={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()}>
                {saved ? <Tooltip label="Already in your library" withArrow><ActionIcon variant="light" color="teal" aria-label={`${item.title} is in your library`}><IconEyeCheck size={17} /></ActionIcon></Tooltip> : <>
                  <Tooltip label={item.type === "tv" ? "Add to watching" : "Mark watched"} withArrow>
                    <ActionIcon color="yellow" variant="filled" aria-label={item.type === "tv" ? `Add ${item.title} to watching` : `Mark ${item.title} watched`} onClick={() => item.type === "tv" ? addToLibrary.mutate({ media: item, status: "watching" }) : addMovieAsWatched.mutate(item)} loading={addToLibrary.isPending || addMovieAsWatched.isPending}>
                      <IconEye size={17} />
                    </ActionIcon>
                  </Tooltip>
                  <Tooltip label="Save for later" withArrow>
                    <ActionIcon variant="default" aria-label={`Save ${item.title} for later`} onClick={() => addToLibrary.mutate({ media: item, status: "watchlist" })} loading={addToLibrary.isPending}>
                      <IconBookmark size={17} />
                    </ActionIcon>
                  </Tooltip>
                </>}
              </Group>
            </Group>
          </Paper>
        );
      })}
    </>
  );
}
