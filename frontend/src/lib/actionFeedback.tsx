import { Button, Group, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";

const feedbackDuration = 5000;

export function showActionFeedback(message: string, undo?: () => Promise<void>) {
  if (!undo) {
    notifications.show({ message, color: "blue", autoClose: feedbackDuration });
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
