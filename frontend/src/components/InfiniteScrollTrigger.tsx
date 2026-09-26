import { useEffect } from "react";
import { useIntersection } from "@mantine/hooks";
import { Alert, Button, Group, Loader } from "@mantine/core";

export function InfiniteScrollTrigger({
  hasNextPage,
  isFetchingNextPage,
  isFetchNextPageError,
  fetchNextPage,
}: {
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  isFetchNextPageError: boolean;
  fetchNextPage: () => void;
}) {
  const { ref, entry } = useIntersection<HTMLDivElement>({ rootMargin: "400px" });
  useEffect(() => {
    if (entry?.isIntersecting && hasNextPage && !isFetchingNextPage && !isFetchNextPageError) {
      fetchNextPage();
    }
  }, [entry?.isIntersecting, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage]);
  if (!hasNextPage) return null;
  return (
    <div ref={ref} aria-live="polite">
      {isFetchNextPageError ? (
        <Alert color="red" mt="md">
          Could not load more.{" "}
          <Button size="compact-sm" onClick={fetchNextPage}>
            Retry
          </Button>
        </Alert>
      ) : isFetchingNextPage ? (
        <Group justify="center" mt="md">
          <Loader size="sm" />
        </Group>
      ) : null}
    </div>
  );
}
