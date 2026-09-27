import type { LibraryMediaFilter } from "./mediaFilter";
import type { LibrarySort } from "./librarySort";
import { readStoredChoice, writeStoredChoice } from "../../lib/browserStorage";

const mediaFilters: readonly LibraryMediaFilter[] = ["all", "movie", "tv"];
const sorts: readonly LibrarySort[] = ["updated", "title", "released"];

function key(userID: string, preference: "filter" | "sort"): string {
  return `visto:library:${userID}:${preference}`;
}

export function readLibraryFilter(userID: string): LibraryMediaFilter {
  return readStoredChoice("local", key(userID, "filter"), mediaFilters, "all");
}

export function saveLibraryFilter(userID: string, value: LibraryMediaFilter): void {
  writeStoredChoice("local", key(userID, "filter"), value);
}

export function readLibrarySort(userID: string): LibrarySort {
  return readStoredChoice("local", key(userID, "sort"), sorts, "updated");
}

export function saveLibrarySort(userID: string, value: LibrarySort): void {
  writeStoredChoice("local", key(userID, "sort"), value);
}
