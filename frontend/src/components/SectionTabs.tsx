import { useRef, type ReactNode } from "react";

type Option<T extends string> = { value: T; label: string; icon?: ReactNode };

/**
 * The quiet segmented tabs shared by Watching, Activity and Settings. Render the panel inside a
 * `<div key={value} className="section-panel">` so switching gets the short shared fade.
 */
export function SectionTabs<T extends string>({
  label,
  value,
  onChange,
  options,
  trailing,
}: {
  label: string;
  value: T;
  onChange: (value: T) => void;
  options: ReadonlyArray<Option<T>>;
  trailing?: ReactNode;
}) {
  const buttons = useRef<Array<HTMLButtonElement | null>>([]);
  const index = Math.max(
    0,
    options.findIndex((option) => option.value === value),
  );
  const move = (next: number) => {
    const target = (next + options.length) % options.length;
    onChange(options[target].value);
    buttons.current[target]?.focus();
  };

  return (
    <div className="section-tabs-row">
      <div
        className="section-tabs"
        role="tablist"
        aria-label={label}
        style={{ "--tab-count": options.length, "--tab-index": index } as React.CSSProperties}
      >
        <span className="section-tabs-indicator" aria-hidden="true" />
        {options.map((option, i) => (
          <button
            key={option.value}
            ref={(element) => {
              buttons.current[i] = element;
            }}
            type="button"
            role="tab"
            className="section-tab"
            aria-selected={option.value === value}
            tabIndex={option.value === value ? 0 : -1}
            onClick={() => onChange(option.value)}
            onKeyDown={(event) => {
              if (event.key === "ArrowRight") move(i + 1);
              else if (event.key === "ArrowLeft") move(i - 1);
              else return;
              event.preventDefault();
            }}
          >
            {option.icon}
            <span>{option.label}</span>
          </button>
        ))}
      </div>
      {trailing}
    </div>
  );
}
