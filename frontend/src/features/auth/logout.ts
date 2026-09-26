import type { QueryClient } from "@tanstack/react-query";

export async function endCurrentSession(): Promise<void> {
  if ("serviceWorker" in navigator) {
    const registration = await navigator.serviceWorker.ready;
    const subscription = await registration.pushManager?.getSubscription();
    if (subscription) {
      const removed = await fetch("/api/v1/profile/web-push-subscription", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ endpoint: subscription.endpoint }),
      });
      if (removed.ok) await subscription.unsubscribe();
    }
  }
  const response = await fetch("/api/v1/auth/logout", { method: "POST" });
  if (!response.ok) throw new Error("Could not sign out.");
}

export function clearSignedInCache(queryClient: QueryClient): void {
  queryClient.removeQueries({
    predicate: (query) => query.queryKey[0] !== "session" && query.queryKey[0] !== "auth-status",
  });
  queryClient.setQueryData(["session"], null);
}
