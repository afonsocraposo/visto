import { Paper, Text } from "@mantine/core";

export function EmptyState({ title, detail }: { title: string; detail?: string }) {
  return (
    <Paper className="empty-state" withBorder p="xl" mt="md" radius="lg" ta="center">
      <Text fw={700}>{title}</Text>
      {detail && <Text size="sm" c="dimmed" mt={6}>{detail}</Text>}
    </Paper>
  );
}
