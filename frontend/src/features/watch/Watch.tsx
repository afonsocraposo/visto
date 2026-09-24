import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ActionIcon, Alert, Button, Group, Image, Loader, Modal, Paper, Text, Title, Tooltip } from "@mantine/core";
import { IconEye } from "@tabler/icons-react";
import { EmptyState } from "../../components/EmptyState";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { calendarMonthRange, dateInTimezone, formatCalendarDate, formatCalendarMonth, groupCalendarEntries, monthInTimezone, shiftCalendarMonth } from "./calendar";
import type { CalendarEntry, ContinueEntry, MediaDetailTarget } from "../../types";
import { backdropURL, posterURL } from "../../lib/artwork";

export function WatchNow({ onOpenDetail }: { onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [confirmation, setConfirmation] = useState<ContinueEntry | null>(null);
  const entries = useQuery({
    queryKey: userQueryKey("continue"),
    queryFn: () => api.get<ContinueEntry[]>("/api/v1/continue-watching", "Watch data is temporarily unavailable."),
  });
  const markWatched = useMutation({
    mutationFn: ({ episodeIDs, bulk }: { episodeIDs: string[]; bulk: boolean }) =>
      api.post(bulk ? "/api/v1/plays/bulk" : "/api/v1/plays", bulk ? { episode_ids: episodeIDs } : { episode_id: episodeIDs[0] }, "Could not mark episode watched."),
    onSuccess: async () => {
      setConfirmation(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("show-progress") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      ]);
    },
  });

  if (entries.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (entries.isError) return <Alert color="red" mt="md">Watch data is temporarily unavailable.</Alert>;
  if (!entries.data?.length) return <EmptyState title="Nothing to continue yet" detail="Add a show to Watching to see the next released episode here." />;

  return (
    <>
      <Modal opened={confirmation !== null} onClose={() => setConfirmation(null)} title="Skipped episodes" centered>
        <Text mb="md">You have {confirmation?.missing_prior_episodes?.length} earlier unplayed episodes of {confirmation?.title}. How would you like to continue?</Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={() => confirmation?.next_episode && markWatched.mutate({ episodeIDs: [confirmation.next_episode.id], bulk: false })} loading={markWatched.isPending}>Only this episode</Button>
          <Button onClick={() => confirmation?.next_episode && markWatched.mutate({ episodeIDs: [...(confirmation.missing_prior_episodes || []).map(episode => episode.id), confirmation.next_episode.id], bulk: true })} loading={markWatched.isPending}>Mark all as watched</Button>
        </Group>
      </Modal>
      <div className="watch-intro">
        <Text className="section-kicker">Watching</Text>
        <Title order={1}>Episodes to watch</Title>
      </div>
      <div className="watch-list">
        {entries.data.map(entry => {
          const art = backdropURL(entry.next_episode_still_path, "w780") ?? posterURL(entry.poster_path, "w500");
          const openEpisode = () => onOpenDetail?.({ mediaType: "tv", tmdbID: Number(entry.show_id.split(":")[1]), mediaID: entry.show_id, episodeID: entry.next_episode?.id, episode: entry.next_episode });
          return <Paper key={entry.show_id} className="watch-row" withBorder p={0} role={onOpenDetail ? "button" : undefined} tabIndex={onOpenDetail ? 0 : undefined} onClick={openEpisode} onKeyDown={event => { if ((event.key === "Enter" || event.key === " ") && event.target === event.currentTarget) { event.preventDefault(); openEpisode(); } }}>
            <div className="watch-row-art">{art ? <Image src={art} alt="" /> : <div className="artwork-fallback">{entry.title.slice(0, 1)}</div>}</div>
            <div className="watch-row-content">
              <Text className="watch-row-title" fw={700} lineClamp={1}>{entry.title}</Text>
              <Text className="watch-row-episode" lineClamp={1}>{entry.next_episode ? `S${String(entry.next_episode.season_number).padStart(2, "0")} | E${String(entry.next_episode.episode_number).padStart(2, "0")}${entry.next_episode_name ? ` · ${entry.next_episode_name}` : ""}` : "Episode details are pending"}</Text>
            </div>
            {entry.next_episode && <Tooltip label="Mark episode watched" withArrow><ActionIcon className="watch-row-action" size="xl" radius="xl" variant="light" color="gray" aria-label={`Mark ${entry.title} season ${entry.next_episode.season_number}, episode ${entry.next_episode.episode_number} watched`} loading={markWatched.isPending} onClick={event => { event.stopPropagation(); entry.missing_prior_episodes?.length ? setConfirmation(entry) : markWatched.mutate({ episodeIDs: [entry.next_episode!.id], bulk: false }); }}><IconEye size={22} stroke={1.8} /></ActionIcon></Tooltip>}
          </Paper>;
        })}
      </div>
    </>
  );
}

export function WatchCalendar() {
  const userQueryKey = useUserQueryKey();
  const [selectedMonth, setSelectedMonth] = useState<string | null>(null);
  const settings = useQuery({
    queryKey: userQueryKey("profile-settings"),
    queryFn: () => api.get<{ timezone: string }>("/api/v1/profile/activity-settings", "Calendar settings are temporarily unavailable."),
  });
  const today = settings.data ? dateInTimezone(settings.data.timezone) : "";
  const currentMonth = today ? today.slice(0, 7) : "";
  const month = selectedMonth ?? currentMonth;
  const range = today && month ? calendarMonthRange(month, today) : null;
  const calendar = useQuery({
    queryKey: userQueryKey("calendar", range?.from, range?.to),
    enabled: range !== null,
    queryFn: () => api.get<CalendarEntry[]>(`/api/v1/calendar?from=${range!.from}&to=${range!.to}`, "Calendar is temporarily unavailable."),
  });
  if (settings.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (settings.isError || !range) return <Alert color="red" mt="md">Calendar settings are temporarily unavailable.</Alert>;
  const controls = (
    <Group justify="space-between" mt="md">
      <Button variant="default" disabled={month <= currentMonth} onClick={() => setSelectedMonth(shiftCalendarMonth(month, -1))}>Previous month</Button>
      <Title order={2} size="h3">{formatCalendarMonth(month)}</Title>
      <Button variant="default" onClick={() => setSelectedMonth(shiftCalendarMonth(month, 1))}>Next month</Button>
    </Group>
  );
  if (calendar.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (calendar.isError) return <Alert color="red" mt="md">Calendar is temporarily unavailable.</Alert>;
  if (!calendar.data?.length) return <>{controls}<EmptyState title="No upcoming episodes this month" detail="You are all clear for this part of your calendar." /></>;
  const groups = groupCalendarEntries(calendar.data);

  return (
    <>
      {controls}
      {groups.map(group => (
        <section key={group.date} aria-label={`Episodes airing ${formatCalendarDate(group.date)}`}>
          <Text component="h2" fw={700} size="lg" mt="lg">{formatCalendarDate(group.date)}</Text>
          {group.entries.map(item => (
            <Paper key={item.episode.id} className="calendar-card" withBorder p="md" mt="sm">
              <Text fw={700}>{item.title}</Text>
              <Text>{`S${String(item.episode.season_number).padStart(2, "0")}E${String(item.episode.episode_number).padStart(2, "0")}`}</Text>
            </Paper>
          ))}
        </section>
      ))}
    </>
  );
}
