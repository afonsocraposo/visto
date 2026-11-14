import { Button, Group, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";

const feedbackDuration = 5000;

export function showActionFeedback(message: string, undo?: () => Promise<void>) {
  // Drop queued feedback before adding another action so old messages do not pile up.
  notifications.cleanQueue();
  if (!undo) {
    notifications.show({
      id: `action:${message}`,
      message,
      color: "blue",
      autoClose: feedbackDuration,
    });
    return;
  }

  let busy = false;
  const id = notifications.show({
    color: "teal",
    autoClose: feedbackDuration,
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
                autoClose: feedbackDuration,
                allowClose: true,
              });
            } catch (error) {
              notifications.update({
                id,
                message: error instanceof Error ? error.message : "Could not undo this action.",
                color: "red",
                loading: false,
                autoClose: feedbackDuration,
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
