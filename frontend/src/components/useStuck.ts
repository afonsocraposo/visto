import { useEffect, useState } from "react";
import { headerOffset } from "../lib/headerOffset";

/**
 * Whether a sticky element is currently stuck below the app header. Attach the returned
 * `sentinel` callback ref to an empty element placed right before the sticky one.
 */
export function useStuck<T extends HTMLElement>() {
  const [element, sentinel] = useState<T | null>(null);
  const [stuck, setStuck] = useState(false);
  useEffect(() => {
    if (!element) return;
    const header = headerOffset();
    const observer = new IntersectionObserver(
      ([entry]) => setStuck(!entry.isIntersecting && entry.boundingClientRect.top < header + 1),
      { rootMargin: `-${header + 1}px 0px 0px 0px` },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);
  return { sentinel, stuck };
}
