export function posterURL(path?: string | null, size: "w185" | "w342" | "w500" = "w342") {
  return path ? `https://image.tmdb.org/t/p/${size}${path}` : null;
}

export function backdropURL(path?: string | null, size: "w780" | "w1280" = "w1280") {
  return path ? `https://image.tmdb.org/t/p/${size}${path}` : null;
}
