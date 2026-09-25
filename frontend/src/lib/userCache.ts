import { useCallback } from "react";
import { useQueryClient, type QueryKey } from "@tanstack/react-query";
import { useUserQueryKey } from "../features/auth/SessionContext";

type CacheScope = QueryKey;

export const userCache = {
  library: ["library"],
  continue: ["continue"],
  calendar: ["calendar"],
  feed: ["feed"],
  history: ["history"],
  progress: ["show-progress"],
} as const;

// Invalidations stay user-scoped and are grouped by the data a mutation can
// affect. Feature hooks can add their own detail keys without duplicating the
// Promise.all boilerplate.
export function useInvalidateUserCache() {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();

  return useCallback(
    (...scopes: CacheScope[]) =>
      Promise.all(
        scopes.map((scope) => queryClient.invalidateQueries({ queryKey: userQueryKey(...scope) })),
      ),
    [queryClient, userQueryKey],
  );
}
