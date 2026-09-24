import { useQuery } from "@tanstack/react-query";
import { Alert, Group, Loader, Paper, Text, Title } from "@mantine/core";
import { EmptyState } from "../../components/EmptyState";
import type { FeedItem } from "../../types";

export function FeedPanel() {
  const feed = useQuery({
    queryKey: ["feed"],
    queryFn: async () => {
      const response = await fetch("/api/v1/feed");
      if (!response.ok) throw new Error();
      return response.json() as Promise<{ items: FeedItem[] }>;
    },
  });
  if (feed.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (feed.isError) return <Alert color="red" mt="md">The feed is temporarily unavailable.</Alert>;
  if (!feed.data?.items.length) return <EmptyState title="No shared activity yet" />;

  return (
    <>
      <Title order={1}>Feed</Title>
      {feed.data.items.map(item => (
        <Paper key={item.id} withBorder p="md" mt="sm">
          <Text>
            {item.display_name} {item.kind === "rewatch" ? "rewatched" : item.kind === "rating" ? "rated" : item.kind === "bulk_watch" ? `marked ${item.count} episodes of` : "watched"}{" "}
            <Text component="span" fw={700}>{item.title}</Text>
            {item.kind === "rating" && item.rating ? ` · ${"★".repeat(item.rating)}` : item.season_number ? ` · S${String(item.season_number).padStart(2, "0")}E${String(item.episode_number).padStart(2, "0")}` : ""}
          </Text>
        </Paper>
      ))}
    </>
  );
}
