export type LibrarySort = "updated" | "title" | "released";

export const librarySortOptions = [
  { value: "updated", label: "Last watched (last updated)" },
  { value: "title", label: "Title" },
  { value: "released", label: "Date released" },
];
