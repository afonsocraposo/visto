import { useState } from "react";
import { FadeImage } from "../../components/FadeImage";
import {
  useInfiniteQuery,
  useMutation,
  useMutationState,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Group,
  Indicator,
  Modal,
  Paper,
  Skeleton,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import { Calendar } from "@mantine/dates";
import { IconCalendar, IconCheck, IconDeviceTv } from "@tabler/icons-react";
import { useNavigate } from "@tanstack/react-router";
import { QueryError } from "../../components/QueryError";
import { episodeCode } from "../../lib/episodePosition";
import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { EmptyState } from "../../components/EmptyState";
import { ListSkeleton } from "../../components/ListSkeleton";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { showActionFeedback } from "../../lib/actionFeedback";
import type { Play } from "../../generated/models/play";
import { useInvalidateUserCache, userCache } from "../../lib/userCache";
import {
  dateInTimezone,
  daysUntilRelease,
  formatCalendarDate,
  groupCalendarEntries,
  upcomingYearRange,
} from "./calendar";
import type { CalendarEntry, ContinueEntry, CursorPage, MediaDetailTarget } from "../../types";
import { backdropURL, posterURL } from "../../lib/artwork";
import { pageURL } from "../../lib/pagination";
import { WatchRowCard } from "./WatchRowCard";

const markWatchedMutationKey = ["continue-watching", "mark-watched"] as const;
type MarkWatchedVariables = { episodeIDs: string[]; bulk: boolean; showID: string };

const upNext = (
  <h1 className="watch-divider">
    <span>Up next</span>
  </h1>
);

export function WatchNow({ onOpenDetail }: { onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const invalidate = useInvalidateUserCache();
  const [confirmation, setConfirmation] = useState<ContinueEntry | null>(null);
  const [completedShowIDs, setCompletedShowIDs] = useState<Set<string>>(() => new Set());
  const navigate = useNavigate();
  const entries = useQuery({
    queryKey: userQueryKey("continue"),
    queryFn: () =>
      api.get<ContinueEntry[]>(
        "/api/v1/continue-watching",
        "Watch data is temporarily unavailable.",
      ),
  });
  const markWatched = useMutation({
    mutationKey: markWatchedMutationKey,
    mutationFn: ({ episodeIDs, bulk }: MarkWatchedVariables) =>
      api.post<Play | Play[]>(
        bulk ? "/api/v1/plays/bulk" : "/api/v1/plays",
        bulk ? { episode_ids: episodeIDs } : { episode_id: episodeIDs[0] },
        "Could not mark episode watched.",
      ),
    onSuccess: async (result, variables) => {
      setConfirmation((current) => (current?.show_id === variables.showID ? null : current));
      setCompletedShowIDs((current) => new Set(current).add(variables.showID));
      const plays = Array.isArray(result) ? result : [result];
      showActionFeedback(
        `${plays.length} ${plays.length === 1 ? "episode" : "episodes"} marked watched.`,
        async () => {
          for (const play of plays)
            await api.delete(`/api/v1/plays/${encodeURIComponent(play.id)}`);
          await invalidate(
            userCache.progress,
            userCache.calendar,
            userCache.feed,
            userCache.history,
            userCache.library,
            userCache.continue,
          );
        },
      );
      await invalidate(
        userCache.progress,
        userCache.calendar,
        userCache.feed,
        userCache.history,
        userCache.library,
      );
    },
  });
  const pendingWatchActions = useMutationState({
    filters: { mutationKey: markWatchedMutationKey, status: "pending" },
    select: (mutation) => mutation.state.variables as MarkWatchedVariables,
  });
  const confirmationActions = pendingWatchActions.filter(
    (action) => action.showID === confirmation?.show_id,
  );

  if (entries.isPending)
    return (
      <>
        {upNext}
        <div className="watch-list" aria-busy="true">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="watch-row">
              <div className="watch-row-art">
                <Skeleton height="100%" width="100%" radius={0} />
              </div>
              <div className="watch-row-content">
                <Skeleton height={12} width={80} />
                <Skeleton height={18} width={160} mt={4} />
                <Skeleton height={12} width={110} mt={4} />
              </div>
              <Skeleton
                height={44}
                width={44}
                circle
                mx="md"
                style={{ alignSelf: "center", flexShrink: 0 }}
              />
            </div>
          ))}
        </div>
      </>
    );
  if (entries.isError)
    return (
      <>
        {upNext}
        <QueryError
          message="Could not load what to watch next."
          onRetry={() => entries.refetch()}
        />
      </>
    );
  if (!entries.data?.length)
    return (
      <>
        {upNext}
        <EmptyState
          icon={<IconDeviceTv size={22} />}
          title="Nothing to watch"
          detail="Add a show and its next released episode will appear here."
          action={
            <Button variant="light" onClick={() => void navigate({ to: "/discover" })}>
              Discover shows
            </Button>
          }
        />
      </>
    );

  const finishWatchedAnimation = (showID: string) => {
    setCompletedShowIDs((current) => {
      if (!current.has(showID)) return current;
      const next = new Set(current);
      next.delete(showID);
      return next;
    });
    void queryClient.invalidateQueries({ queryKey: userQueryKey("continue") });
  };

  return (
    <>
      <Modal
        opened={confirmation !== null}
        onClose={() => setConfirmation(null)}
        title="Skipped episodes"
        centered
      >
        <Text mb="md">
          You have {confirmation?.missing_prior_episodes?.length} earlier unplayed episodes of{" "}
          {confirmation?.title}. How would you like to continue?
        </Text>
        <Group justify="flex-end">
          <Button
            variant="default"
            onClick={() =>
              confirmation?.next_episode &&
              markWatched.mutate({
                episodeIDs: [confirmation.next_episode.id],
                bulk: false,
                showID: confirmation.show_id,
              })
            }
            loading={confirmationActions.some((action) => !action.bulk)}
            disabled={confirmationActions.length > 0}
          >
            Only this episode
          </Button>
          <Button
            onClick={() =>
              confirmation?.next_episode &&
              markWatched.mutate({
                episodeIDs: [
                  ...(confirmation.missing_prior_episodes || []).map((episode) => episode.id),
                  confirmation.next_episode.id,
                ],
                bulk: true,
                showID: confirmation.show_id,
              })
            }
            loading={confirmationActions.some((action) => action.bulk)}
            disabled={confirmationActions.length > 0}
          >
            Mark all as watched
          </Button>
        </Group>
      </Modal>
      {upNext}
      <div className="watch-list content-ready">
        {entries.data.map((entry) => {
          const art =
            backdropURL(entry.next_episode_still_path, "w780") ??
            posterURL(entry.poster_path, "w500");
          const isCompleted = completedShowIDs.has(entry.show_id);
          const next = entry.next_episode;
          const openEpisode = () =>
            onOpenDetail?.({
              mediaType: "tv",
              tmdbID: Number(entry.show_id.split(":")[1]),
              mediaID: entry.show_id,
              episodeID: next?.id,
              episode: next,
              seasonNumber: next?.season_number,
              episodeNumber: next?.episode_number,
            });
          const openShow = () =>
            onOpenDetail?.({
              mediaType: "tv",
              tmdbID: Number(entry.show_id.split(":")[1]),
              mediaID: entry.show_id,
              seasonNumber: next?.season_number,
            });
          return (
            <WatchRowCard
              key={entry.show_id}
              className={isCompleted ? "watch-row-completed" : ""}
              show={entry.title}
              onOpenShow={onOpenDetail && !isCompleted ? openShow : undefined}
              title={
                next
                  ? entry.next_episode_name || `Episode ${next.episode_number}`
                  : "Episode details are pending"
              }
              openLabel={
                next
                  ? `Open ${entry.title}, season ${next.season_number}, episode ${next.episode_number}`
                  : undefined
              }
              meta={
                next && (
                  <>
                    <span>{episodeCode(next)}</span>
                    {entry.remaining_episodes > 0 && <span>{entry.remaining_episodes} left</span>}
                  </>
                )
              }
              art={art}
              onOpen={onOpenDetail && !isCompleted ? openEpisode : undefined}
              onAnimationEnd={(event) => {
                if (event.animationName === "watch-row-exit") finishWatchedAnimation(entry.show_id);
              }}
              trailing={
                isCompleted ? (
                  <span
                    className="watch-row-action watch-row-complete-indicator"
                    role="status"
                    aria-label={`${entry.title} episode marked watched`}
                  >
                    <IconCheck size={22} stroke={2.4} />
                  </span>
                ) : (
                  next && (
                    <Tooltip label="Mark episode watched" withArrow>
                      <ActionIcon
                        className="watch-row-action"
                        size={48}
                        radius="xl"
                        variant="default"
                        aria-label={`Mark ${entry.title} season ${next.season_number}, episode ${next.episode_number} watched`}
                        loading={pendingWatchActions.some(
                          (action) => action.showID === entry.show_id,
                        )}
                        onClick={() =>
                          entry.missing_prior_episodes?.length
                            ? setConfirmation(entry)
                            : markWatched.mutate({
                                episodeIDs: [next.id],
                                bulk: false,
                                showID: entry.show_id,
                              })
                        }
                      >
                        <IconCheck size={22} stroke={2} />
                      </ActionIcon>
                    </Tooltip>
                  )
                )
              }
            />
          );
        })}
      </div>
    </>
  );
}

