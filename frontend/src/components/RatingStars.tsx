import { useState } from "react";
import { ActionIcon } from "@mantine/core";
import { IconStarFilled } from "@tabler/icons-react";

type Props = {
  value?: number | null;
  onChange?: (value: number | null) => void;
  label?: string;
  disabled?: boolean;
  readOnly?: boolean;
  size?: "sm" | "md" | "lg";
};

const dimensions = {
  sm: { button: 28, icon: 17 },
  md: { button: 36, icon: 22 },
  lg: { button: 42, icon: 26 },
} as const;

export function RatingStars({ value, onChange, label = "Rating", disabled = false, readOnly = false, size = "sm" }: Props) {
  const [hovered, setHovered] = useState<number | null>(null);
  const shownValue = hovered ?? value ?? 0;
  const { button, icon } = dimensions[size];

  if (readOnly) {
    return <div className="rating-stars rating-stars-readonly" role="img" aria-label={`${label}: ${value ?? 0} out of 5 stars`}>
      {[1, 2, 3, 4, 5].map(star => <span key={star} className={`rating-star${star <= (value ?? 0) ? " is-filled" : ""}`} aria-hidden="true">
        <IconStarFilled size={icon} />
      </span>)}
    </div>;
  }

  return <div className="rating-stars" role="group" aria-label={label} onMouseLeave={() => setHovered(null)}>
    {[1, 2, 3, 4, 5].map(star => {
      const filled = star <= shownValue;
      const clearsRating = value === star;
      return <ActionIcon
        key={star}
        className={`rating-star-button${filled ? " is-filled" : ""}`}
        variant="subtle"
        size={button}
        aria-label={`${label}: ${star} ${star === 1 ? "star" : "stars"}${clearsRating ? ", clear rating" : ""}`}
        aria-pressed={value === star}
        title={clearsRating ? `Clear ${label.toLowerCase()}` : `${star} ${star === 1 ? "star" : "stars"}`}
        disabled={disabled}
        onMouseEnter={() => setHovered(star)}
        onFocus={() => setHovered(star)}
        onBlur={() => setHovered(null)}
        onClick={() => {
          setHovered(null);
          onChange?.(clearsRating ? null : star);
        }}
      >
        <IconStarFilled size={icon} />
      </ActionIcon>;
    })}
  </div>;
}
