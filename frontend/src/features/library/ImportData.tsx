import { useRef, useState } from "react";
import { Alert, Button, Group, List, Modal, Stack, Text } from "@mantine/core";
import { IconUpload } from "@tabler/icons-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { request } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";

type ImportResult = {
  titles: number;
  watches: number;
  ratings: number;
  skipped: number;
  unsupported: number;
  issues: { item: string; reason: string }[];
};

export function ImportData({
  withinModal = false,
  prominent = false,
  refreshLibraryOnClose = false,
}: {
  withinModal?: boolean;
  prominent?: boolean;
  refreshLibraryOnClose?: boolean;
}) {
  const [selected, setSelected] = useState(false);
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const sourceButton = (
    <Button
      variant={prominent ? "filled" : "default"}
      size="lg"
      className="import-source-button"
      leftSection={<img src="/bingers.svg" alt="" width={32} height={32} />}
      onClick={() => setSelected(true)}
    >
      {prominent ? "Import from Bingers" : "Bingers"}
    </Button>
  );

  if (withinModal) {
    return selected ? <BingersImportForm /> : sourceButton;
  }

  return (
    <>
      {sourceButton}
      <Modal
        opened={selected}
        onClose={() => {
          setSelected(false);
          if (refreshLibraryOnClose) {
            void queryClient.invalidateQueries({ queryKey: userQueryKey("library") });
          }
        }}
        title="Import from Bingers"
        centered
      >
        <BingersImportForm refreshLibraryOnClose={refreshLibraryOnClose} />
      </Modal>
    </>
  );
}

function BingersImportForm({ refreshLibraryOnClose = false }: { refreshLibraryOnClose?: boolean }) {
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [fileError, setFileError] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const upload = useMutation({
    mutationFn: async (archive: File) =>
      request<ImportResult>(
        "/api/v1/imports/bingers",
        { method: "POST", headers: { "Content-Type": "application/zip" }, body: archive },
        "Could not import the archive.",
      ),
    onSuccess: async () => {
      await Promise.all(
        ["library", "history", "continue", "calendar", "show-progress", "feed"]
          .filter((scope) => !refreshLibraryOnClose || scope !== "library")
          .map((scope) => queryClient.invalidateQueries({ queryKey: userQueryKey(scope) })),
      );
    },
  });

  const selectFile = (next: File | null) => {
    upload.reset();
    setFile(null);
    if (!next) return;
    if (!next.name.toLowerCase().endsWith(".zip")) {
      setFileError("Choose a ZIP file exported by Bingers.");
      return;
    }
    if (next.size > 5 * 1024 * 1024) {
      setFileError("The ZIP must be 5 MB or smaller.");
      return;
    }
    setFileError("");
    setFile(next);
  };

  return (
    <Stack gap="md">
      <Text size="sm" c="dimmed">
        In Bingers, open Settings and export your data. Upload the downloaded ZIP file here.
      </Text>
      <input
        ref={inputRef}
        type="file"
        accept=".zip,application/zip"
        aria-label="Bingers export ZIP"
        className="import-file-input"
        onChange={(event) => {
          selectFile(event.currentTarget.files?.[0] ?? null);
          event.currentTarget.value = "";
        }}
      />
      <button
        type="button"
        disabled={upload.isPending}
        className={`import-dropzone${dragging ? " import-dropzone-active" : ""}`}
        onClick={() => inputRef.current?.click()}
        onDragOver={(event) => {
          event.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          selectFile(event.dataTransfer.files[0] ?? null);
        }}
      >
        <IconUpload size={28} stroke={1.5} aria-hidden="true" />
        <strong>{file ? file.name : "Drop your Bingers ZIP here"}</strong>
        <span>
          {file ? "Click to choose a different file" : "or click to choose a file · Max 5 MB"}
        </span>
      </button>
      {fileError && <Alert color="red">{fileError}</Alert>}
      <Group justify="flex-end">
        <Button
          disabled={!file || upload.isSuccess}
          loading={upload.isPending}
          onClick={() => file && upload.mutate(file)}
        >
          Import data
        </Button>
      </Group>
      {upload.isError && <Alert color="red">{upload.error.message}</Alert>}
      {upload.data && (
        <Alert color="green" title="Import finished">
          {upload.data.titles} titles, {upload.data.watches} watches, and {upload.data.ratings}{" "}
          ratings imported.
          {upload.data.skipped > 0 && <Text size="sm">{upload.data.skipped} records skipped.</Text>}
          {upload.data.unsupported > 0 && (
            <Text size="sm">{upload.data.unsupported} custom list rows are not supported.</Text>
          )}
          {upload.data.issues.length > 0 && (
            <List size="sm" mt="sm">
              {upload.data.issues.map((issue, index) => (
                <List.Item key={`${issue.item}-${index}`}>
                  {issue.item}: {issue.reason}
                </List.Item>
              ))}
            </List>
          )}
        </Alert>
      )}
    </Stack>
  );
}
