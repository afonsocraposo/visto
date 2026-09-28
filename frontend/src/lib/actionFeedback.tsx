import { Button, Group, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";

export function showActionFeedback(message: string, undo?: () => Promise<void>) {
  if (!undo) {
    notifications.show({ message, color: "blue", autoClose: 8000 });
    return;
  }

  let busy = false;
  const id = notifications.show({
    color: "teal",
    autoClose: 8000,
    message: (
      <Group justify="space-between" gap="sm" wrap="nowrap">
        <Text size="sm">{message}</Text>
        <Button
          size="compact-sm"
          variant="subtle"
          onClick={async () => {
            if (busy) return;
            busy = true;
            notifications.update({
              id,
              message,
              loading: true,
              autoClose: false,
              allowClose: false,
            });
            try {
              await undo();
              notifications.update({
                id,
                message: "Action undone.",
                color: "blue",
                loading: false,
                autoClose: 8000,
                allowClose: true,
              });
            } catch (error) {
              notifications.update({
                id,
                message: error instanceof Error ? error.message : "Could not undo this action.",
                color: "red",
                loading: false,
                autoClose: 8000,
                allowClose: true,
              });
            }
          }}
        >
          Undo
        </Button>
      </Group>
    ),
  });
}
