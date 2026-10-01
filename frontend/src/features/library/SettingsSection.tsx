import { useEffect, useRef, useState, type ReactNode } from "react";
import { Skeleton, Text, Title } from "@mantine/core";

/** One titled group on the Settings page. The id doubles as the in-page anchor. */
export function SettingsSection({
  id,
  title,
  description,
  children,
}: {
  id: string;
  title: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section
      id={`settings-${id}`}
      className="settings-section"
      aria-labelledby={`settings-${id}-title`}
    >
      <Title order={2} id={`settings-${id}-title`}>
        {title}
      </Title>
      {description && (
        <Text size="sm" c="dimmed" className="settings-section-description">
          {description}
        </Text>
      )}
      {children}
    </section>
  );
}

/** A divided part of a section, such as Pushover inside Notifications. */
export function SettingsSubsection({
  title,
  description,
  children,
}: {
  title: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="settings-subsection">
      <Title order={3}>{title}</Title>
      {description && (
        <Text size="sm" c="dimmed" mt={4}>
          {description}
        </Text>
      )}
      {children}
    </div>
  );
}

/** Placeholder rows matching `.settings-list` while a list loads. */
export function SettingsListSkeleton() {
  return (
    <div className="settings-list" aria-hidden="true">
      {[0, 1].map((row) => (
        <div key={row} className="settings-list-row">
          <div className="settings-list-text">
            <Skeleton h={14} w="40%" radius="sm" />
            <Skeleton h={10} w="70%" radius="sm" mt={8} />
          </div>
        </div>
      ))}
    </div>
  );
}

/** Desktop-only jump list. Highlights the section being read. */
export function SettingsIndex({ sections }: { sections: { id: string; label: string }[] }) {
  const [active, setActive] = useState(sections[0]?.id);
  // A section picked from the list that is too close to the end to reach the top.
  const picked = useRef<string | null>(null);

  useEffect(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      const elements = sections
        .map((section) => document.getElementById(`settings-${section.id}`))
        .filter((element): element is HTMLElement => element !== null);
      if (!elements.length) return;
      const root = document.documentElement;
      // At the end of the page the last sections can never reach the top; pick the last one.
      const atBottom = window.innerHeight + window.scrollY >= root.scrollHeight - 4;
      const line = window.innerHeight * 0.3;
      if (!atBottom) picked.current = null;
      if (atBottom && picked.current) {
        setActive(picked.current);
        return;
      }
      const current = atBottom
        ? elements[elements.length - 1]
        : (elements.filter((element) => element.getBoundingClientRect().top <= line).pop() ??
          elements[0]);
      setActive(current.id.replace(/^settings-/, ""));
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    update();
    window.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("resize", schedule);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
    };
  }, [sections]);

  return (
    <nav className="settings-index" aria-label="Settings sections">
      {sections.map((section) => (
        <a
          key={section.id}
          href={`#settings-${section.id}`}
          className="settings-index-link"
          aria-current={active === section.id ? "location" : undefined}
          onClick={(event) => {
            const target = document.getElementById(`settings-${section.id}`);
            if (!target) return;
            event.preventDefault();
            picked.current = section.id;
            setActive(section.id);
            target.scrollIntoView({ block: "start" });
          }}
        >
          {section.label}
        </a>
      ))}
    </nav>
  );
}
