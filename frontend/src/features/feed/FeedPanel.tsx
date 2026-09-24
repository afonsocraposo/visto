import { useInfiniteQuery } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, Paper, Text, Title } from "@mantine/core";
import { EmptyState } from "../../components/EmptyState";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { FeedItem } from "../../types";

export function FeedPanel() {
  const userQueryKey = useUserQueryKey();
  const feed = useInfiniteQuery({
    queryKey: userQueryKey("feed"),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const query = pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : "";
      return api.get<{ items: FeedItem[]; next_cursor: string | null }>(`/api/v1/feed${query}`, "The feed is temporarily unavailable.");
    },
    getNextPageParam: page => page.next_cursor ?? undefined,
  });
  if (feed.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (feed.isError) return <Alert color="red" mt="md">The feed is temporarily unavailable.</Alert>;
  const items = feed.data.pages.flatMap(page => page.items);
  if (items.length === 0) return <EmptyState title="No shared activity yet" />;

  return (
    <>
      <Title order={1}>Feed</Title>
      {items.map(item => (
        <Paper key={item.id} withBorder p="md" mt="sm">
          <Text>
            {item.display_name} {item.kind === "rewatch" ? "rewatched" : item.kind === "rating" ? "rated" : item.kind === "bulk_watch" ? `marked ${item.count} episodes of` : "watched"}{" "}
            <Text component="span" fw={700}>{item.title}</Text>
            {item.kind === "rating" && item.rating ? ` · ${"★".repeat(item.rating)}` : item.season_number ? ` · S${String(item.season_number).padStart(2, "0")}E${String(item.episode_number).padStart(2, "0")}` : ""}
          </Text>
        </Paper>
      ))}
      {feed.hasNextPage && (
        <Group justify="center" mt="md">
          <Button variant="light" onClick={() => void feed.fetchNextPage()} loading={feed.isFetchingNextPage}>
            Load more activity
          </Button>
        </Group>
      )}
      {feed.isFetchNextPageError && <Alert color="red" mt="md">Could not load more activity. Try again.</Alert>}
    </>
  );
}
