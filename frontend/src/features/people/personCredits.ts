import type { PersonCredit } from "../../types";

export function sortPersonCredits(credits: PersonCredit[]): PersonCredit[] {
  return [...credits].sort((a, b) => b.popularity - a.popularity || a.title.localeCompare(b.title));
}
