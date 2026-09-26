import type { User } from "../../types";

export async function loadSession(): Promise<User | null> {
  const response = await fetch("/api/v1/me");
  if (response.status === 401) return null;
  if (!response.ok) throw new Error("Could not check your session.");
  return response.json() as Promise<User>;
}
