import { useState } from "react";
import { Alert, Button, Group, Skeleton, Stack, Text, Textarea, Title } from "@mantine/core";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { TitleNote as Note } from "../../generated/models/titleNote";
import { api } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";

export function TitleNote({ mediaID }: { mediaID: string }) {
  const userQueryKey = useUserQueryKey();
  const queryKey = userQueryKey("title-note", mediaID);
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<string>();
  const note = useQuery({
    queryKey,
    queryFn: () =>
      api.get<Note>(
        `/api/v1/library/${encodeURIComponent(mediaID)}/note`,
        "Could not load your note.",
      ),
  });
  const save = useMutation({
    mutationFn: (text: string) =>
      api.put<Note>(
        `/api/v1/library/${encodeURIComponent(mediaID)}/note`,
        { note: text },
        "Could not save your note.",
      ),
    onSuccess: (saved) => {
      queryClient.setQueryData(queryKey, saved);
      setDraft(undefined);
    },
  });
  const value = draft ?? note.data?.note ?? "";
  const tooLong = Array.from(value).length > 5000;
  return (
    <section className="detail-section title-note" aria-labelledby="title-note-heading">
      <Stack gap="sm">
        <div>
          <Title order={2} id="title-note-heading">
            Your note
          </Title>
          <Text size="sm" c="dimmed">
            Private. Only you can see this note.
          </Text>
        </div>
        {note.isPending ? (
          <Skeleton height={100} />
        ) : note.isError ? (
          <Alert color="red" title="Could not load your note">
            <Button variant="subtle" onClick={() => void note.refetch()}>
              Try again
            </Button>
          </Alert>
        ) : (
          <>
            <Textarea
              aria-label="Your private note"
              placeholder="What did you think?"
              autosize
              minRows={3}
              maxRows={12}
              value={value}
              onChange={(event) => {
                setDraft(event.currentTarget.value);
                save.reset();
              }}
              disabled={save.isPending}
              error={tooLong ? "Use 5,000 characters or fewer." : undefined}
            />
            <Group justify="space-between" gap="sm">
              <Text size="xs" c="dimmed" role="status">
                {value !== note.data.note
                  ? "Unsaved changes"
                  : save.isSuccess
                    ? "Saved"
                    : `${Array.from(value).length.toLocaleString()} / 5,000`}
              </Text>
              <Group gap="xs">
                {note.data.note !== "" && (
                  <Button
                    variant="subtle"
                    color="red"
                    disabled={save.isPending}
                    onClick={() => save.mutate("")}
                  >
                    Delete note
                  </Button>
                )}
                <Button
                  loading={save.isPending}
                  disabled={tooLong || value === note.data.note}
                  onClick={() => save.mutate(value)}
                >
                  Save note
                </Button>
              </Group>
            </Group>
            {save.isError && (
              <Alert color="red">{save.error.message} Your text is still here; try again.</Alert>
            )}
          </>
        )}
      </Stack>
    </section>
  );
}
