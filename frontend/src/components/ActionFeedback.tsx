import { useEffect, useState } from "react";
import { Alert, Button, Group, Text } from "@mantine/core";

type Feedback = { message: string; undo?: () => Promise<void> };
const feedbackEvent = "visto:action-feedback";

export function showActionFeedback(message: string, undo?: () => Promise<void>) {
  window.dispatchEvent(new CustomEvent<Feedback>(feedbackEvent, { detail: { message, undo } }));
}

export function ActionFeedback() {
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const onFeedback = (event: Event) => {
      setFeedback((event as CustomEvent<Feedback>).detail);
      setBusy(false);
    };
    window.addEventListener(feedbackEvent, onFeedback);
    return () => window.removeEventListener(feedbackEvent, onFeedback);
  }, []);

  useEffect(() => {
    if (!feedback || busy) return;
    const timeout = window.setTimeout(() => setFeedback(null), 8000);
    return () => window.clearTimeout(timeout);
  }, [feedback, busy]);

  if (!feedback) return null;
  return (
    <Alert
      color={feedback.undo ? "teal" : "blue"}
      role="status"
      style={{
        position: "fixed",
        bottom: 88,
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: 1000,
        width: "min(92vw, 440px)",
        boxShadow: "0 8px 28px rgba(0, 0, 0, 0.2)",
      }}
    >
      <Group justify="space-between" gap="sm" wrap="nowrap">
        <Text size="sm">{feedback.message}</Text>
        {feedback.undo && (
          <Button
            size="compact-sm"
            variant="subtle"
            loading={busy}
            onClick={async () => {
              setBusy(true);
              try {
                await feedback.undo?.();
                setFeedback({ message: "Action undone." });
              } catch (error) {
                setFeedback({
                  message: error instanceof Error ? error.message : "Could not undo this action.",
                });
              } finally {
                setBusy(false);
              }
            }}
          >
            Undo
          </Button>
        )}
      </Group>
    </Alert>
  );
}
