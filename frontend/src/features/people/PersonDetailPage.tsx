import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Alert,
  Badge,
  Button,
  Group,
  Image,
  Loader,
  Paper,
  SimpleGrid,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { IconArrowLeft, IconMapPin } from "@tabler/icons-react";
import { MediaPosterCard } from "../../components/MediaPosterCard";
import { api } from "../../lib/api";
import { posterURL } from "../../lib/artwork";
import { useUserQueryKey } from "../auth/SessionContext";
import type {
  MediaDetailTarget,
  PersonDetails as PersonDetailsData,
  SearchMedia,
} from "../../types";
import { sortPersonCredits } from "./personCredits";

type Props = {
  personID: number;
  onBack: () => void;
  onOpenDetail: (target: MediaDetailTarget) => void;
};

export function PersonDetailPage({ personID, onBack, onOpenDetail }: Props) {
  const queryKey = useUserQueryKey();
  const [bioExpanded, setBioExpanded] = useState(false);
  const person = useQuery({
    queryKey: queryKey("person-details", personID),
    queryFn: () =>
      api.get<PersonDetailsData>(`/api/v1/people/${personID}`, "Could not load actor details."),
    staleTime: 24 * 60 * 60 * 1000,
  });

  if (person.isPending)
    return (
      <Stack align="center" py="xl">
        <Loader aria-label="Loading actor details" />
      </Stack>
    );
  if (person.isError)
    return (
      <Stack>
        <Button
          className="detail-back"
          variant="subtle"
          leftSection={<IconArrowLeft size={17} />}
          onClick={onBack}
        >
          Back
        </Button>
        <Alert color="red">{person.error.message}</Alert>
      </Stack>
    );

  const data = person.data;
  const portrait = posterURL(data.profile_path, "w500");
  const credits = sortPersonCredits(data.credits);

  return (
    <div className="person-detail-page">
      <Button
        className="detail-back"
        variant="subtle"
        leftSection={<IconArrowLeft size={17} />}
        onClick={onBack}
      >
        Back
      </Button>
      <section className="person-hero">
        <Paper className="person-portrait" withBorder p={0}>
          {portrait ? (
            <Image src={portrait} alt={`Portrait of ${data.name}`} />
          ) : (
            <div className="person-portrait-fallback" aria-hidden="true">
              {data.name.slice(0, 1)}
            </div>
          )}
        </Paper>
        <div className="person-biography">
          {data.known_for_department && (
            <Badge color="yellow" variant="light">
              {data.known_for_department}
            </Badge>
          )}
          <Title order={1}>{data.name}</Title>
          {(data.birthday || data.deathday || data.place_of_birth) && (
            <Group className="person-facts" gap="md">
              {(data.birthday || data.deathday) && (
                <Text size="sm">
                  {data.birthday && `Born ${formatDate(data.birthday)}`}
                  {data.deathday && ` · Died ${formatDate(data.deathday)}`}
                </Text>
              )}
              {data.place_of_birth && (
                <Text size="sm">
                  <IconMapPin size={15} />
                  {data.place_of_birth}
                </Text>
              )}
            </Group>
          )}
          <Text className="person-bio-copy" lineClamp={bioExpanded ? undefined : 5}>
            {data.biography || "No biography is available for this person."}
          </Text>
          {data.biography.length > 280 && (
            <Button
              className="person-bio-toggle"
              variant="subtle"
              size="compact-sm"
              onClick={() => setBioExpanded((value) => !value)}
            >
              {bioExpanded ? "Show less" : "Read full bio"}
            </Button>
          )}
        </div>
      </section>

      <section className="person-filmography">
        <Group className="person-filmography-heading" justify="space-between" align="end">
          <div>
            <Text className="section-kicker">Movies and TV</Text>
            <Title order={2}>Filmography</Title>
          </div>
          <Text size="sm" c="dimmed">
            {credits.length} {credits.length === 1 ? "title" : "titles"} · sorted by popularity
          </Text>
        </Group>
        {credits.length ? (
          <SimpleGrid
            className="library-grid library-grid-full person-credit-grid"
            cols={{ base: 2, xs: 3, sm: 4, md: 5, lg: 6 }}
            spacing="md"
          >
            {credits.map((credit) => {
              const media: SearchMedia = {
                tmdb_id: credit.tmdb_id,
                type: credit.type,
                title: credit.title,
                original_title: credit.original_title || credit.title,
                overview: credit.overview ?? "",
                release_date: credit.release_date ?? "",
                poster_path: credit.poster_path ?? "",
                backdrop_path: credit.backdrop_path,
                original_language: credit.original_language ?? "",
              };
              const subtitle = `${credit.type === "tv" ? "TV show" : "Movie"}${credit.character ? ` · ${credit.character}` : ""}`;
              return (
                <MediaPosterCard
                  key={`${credit.type}:${credit.tmdb_id}`}
                  media={media}
                  subtitle={subtitle}
                  onOpenDetail={onOpenDetail}
                />
              );
            })}
          </SimpleGrid>
        ) : (
          <Text c="dimmed" mt="md">
            No movies or TV shows are listed for this person.
          </Text>
        )}
      </section>
    </div>
  );
}

function formatDate(value: string): string {
  const [year, month, day] = value.split("-").map(Number);
  if (!year || !month || !day) return value;
  return new Date(year, month - 1, day).toLocaleDateString(undefined, {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}
