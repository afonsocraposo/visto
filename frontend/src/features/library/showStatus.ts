import type { ShowProgress } from "../../types";

export function hasCaughtUpDisplayState(progress: ShowProgress | undefined): boolean {
  return progress?.is_caught_up === true && progress.cursor !== null && progress.cursor !== undefined;
}
