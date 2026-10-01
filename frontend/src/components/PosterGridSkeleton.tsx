import { Skeleton } from "@mantine/core";

export function PosterGridSkeleton({
  count = 12,
  className = "poster-grid",
  caption = false,
}: {
  count?: number;
  className?: string;
  /** Match the "caption" poster variant, which has its title below the artwork. */
  caption?: boolean;
}) {
  return (
    <div className={className} aria-busy="true">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i}>
          <Skeleton radius="md" style={{ aspectRatio: "2 / 3", width: "100%" }} />
          {caption && (
            <>
              <Skeleton height={12} width="80%" mt={8} />
              <Skeleton height={10} width="35%" mt={6} />
            </>
          )}
        </div>
      ))}
    </div>
  );
}
