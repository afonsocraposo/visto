import { ActionIcon, Group, Tooltip } from "@mantine/core";
import { IconStar, IconStarFilled } from "@tabler/icons-react";

type Props = {
  value?: number | null;
  onChange?: (value: number | null) => void;
  label?: string;
  disabled?: boolean;
  size?: "sm" | "md" | "lg";
};

export function RatingStars({ value, onChange, label = "Rating", disabled = false, size = "sm" }: Props) {
  return <Group className="rating-stars" gap={2} aria-label={label} role="group">
    {[1, 2, 3, 4, 5].map(star => {
      const active = Boolean(value && star <= value);
      return <Tooltip key={star} label={`${star} ${star === 1 ? "star" : "stars"}`} withArrow>
        <ActionIcon
          variant="subtle"
          color={active ? "yellow" : "gray"}
          size={size}
          aria-label={`${label}: ${star} ${star === 1 ? "star" : "stars"}`}
          disabled={disabled}
          onClick={() => onChange?.(value === star ? null : star)}
        >
          {active ? <IconStarFilled size={size === "lg" ? 20 : 16} /> : <IconStar size={size === "lg" ? 20 : 16} />}
        </ActionIcon>
      </Tooltip>;
    })}
  </Group>;
}
