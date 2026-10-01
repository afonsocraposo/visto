import type { ReactNode } from "react";
import { Paper, Text, ThemeIcon } from "@mantine/core";

/** An empty result that explains what will appear and, where possible, how to continue. */
export function EmptyState({
  title,
  detail,
  icon,
  action,
}: {
  title: string;
  detail?: string;
  icon?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <Paper className="empty-state" withBorder p="xl" mt="md" radius="lg" ta="center">
      {icon && (
        <ThemeIcon className="empty-state-icon" size={44} radius="xl" variant="light" color="gray">
          {icon}
        </ThemeIcon>
      )}
      <Text fw={700}>{title}</Text>
      {detail && (
        <Text className="empty-state-detail" size="sm" c="dimmed" mt={6}>
          {detail}
        </Text>
      )}
      {action && <div className="empty-state-action">{action}</div>}
    </Paper>
  );
}
