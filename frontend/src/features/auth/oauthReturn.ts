export function oauthReturnLocation(search: string, origin: string): string | null {
  const returnTo = new URLSearchParams(search).get("oauth_return");
  if (!returnTo || !returnTo.startsWith("/") || returnTo.startsWith("//")) return null;

  try {
    const destination = new URL(returnTo, origin);
    if (
      destination.origin !== origin ||
      destination.pathname !== "/oauth/authorize" ||
      !destination.search ||
      destination.hash
    ) {
      return null;
    }
    return `${destination.pathname}${destination.search}`;
  } catch {
    return null;
  }
}
