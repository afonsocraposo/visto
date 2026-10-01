import { useState } from "react";

/**
 * Artwork that loads lazily (unless eager) inside a box that already reserves its aspect ratio,
 * and fades from 0.6 to full opacity once decoded, so nothing shifts or pops.
 */
export function FadeImage({
  src,
  alt,
  eager = false,
  className = "",
}: {
  src: string;
  alt: string;
  eager?: boolean;
  className?: string;
}) {
  const [loaded, setLoaded] = useState(false);
  return (
    <img
      className={`fade-image ${className}`.trim()}
      src={src}
      alt={alt}
      loading={eager ? "eager" : "lazy"}
      decoding="async"
      data-loaded={loaded || undefined}
      ref={(image) => {
        // Cached images can be complete before React attaches onLoad.
        if (image?.complete && image.naturalWidth > 0 && !loaded) setLoaded(true);
      }}
      onLoad={() => setLoaded(true)}
      onError={() => setLoaded(true)}
    />
  );
}
