import { Skeleton } from "@mantine/core";

export function PosterGridSkeleton({
  count = 12,
  className = "poster-grid",
}: {
  count?: number;
  className?: string;
}) {
  return (
    <div className={className}>
      {Array.from({ length: count }).map((_, i) => (
        <Skeleton key={i} radius="md" style={{ aspectRatio: "2 / 3", width: "100%" }} />
      ))}
    </div>
  );
}
