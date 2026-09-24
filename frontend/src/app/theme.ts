import type { Theme } from "../types";

export type ForcedColorScheme = "light" | "dark" | undefined;

export function forcedColorScheme(theme: Theme): ForcedColorScheme {
  return theme === "system" ? undefined : theme;
}
