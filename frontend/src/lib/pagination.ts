import { api } from "./api";

export type Page<T> = { items: T[]; next_cursor: string | null };

export function pageURL(path: string, cursor?: string): string {
  if (!cursor) return path;
  return `${path}${path.includes("?") ? "&" : "?"}cursor=${encodeURIComponent(cursor)}`;
}

export async function fetchAllPages<T>(path: string, errorMessage: string): Promise<T[]> {
  const items: T[] = [];
  let cursor: string | undefined;
  do {
    const page: Page<T> = await api.get(pageURL(path, cursor), errorMessage);
    items.push(...page.items);
    cursor = page.next_cursor ?? undefined;
  } while (cursor);
  return items;
}
