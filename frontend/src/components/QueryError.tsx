import { useState } from "react";
import { Alert, Button, Group, Text } from "@mantine/core";
import { IconAlertTriangle, IconRefresh } from "@tabler/icons-react";

/** A failed load with a local retry, used wherever the query can simply be repeated. */
export function QueryError({
  message,
  onRetry,
  mt = "md",
}: {
  message: string;
  onRetry?: () => unknown;
  mt?: string | number;
}) {
  const [retrying, setRetrying] = useState(false);
  return (
    <Alert
      className="query-error"
      color="red"
      variant="light"
      mt={mt}
      icon={<IconAlertTriangle size={18} />}
    >
      <Group justify="space-between" align="center" gap="sm">
        <Text size="sm">{message}</Text>
        {onRetry && (
          <Button
            size="xs"
            variant="default"
            leftSection={<IconRefresh size={14} />}
            loading={retrying}
            onClick={async () => {
              setRetrying(true);
              try {
                await onRetry();
              } finally {
                setRetrying(false);
              }
            }}
          >
            Try again
          </Button>
        )}
      </Group>
    </Alert>
  );
}
