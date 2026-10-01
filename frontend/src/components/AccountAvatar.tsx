import { Avatar } from "@mantine/core";

/** The signed-in user's initial, used for the account entry point in both navigation bars. */
export function AccountAvatar({ name, size }: { name?: string; size: number }) {
  return (
    <Avatar className="account-avatar" size={size} radius="xl">
      {name?.trim().slice(0, 1).toLocaleUpperCase() || "?"}
    </Avatar>
  );
}
