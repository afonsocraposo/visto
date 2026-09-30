import dayjs from "dayjs";

// Shows "in N days" for episodes airing within the next week; otherwise the raw air date.
export function formatEpisodeAirDate(airDate: string | undefined | null, today = dayjs()) {
  if (!airDate) return "Air date not announced";
  const days = dayjs(airDate).startOf("day").diff(today.startOf("day"), "day");
  if (days >= 1 && days <= 7) return `in ${days} ${days === 1 ? "day" : "days"}`;
  return airDate;
}
