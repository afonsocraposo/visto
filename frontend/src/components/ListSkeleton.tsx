import { Paper, Skeleton } from "@mantine/core";

type RowProps = {
  rowClassName: string;
  artClassName?: string;
  contentClassName?: string;
  layoutClassName?: string;
  lines?: number;
  padded?: boolean;
};

export function ListRowSkeleton({
  rowClassName,
  artClassName,
  contentClassName,
  layoutClassName,
  lines = 2,
  padded = true,
}: RowProps) {
  const content = (
    <>
      {artClassName && (
        <div className={artClassName}>
          <Skeleton height="100%" width="100%" radius={0} />
        </div>
      )}
      <div className={contentClassName} style={{ flex: 1, minWidth: 0 }}>
        {Array.from({ length: lines }).map((_, i) => (
          <Skeleton
            key={i}
            height={i === 0 ? 16 : 12}
            width={i === 0 ? "60%" : `${90 - i * 15}%`}
            mb={i === lines - 1 ? 0 : 6}
          />
        ))}
      </div>
    </>
  );

  return (
    <Paper className={rowClassName} withBorder p={padded ? undefined : 0}>
      {layoutClassName ? <div className={layoutClassName}>{content}</div> : content}
    </Paper>
  );
}

export function ListSkeleton({ count = 4, ...rowProps }: { count?: number } & RowProps) {
  return (
    <>
      {Array.from({ length: count }).map((_, i) => (
        <ListRowSkeleton key={i} {...rowProps} />
      ))}
    </>
  );
}
