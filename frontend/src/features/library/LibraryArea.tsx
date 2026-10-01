import { AccountAvatar } from "../../components/AccountAvatar";
import { RouteLink } from "../../components/RouteLink";
import { LibraryPanel } from "./Library";
import type { LibraryStatus, MediaDetailTarget, User } from "../../types";
import type { LibraryMediaFilter } from "./mediaFilter";

/** Library is the main surface; account settings open from the avatar in its header. */
export function LibraryArea({
  user,
  onOpenSettings,
  onOpenDetail,
  onOpenList,
}: {
  user: User;
  onOpenSettings: () => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
  onOpenList?: (status: LibraryStatus, mediaFilter: LibraryMediaFilter) => void;
}) {
  return (
    <LibraryPanel
      onOpenDetail={onOpenDetail}
      onOpenList={onOpenList}
      headerAction={
        <RouteLink
          href="/settings"
          className="library-account-button"
          aria-label="Account and settings"
          onOpen={onOpenSettings}
        >
          <AccountAvatar name={user.name} size={36} />
        </RouteLink>
      }
    />
  );
}