export function WatchCalendar({
  onOpenDetail,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const userQueryKey = useUserQueryKey();
  const [calendarOpen, setCalendarOpen] = useState(false);
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const settings = useQuery({
    queryKey: userQueryKey("profile-settings"),
    queryFn: () =>
      api.get<{ timezone: string }>(
        "/api/v1/profile/activity-settings",
        "Calendar settings are temporarily unavailable.",
      ),
  });
  const today = settings.data ? dateInTimezone(settings.data.timezone) : "";
  const range = today ? upcomingYearRange(today) : null;
  const calendarPath = range ? `/api/v1/calendar?from=${range.from}&to=${range.to}` : "";
  const entries = useInfiniteQuery({
    queryKey: userQueryKey("calendar", range?.from, range?.to),
    enabled: range !== null,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<CalendarEntry>>(
        pageURL(calendarPath, pageParam),
        "Calendar is temporarily unavailable.",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const dates = useQuery({
    queryKey: userQueryKey("calendar-dates", range?.from, range?.to),
    enabled: range !== null && calendarOpen,
    queryFn: () =>
      api.get<string[]>(
        `/api/v1/calendar/dates?from=${range!.from}&to=${range!.to}`,
        "Release dates are temporarily unavailable.",
      ),
  });
  const dayEntries = useInfiniteQuery({
    queryKey: userQueryKey("calendar", selectedDate, selectedDate),
    enabled: selectedDate !== null && calendarOpen,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<CalendarEntry>>(
        pageURL(`/api/v1/calendar?from=${selectedDate}&to=${selectedDate}`, pageParam),
        "Episodes for this date are temporarily unavailable.",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });

  if (settings.isPending)
    return (
      <>
        <Group className="calendar-header" justify="space-between" align="center" mt="md">
          <Skeleton height={28} width={220} />
          <Skeleton height={44} width={44} circle />
        </Group>
        <div className="calendar-list">
          <CalendarDayGroupSkeleton />
        </div>
      </>
    );
  if (settings.isError || !range)
    return (
      <QueryError message="Could not load your calendar." onRetry={() => settings.refetch()} />
    );

  const releaseDates = new Set(dates.data ?? []);
  const loaded = entries.data?.pages.flatMap((page) => page.items) ?? [];
  const groups = groupCalendarEntries(loaded);
  const selectedEntries = dayEntries.data?.pages.flatMap((page) => page.items) ?? [];
  return (
    <>
      <Group className="calendar-header" justify="space-between" align="center" mt="md">
        <Title order={1} className="calendar-month">
          Upcoming episodes
        </Title>
        <ActionIcon
          variant="default"
          size="xl"
          aria-label="Open release calendar"
          onClick={() => setCalendarOpen(true)}
        >
          <IconCalendar size={22} />
        </ActionIcon>
      </Group>
      {entries.isPending ? (
        <div className="calendar-list">
          <CalendarDayGroupSkeleton />
          <CalendarDayGroupSkeleton />
        </div>
      ) : entries.isError ? (
        <QueryError message="Could not load upcoming episodes." onRetry={() => entries.refetch()} />
      ) : loaded.length === 0 ? (
        <EmptyState
          icon={<IconCalendar size={20} />}
          title="No upcoming episodes"
          detail="When shows you're watching announce new episodes, their dates appear here."
        />
      ) : (
        <div className="calendar-list content-ready">
          {groups.map((group) => (
            <section
              className="calendar-day"
              key={group.date}
              aria-label={`Episodes airing ${formatCalendarDate(group.date, today.slice(0, 4))}`}
            >
              <Text component="h2" className="calendar-date" fw={700}>
                {formatCalendarDate(group.date, today.slice(0, 4))}
              </Text>
              <div className="calendar-day-entries">
                {group.entries.map((item) => (
                  <CalendarEpisodeCard
                    key={item.episode.id}
                    item={item}
                    today={today}
                    onOpenDetail={onOpenDetail}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
      <InfiniteScrollTrigger
        hasNextPage={!!entries.hasNextPage}
        isFetchingNextPage={entries.isFetchingNextPage}
        isFetchNextPageError={entries.isFetchNextPageError}
        fetchNextPage={() => void entries.fetchNextPage()}
      />
      <Modal
        opened={calendarOpen}
        onClose={() => {
          setCalendarOpen(false);
          setSelectedDate(null);
        }}
        title="Release calendar"
        centered
        size="lg"
      >
        {dates.isPending ? (
          <Skeleton height={320} radius="md" />
        ) : dates.isError ? (
          <QueryError
            mt={0}
            message="Could not load release dates."
            onRetry={() => dates.refetch()}
          />
        ) : (
          <Calendar
            fullWidth
            defaultDate={today}
            minDate={range.from}
            maxDate={range.to}
            renderDay={(date) => (
              <Indicator size={6} color="amber" offset={-3} disabled={!releaseDates.has(date)}>
                <span>{Number(date.slice(-2))}</span>
              </Indicator>
            )}
            getDayProps={(date) => ({
              selected: date === selectedDate,
              onClick: () => setSelectedDate(releaseDates.has(date) ? date : null),
            })}
          />
        )}
        {selectedDate && (
          <div className="calendar-selected-day">
            <Text component="h2" fw={700} mb="sm">
              {formatCalendarDate(selectedDate, today.slice(0, 4))}
            </Text>
            {dayEntries.isPending ? (
              <div className="calendar-day-entries">
                <ListSkeleton
                  count={2}
                  rowClassName="calendar-card"
                  artClassName="calendar-card-art"
                  contentClassName="calendar-card-copy"
                  lines={3}
                  padded={false}
                />
              </div>
            ) : dayEntries.isError ? (
              <QueryError
                mt={0}
                message="Could not load this day's episodes."
                onRetry={() => dayEntries.refetch()}
              />
            ) : (
              <div className="calendar-day-entries">
                {selectedEntries.map((item) => (
                  <CalendarEpisodeCard
                    key={item.episode.id}
                    item={item}
                    today={today}
                    onOpenDetail={onOpenDetail}
                  />
                ))}
              </div>
            )}
            <InfiniteScrollTrigger
              hasNextPage={!!dayEntries.hasNextPage}
              isFetchingNextPage={dayEntries.isFetchingNextPage}
              isFetchNextPageError={dayEntries.isFetchNextPageError}
              fetchNextPage={() => void dayEntries.fetchNextPage()}
            />
          </div>
        )}
      </Modal>
    </>
  );
}

function CalendarDayGroupSkeleton() {
  return (
    <section className="calendar-day">
      <Skeleton height={20} width={160} mb="sm" />
      <div className="calendar-day-entries">
        <ListSkeleton
          count={2}
          rowClassName="calendar-card"
          artClassName="calendar-card-art"
          contentClassName="calendar-card-copy"
          lines={3}
          padded={false}
        />
      </div>
    </section>
  );
}

function CalendarEpisodeCard({
  item,
  today,
  onOpenDetail,
}: {
  item: CalendarEntry;
  today: string;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const art = backdropURL(item.episode_still_path, "w780") ?? posterURL(item.poster_path, "w500");
  const releaseDate = item.episode.air_date?.slice(0, 10) ?? today;
  const days = daysUntilRelease(today, releaseDate);
  const open = () =>
    onOpenDetail?.({
      mediaType: "tv",
      tmdbID: Number(item.show_id.split(":")[1]),
      mediaID: item.show_id,
      episodeID: item.episode.id,
      episode: item.episode,
      seasonNumber: item.episode.season_number,
      episodeNumber: item.episode.episode_number,
    });
  return (
    <Paper className="calendar-card" withBorder p={0} component="article">
      {onOpenDetail && (
        <button type="button" className="calendar-card-open" onClick={open}>
          <span className="visually-hidden">
            Open {item.title}, season {item.episode.season_number}, episode{" "}
            {item.episode.episode_number}
          </span>
        </button>
      )}
      <div className="calendar-card-art">
        {art ? (
          <FadeImage src={art} alt="" />
        ) : (
          <div className="artwork-fallback">{item.title.slice(0, 1)}</div>
        )}
      </div>
      <div className="calendar-card-copy">
        <button
          type="button"
          className="calendar-card-show calendar-show-link"
          disabled={!onOpenDetail}
          onClick={() => {
            onOpenDetail?.({
              mediaType: "tv",
              tmdbID: Number(item.show_id.split(":")[1]),
              mediaID: item.show_id,
            });
          }}
        >
          {item.title}
        </button>
        <Text className="calendar-card-number">{episodeCode(item.episode)}</Text>
        <Text className="calendar-card-episode" lineClamp={1}>
          {item.episode_name || `Episode ${item.episode.episode_number}`}
        </Text>
      </div>
      <div className={`calendar-card-countdown${days === 0 ? " is-today" : ""}`}>
        <strong>{days === 0 ? "Today" : days}</strong>
        {days !== 0 && <span>{days === 1 ? "day" : "days"}</span>}
      </div>
    </Paper>
  );
}
