import { useLayoutEffect, useRef, useState } from "react";

/** Text clamped to a few lines with a "More" toggle that appears only when it overflows. */
export function ClampedText({
  children,
  className = "",
  lines = 3,
}: {
  children: string;
  className?: string;
  lines?: number;
}) {
  const ref = useRef<HTMLParagraphElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflows, setOverflows] = useState(false);

  useLayoutEffect(() => {
    setExpanded(false);
  }, [children]);

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element || expanded) return;
    const measure = () => setOverflows(element.scrollHeight > element.clientHeight + 1);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [children, expanded, lines]);

  return (
    <div className={`clamped-text ${className}`.trim()}>
      <p
        ref={ref}
        className="clamped-text-body"
        style={expanded ? undefined : { WebkitLineClamp: lines }}
        data-expanded={expanded || undefined}
      >
        {children}
      </p>
      {(overflows || expanded) && (
        <button
          type="button"
          className="clamped-text-toggle"
          aria-expanded={expanded}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? "Less" : "More"}
        </button>
      )}
    </div>
  );
}
