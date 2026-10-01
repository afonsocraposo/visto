import { UnstyledButton } from "@mantine/core";
import { AccountAvatar } from "../../components/AccountAvatar";
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
        <UnstyledButton
          className="library-account-button"
          aria-label="Account and settings"
          onClick={onOpenSettings}
        >
          <AccountAvatar name={user.name} size={36} />
        </UnstyledButton>
      }
    />
  );
}
