import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Alert, Avatar, Button, Drawer, Loader, Stack, Text, TextInput } from "@mantine/core";
import { useUserQueryKey } from "../auth/SessionContext";
import { fetchAllPages } from "../../lib/pagination";
import type { CommunityUser } from "../../types";

export function UserDirectory({
  opened,
  onClose,
  onOpenUser,
}: {
  opened: boolean;
  onClose: () => void;
  onOpenUser: (userID: string) => void;
}) {
  const [search, setSearch] = useState("");
  const userQueryKey = useUserQueryKey();
  const users = useQuery({
    queryKey: userQueryKey("community-users"),
    queryFn: () => fetchAllPages<CommunityUser>("/api/v1/community/users", "Could not load users."),
    enabled: opened,
  });
  const matches =
    users.data?.filter((user) =>
      user.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()),
    ) ?? [];

  return (
    <Drawer opened={opened} onClose={onClose} title="Users" position="right">
      <TextInput
        aria-label="Search users"
        placeholder="Search users"
        value={search}
        onChange={(event) => setSearch(event.currentTarget.value)}
        mb="md"
      />
      {users.isPending && <Loader size="sm" />}
      {users.isError && <Alert color="red">Could not load users.</Alert>}
      {users.isSuccess && (
        <Stack gap="xs">
          {matches.length === 0 && <Text c="dimmed">No users found.</Text>}
          {matches.map((user) => (
            <Button
              key={user.id}
              variant="subtle"
              color="gray"
              justify="flex-start"
              leftSection={
                <Avatar size={28}>{user.name.trim().slice(0, 1).toLocaleUpperCase()}</Avatar>
              }
              onClick={() => onOpenUser(user.id)}
            >
              {user.name}
            </Button>
          ))}
        </Stack>
      )}
    </Drawer>
  );
}
