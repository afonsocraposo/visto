import { Anchor, Group, Text, Tooltip } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import type { ExternalRatings as Scores } from "../../generated/models/externalRatings";
import { api } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";

export function ExternalRatings({
  mediaType,
  tmdbID,
  title,
}: {
  mediaType: "movie" | "tv";
  tmdbID: number;
  title: string;
}) {
  const userQueryKey = useUserQueryKey();
  const scores = useQuery({
    queryKey: userQueryKey("external-ratings", mediaType, tmdbID),
    queryFn: () => api.get<Scores>(`/api/v1/media/${mediaType}/${tmdbID}/ratings`),
    staleTime: 60 * 60 * 1000,
    retry: false,
  });
  const data = scores.data;
  if (!data?.enabled || (data.imdb === undefined && data.rotten_tomatoes === undefined))
    return null;
  const updated = data.fetched_at
    ? `Updated ${new Date(data.fetched_at).toLocaleDateString()}`
    : "";
  return (
    <Group gap="sm" className="detail-external-ratings" aria-label="External ratings">
      {data.imdb !== undefined && (
        <Tooltip label={`IMDb. ${updated}`}>
          <Anchor
            size="sm"
            c="inherit"
            className="detail-external-score"
            aria-label={`IMDb ${data.imdb.toFixed(1)}/10`}
            href={`https://www.imdb.com/title/${encodeURIComponent(data.imdb_id ?? "")}/`}
            target="_blank"
            rel="noopener noreferrer"
          >
            <img src="/imdb.svg" width={22} height={22} alt="" aria-hidden="true" />
            {data.imdb.toFixed(1)}/10
          </Anchor>
        </Tooltip>
      )}
      {data.rotten_tomatoes !== undefined && (
        <Tooltip label={`Rotten Tomatoes. ${updated}. Critics score.`}>
          <Anchor
            size="sm"
            c="inherit"
            className="detail-external-score"
            aria-label={`Rotten Tomatoes ${data.rotten_tomatoes}%`}
            href={`https://www.rottentomatoes.com/search?search=${encodeURIComponent(title)}`}
            target="_blank"
            rel="noopener noreferrer"
          >
            <img src="/rottentomatoes.svg" width={22} height={22} alt="" aria-hidden="true" />
            {data.rotten_tomatoes}%
          </Anchor>
        </Tooltip>
      )}
      <Text size="xs" c="inherit">
        via{" "}
        <Anchor
          size="xs"
          c="inherit"
          href="https://www.omdbapi.com/"
          target="_blank"
          rel="noopener noreferrer"
        >
          OMDb
        </Anchor>
      </Text>
    </Group>
  );
}
