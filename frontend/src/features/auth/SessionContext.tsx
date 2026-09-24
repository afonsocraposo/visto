import { createContext, useContext, type ReactNode } from "react";
import type { QueryKey } from "@tanstack/react-query";
import type { User } from "../../types";

const SessionContext = createContext<User | null>(null);

export function SessionProvider({ user, children }: { user: User; children: ReactNode }) {
  return <SessionContext.Provider value={user}>{children}</SessionContext.Provider>;
}

export function useUserQueryKey() {
  const user = useContext(SessionContext);
  if (!user) throw new Error("User-scoped queries require an authenticated session.");
  return (...key: QueryKey): QueryKey => ["user", user.id, ...key];
}
