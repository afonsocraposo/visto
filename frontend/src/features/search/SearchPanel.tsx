import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Paper, Text, TextInput, Title } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import type { LibraryEntry, SearchMedia } from "../../types";

export function SearchPanel() {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [query, setQuery] = useState("");
  const library = useQuery({
    queryKey: userQueryKey("library"),
    queryFn: async () => {
      const response = await fetch("/api/v1/library");
      if (!response.ok) throw new Error();
      return response.json() as Promise<LibraryEntry[]>;
    },
  });
  const results = useQuery({
    queryKey: userQueryKey("search", query),
    enabled: query.trim().length > 1,
    queryFn: async () => {
      const response = await fetch(`/api/v1/search?q=${encodeURIComponent(query)}`);
      if (!response.ok) throw new Error();
      return response.json() as Promise<SearchMedia[]>;
    },
  });
  const addToLibrary = useMutation({
    mutationFn: async (media: SearchMedia) => {
      const response = await fetch("/api/v1/library", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ media, status: "watchlist" }),
      });
      if (!response.ok) throw new Error("Could not add this title.");
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
  });
  const libraryIDs = new Set(library.data?.map(entry => entry.item.media_id) ?? []);

  return (
    <>
      <Title order={1}>Search</Title>
      <TextInput mt="md" label="Search TMDB" value={query} onChange={event => setQuery(event.currentTarget.value)} leftSection={<IconSearch size={16} />} />
      {results.isError && <Alert color="red" mt="md">Search is temporarily unavailable.</Alert>}
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
              <Button size="xs" disabled={saved} onClick={() => addToLibrary.mutate(item)} loading={addToLibrary.isPending}>
                {saved ? "In library" : "Add"}
              </Button>
            </Group>
          </Paper>
        );
      })}
    </>
  );
}
