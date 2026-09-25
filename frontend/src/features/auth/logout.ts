import type { QueryClient } from "@tanstack/react-query";

export async function endCurrentSession(): Promise<void> {
  const response = await fetch("/api/v1/auth/logout", { method: "POST" });
  if (!response.ok) throw new Error("Could not sign out.");
}

export function clearSignedInCache(queryClient: QueryClient): void {
  queryClient.removeQueries({
    predicate: (query) => query.queryKey[0] !== "session" && query.queryKey[0] !== "auth-status",
  });
  queryClient.setQueryData(["session"], null);
}
