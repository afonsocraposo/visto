import { Paper, Text } from "@mantine/core";

export function EmptyState({ title }: { title: string }) {
  return (
    <Paper withBorder p="xl" mt="md" radius="md" ta="center">
      <Text fw={700}>{title}</Text>
    </Paper>
  );
}
