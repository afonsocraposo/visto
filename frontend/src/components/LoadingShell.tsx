import { Skeleton } from "@mantine/core";

/** First load of the app: the Visto mark, appearing only if loading takes a moment. */
export function AppLoadingShell() {
  return (
    <div className="app-loading" role="status" aria-label="Loading Visto">
      <img className="app-loading-mark" src="/icon.svg?v=3" alt="" aria-hidden="true" />
      <span className="app-loading-name" aria-hidden="true">
        Visto
      </span>
    </div>
  );
}

/** While a page's code loads: a quiet heading-and-content placeholder instead of a spinner. */
export function PageLoadingShell() {
  return (
    <div className="page-loading" role="status" aria-label="Loading">
      <Skeleton height={32} width={180} mb="lg" />
      <Skeleton height={14} width="70%" mb={10} />
      <Skeleton height={14} width="55%" mb={28} />
      <Skeleton height={120} radius="md" />
    </div>
  );
}
