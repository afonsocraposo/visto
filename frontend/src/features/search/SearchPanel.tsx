import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDebouncedValue } from "@mantine/hooks";
import { Alert, Button, Group, Image, Modal, Paper, Text, TextInput, Title } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { LibraryEntry, SearchMedia } from "../../types";

export function SearchPanel() {
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
    mutationFn: (media: SearchMedia) => api.post("/api/v1/library", { media, status: "watchlist" }, "Could not add this title."),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
  });
  const libraryIDs = new Set(library.data?.map(entry => entry.item.media_id) ?? []);

  return (
    <>
      <Title order={1}>Search</Title>
      <TextInput mt="md" label="Search TMDB" value={query} onChange={event => setQuery(event.currentTarget.value)} leftSection={<IconSearch size={16} />} />
      {results.isError && <Alert color="red" mt="md">Search is temporarily unavailable.</Alert>}
      {library.isError && <Alert color="red" mt="md">Could not check your library. You can still view search results.</Alert>}
      {addToLibrary.isError && <Alert color="red" mt="md">{addToLibrary.error.message}</Alert>}
      <Modal opened={selectedMedia !== null} onClose={() => setSelectedMedia(null)} title={selectedMedia?.title} centered>
        {selectedMedia && (
          <Group align="start" wrap="nowrap">
            {selectedMedia.poster_path && (
              <Image src={`https://image.tmdb.org/t/p/w342${selectedMedia.poster_path}`} alt={`${selectedMedia.title} poster`} w={112} radius="sm" />
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
              <Button
                mt="lg"
                disabled={libraryIDs.has(`${selectedMedia.type}:${selectedMedia.tmdb_id}`)}
                loading={addToLibrary.isPending}
                onClick={() => addToLibrary.mutate(selectedMedia)}
              >
                {libraryIDs.has(`${selectedMedia.type}:${selectedMedia.tmdb_id}`) ? "In library" : "Add to watchlist"}
              </Button>
            </div>
          </Group>
        )}
      </Modal>
      {results.data?.map(item => {
        const mediaID = `${item.type}:${item.tmdb_id}`;
        const saved = libraryIDs.has(mediaID);
        return (
          <Paper key={`${item.type}-${item.tmdb_id}`} withBorder p="md" mt="sm">
            <Group justify="space-between" align="start">
              <div>
                <Text fw={700}>{item.title}</Text>
                <Text c="dimmed">{item.type === "tv" ? "TV show" : "Movie"}{item.release_date ? ` · ${item.release_date.slice(0, 4)}` : ""}</Text>
              </div>
              <Group gap="xs">
                <Button size="xs" variant="default" onClick={() => setSelectedMedia(item)}>Details</Button>
                <Button size="xs" disabled={saved} onClick={() => addToLibrary.mutate(item)} loading={addToLibrary.isPending}>
                  {saved ? "In library" : "Add"}
                </Button>
              </Group>
            </Group>
          </Paper>
        );
      })}
    </>
  );
}
