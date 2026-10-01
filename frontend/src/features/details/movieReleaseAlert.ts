export function showMovieReleaseAlert(
  status: string | undefined,
  releaseDate: string,
  today: string,
) {
  return status === "watchlist" && /^\d{4}-\d{2}-\d{2}$/.test(releaseDate) && releaseDate > today;
}

export function movieReleaseAlertLabel(enabled: boolean) {
  return `Release alert · ${enabled ? "On" : "Off"}`;
}
