import { IconCompass, IconHome, IconLayoutGrid, IconSearch } from "@tabler/icons-react";
import type { Tab } from "../../types";

/** The four main destinations, shared by the mobile bottom bar and the desktop top bar. */
export const navItems: ReadonlyArray<{
  tab: Tab;
  label: string;
  path: string;
  Icon: typeof IconHome;
}> = [
  { tab: "watch", label: "Watching", path: "/watch", Icon: IconHome },
  { tab: "search", label: "Discover", path: "/discover", Icon: IconSearch },
  { tab: "feed", label: "Activity", path: "/feed", Icon: IconCompass },
  { tab: "library", label: "Library", path: "/profile", Icon: IconLayoutGrid },
];
