import { useRef, useState } from "react";
import { Alert, Button, Group, Skeleton, Stack, Text, Textarea, Title } from "@mantine/core";
import { IconNote, IconPencil } from "@tabler/icons-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { TitleNote as Note } from "../../generated/models/titleNote";
import { api } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";

export function TitleNote({
  mediaID,
  episodeID,
}: { mediaID: string; episodeID?: never } | { episodeID: string; mediaID?: never }) {
  const userQueryKey = useUserQueryKey();
  const resource = episodeID ? "episodes" : "library";
  const path = `/api/v1/${resource}/${encodeURIComponent(episodeID ?? mediaID!)}/note`;
  const queryKey = userQueryKey("private-note", resource, episodeID ?? mediaID);
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<string>();
  const trigger = useRef<HTMLButtonElement>(null);
  const closeEditor = () => {
    setDraft(undefined);
    requestAnimationFrame(() => trigger.current?.focus());
  };
  const note = useQuery({
    queryKey,
    queryFn: () => api.get<Note>(path, "Could not load your note."),
  });
  const save = useMutation({
    mutationFn: (text: string) => api.put<Note>(path, { note: text }, "Could not save your note."),
    onSuccess: (saved) => {
      queryClient.setQueryData(queryKey, saved);
      closeEditor();
    },
  });
  const value = draft ?? note.data?.note ?? "";
  const tooLong = Array.from(value).length > 5000;
  return (
    <section className="detail-section title-note" aria-label="Your note">
      <Stack gap="sm">
        {(draft !== undefined || note.data?.note) && (
          <div>
            <Title order={2}>Your note</Title>
            <Text size="sm" c="dimmed">
              Private. Only you can see this note.
            </Text>
          </div>
        )}
        {note.isPending ? (
          <Skeleton height={28} />
        ) : note.isError ? (
          <Alert color="red" title="Could not load your note">
            <Button variant="subtle" onClick={() => void note.refetch()}>
              Try again
            </Button>
          </Alert>
        ) : draft === undefined ? (
          <>
            {note.data.note && (
              <Text style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>
                {note.data.note}
              </Text>
            )}
            <Button
              ref={trigger}
              variant="subtle"
              color="gray"
              style={{ alignSelf: "flex-start" }}
              leftSection={note.data.note ? <IconPencil size={16} /> : <IconNote size={16} />}
              onClick={() => {
                setDraft(note.data.note);
                save.reset();
              }}
            >
              {note.data.note ? "Edit note" : "Add note"}
            </Button>
          </>
        ) : (
          <>
            <Textarea
              aria-label="Your private note"
              autoFocus
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
                  variant="subtle"
                  color="gray"
                  disabled={save.isPending}
                  onClick={() => {
                    closeEditor();
                    save.reset();
                  }}
                >
                  Cancel
                </Button>
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
