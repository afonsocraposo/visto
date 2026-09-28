import { useQuery } from "@tanstack/react-query";
import { useUserQueryKey } from "../auth/SessionContext";
import { fetchAllPages } from "../../lib/pagination";
import type { User } from "../../types";

export function useAdminUsers() {
  const userQueryKey = useUserQueryKey();
  return useQuery({
    queryKey: userQueryKey("admin-users"),
    queryFn: () => fetchAllPages<User>("/api/v1/users", "Could not load users."),
  });
}
