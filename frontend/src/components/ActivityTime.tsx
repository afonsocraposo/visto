import { formatActivityTime } from "../lib/time";

type Props = {
  value: string | Date;
};

export function ActivityTime({ value }: Props) {
  const date = value instanceof Date ? value : new Date(value);

  return (
    <time
      dateTime={date.toISOString()}
      title={date.toLocaleString(undefined, { dateStyle: "full", timeStyle: "long" })}
    >
      {formatActivityTime(date)}
    </time>
  );
}
