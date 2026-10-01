import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Alert, Avatar, Button, Drawer, Skeleton, Stack, Text, TextInput } from "@mantine/core";
import { useSessionUserID, useUserQueryKey } from "../auth/SessionContext";
import { fetchAllPages } from "../../lib/pagination";
import type { CommunityUser } from "../../types";
import { RouteLink } from "../../components/RouteLink";

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
  const currentUserID = useSessionUserID();
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
      {users.isPending && (
        <Stack gap="xs">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} height={36} radius="sm" />
          ))}
        </Stack>
      )}
      {users.isError && <Alert color="red">Could not load users.</Alert>}
      {users.isSuccess && (
        <Stack gap="xs">
          {matches.length === 0 && <Text c="dimmed">No users found.</Text>}
          {matches.map((user) => (
            <Button
              key={user.id}
              component={RouteLink}
              href={
                user.id === currentUserID ? "/profile" : `/users/${encodeURIComponent(user.id)}`
              }
              variant="subtle"
              color="gray"
              justify="flex-start"
              leftSection={
                <Avatar size={28}>{user.name.trim().slice(0, 1).toLocaleUpperCase()}</Avatar>
              }
              onOpen={() => onOpenUser(user.id)}
            >
              {user.name}
            </Button>
          ))}
        </Stack>
      )}
    </Drawer>
  );
}
