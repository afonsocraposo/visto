import { useState } from "react";
import { PlexAdminPanel } from "./PlexAdminPanel";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@mantine/form";
import {
  ActionIcon,
  Alert,
  Avatar,
  Badge,
  Button,
  Group,
  Loader,
  Modal,
  Paper,
  PasswordInput,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import { IconPencil, IconSearch, IconTrash, IconUserPlus } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { useAdminUsers } from "./useAdminUsers";
import type { User } from "../../types";
import { api } from "../../lib/api";
import { showActionFeedback } from "../../lib/actionFeedback";
import { ActivityTime } from "../../components/ActivityTime";
import { EmptyState } from "../../components/EmptyState";

export function AdminPanel({ currentUser }: { currentUser: User }) {
  const [search, setSearch] = useState("");
  const [addUserOpened, setAddUserOpened] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [pendingDelete, setPendingDelete] = useState<User | null>(null);
  const users = useAdminUsers();
  const matches = (users.data ?? []).filter((account) => {
    const term = search.trim().toLocaleLowerCase();
    if (!term) return true;
    return (
      account.name.toLocaleLowerCase().includes(term) ||
      account.email.toLocaleLowerCase().includes(term)
    );
  });

  return (
    <Stack>
      <PlexAdminPanel />
      <Paper withBorder p="md">
        <Group justify="space-between" align="flex-start" wrap="wrap">
          <div>
            <Title order={2}>Manage users</Title>
            <Text size="sm" c="dimmed" mt="xs">
              {users.data ? `${users.data.length} user${users.data.length === 1 ? "" : "s"}` : ""}
            </Text>
          </div>
          <Group gap="sm" wrap="wrap">
            <TextInput
              aria-label="Search users"
              placeholder="Search users"
              leftSection={<IconSearch size={16} />}
              value={search}
              onChange={(event) => setSearch(event.currentTarget.value)}
            />
            <Button leftSection={<IconUserPlus size={16} />} onClick={() => setAddUserOpened(true)}>
              Add user
            </Button>
          </Group>
        </Group>

        {users.isError && (
          <Alert color="red" mt="md">
            {users.error.message}
          </Alert>
        )}
        {users.isPending && (
          <Group justify="center" mt="xl">
            <Loader size="sm" />
          </Group>
        )}
        {users.isSuccess && matches.length === 0 && (
          <EmptyState
            title={search ? "No users found" : "No users yet"}
            detail={search ? "Try a different name or email." : "Add a user to get started."}
          />
        )}
        {users.isSuccess && matches.length > 0 && (
          <Table.ScrollContainer minWidth={640} mt="md">
            <Table striped withTableBorder verticalSpacing="sm">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>User</Table.Th>
                  <Table.Th>Email</Table.Th>
                  <Table.Th>Role</Table.Th>
                  <Table.Th>Joined</Table.Th>
                  <Table.Th></Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {matches.map((account) => (
                  <Table.Tr key={account.id}>
                    <Table.Td>
                      <Group gap="sm" wrap="nowrap">
                        <Avatar size={32}>
                          {account.name.trim().slice(0, 1).toLocaleUpperCase()}
                        </Avatar>
                        <Text fw={600}>
                          {account.name}
                          {account.id === currentUser.id ? " (you)" : ""}
                        </Text>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" c="dimmed">
                        {account.email}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Badge
                        color={account.role === "admin" ? undefined : "gray"}
                        variant={account.role === "admin" ? "filled" : "light"}
                      >
                        {account.role === "admin" ? "Admin" : "User"}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" c="dimmed">
                        <ActivityTime value={account.created_at} />
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Group gap={4} justify="flex-end" wrap="nowrap">
                        <Tooltip label="Edit user">
                          <ActionIcon
                            variant="subtle"
                            aria-label={`Edit ${account.name}`}
                            onClick={() => setEditingUser(account)}
                          >
                            <IconPencil size={16} />
                          </ActionIcon>
                        </Tooltip>
                        {account.id !== currentUser.id && (
                          <Tooltip label="Delete user">
                            <ActionIcon
                              variant="subtle"
                              color="red"
                              aria-label={`Delete ${account.name}`}
                              onClick={() => setPendingDelete(account)}
                            >
                              <IconTrash size={16} />
                            </ActionIcon>
                          </Tooltip>
                        )}
                      </Group>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        )}
      </Paper>

      <AddUserModal opened={addUserOpened} onClose={() => setAddUserOpened(false)} />
      {editingUser && (
        <EditUserModal
          user={editingUser}
          currentUser={currentUser}
          onClose={() => setEditingUser(null)}
        />
      )}
      <DeleteUserModal user={pendingDelete} onClose={() => setPendingDelete(null)} />
    </Stack>
  );
}

function AddUserModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const form = useForm({ initialValues: { email: "", name: "", password: "" } });
  const createUser = useMutation({
    mutationFn: () =>
      api.post<User>(
        "/api/v1/users",
        { email: form.values.email, name: form.values.name, password: form.values.password },
        "Could not create account.",
      ),
    onSuccess: async () => {
      form.reset();
      showActionFeedback("Account created.");
      await queryClient.invalidateQueries({ queryKey: userQueryKey("admin-users") });
      onClose();
    },
  });

  return (
    <Modal
      opened={opened}
      onClose={() => {
        form.reset();
        onClose();
      }}
      title="Add a user"
      centered
    >
      <Text size="sm" c="dimmed" mb="md">
        Each person gets a separate library and watch history.
      </Text>
      <form
        onSubmit={form.onSubmit(() => {
          createUser.mutate();
        })}
      >
        <TextInput required type="email" label="Email" {...form.getInputProps("email")} />
        <TextInput required maxLength={80} label="Name" mt="md" {...form.getInputProps("name")} />
        <PasswordInput
          required
          minLength={12}
          label="Initial password"
          mt="md"
          {...form.getInputProps("password")}
        />
        {createUser.isError && (
          <Alert color="red" mt="md">
            {createUser.error.message}
          </Alert>
        )}
        <Group justify="flex-end" mt="lg">
          <Button variant="default" onClick={onClose} type="button">
            Cancel
          </Button>
          <Button type="submit" loading={createUser.isPending}>
            Create account
          </Button>
        </Group>
      </form>
    </Modal>
  );
}

function EditUserModal({
  user,
  currentUser,
  onClose,
}: {
  user: User;
  currentUser: User;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const form = useForm({ initialValues: { displayName: user.name, password: "" } });
  const update = useMutation({
    mutationFn: () =>
      api.patch(
        `/api/v1/users/${encodeURIComponent(user.id)}`,
        { name: form.values.displayName, password: form.values.password },
        "Could not update account.",
      ),
    onSuccess: async () => {
      showActionFeedback("Account updated.");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: userQueryKey("admin-users") }),
        ...(user.id === currentUser.id
          ? [queryClient.invalidateQueries({ queryKey: ["session"] })]
          : []),
      ]);
      onClose();
    },
  });

  return (
    <Modal opened onClose={onClose} title={`Edit ${user.name}`} centered>
      <form
        onSubmit={form.onSubmit(() => {
          update.mutate();
        })}
      >
        <TextInput label="Name" required maxLength={80} {...form.getInputProps("displayName")} />
        <PasswordInput
          label="New password"
          description="Leave blank to keep the current password."
          mt="md"
          minLength={12}
          {...form.getInputProps("password")}
        />
        {update.isError && (
          <Alert color="red" mt="md">
            {update.error.message}
          </Alert>
        )}
        <Group justify="flex-end" mt="lg">
          <Button variant="default" onClick={onClose} type="button">
            Cancel
          </Button>
          <Button type="submit" loading={update.isPending}>
            Save changes
          </Button>
        </Group>
      </form>
    </Modal>
  );
}

function DeleteUserModal({ user, onClose }: { user: User | null; onClose: () => void }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const remove = useMutation({
    mutationFn: (id: string) =>
      api.delete(`/api/v1/users/${encodeURIComponent(id)}`, "Could not delete account."),
    onSuccess: async () => {
      showActionFeedback("Account deleted.");
      await queryClient.invalidateQueries({ queryKey: userQueryKey("admin-users") });
      onClose();
    },
  });

  return (
    <Modal opened={user !== null} onClose={onClose} title="Delete account?" centered>
      <Text size="sm">
        Delete {user?.name}&apos;s account and personal data? This cannot be undone.
      </Text>
      {remove.isError && (
        <Alert color="red" mt="md">
          {remove.error.message}
        </Alert>
      )}
      <Group justify="flex-end" mt="lg">
        <Button variant="default" onClick={onClose}>
          Cancel
        </Button>
        <Button
          color="red"
          loading={remove.isPending}
          onClick={() => user && remove.mutate(user.id)}
        >
          Delete account
        </Button>
      </Group>
    </Modal>
  );
}
