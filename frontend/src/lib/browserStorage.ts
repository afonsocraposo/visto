type BrowserStorageKind = "local" | "session";

function browserStorage(kind: BrowserStorageKind): Storage | undefined {
  if (typeof window === "undefined") return undefined;
  try {
    return kind === "local" ? window.localStorage : window.sessionStorage;
  } catch {
    return undefined;
  }
}

export function readStoredChoice<T extends string>(
  kind: BrowserStorageKind,
  key: string,
  allowed: readonly T[],
  fallback: T,
): T {
  try {
    const value = browserStorage(kind)?.getItem(key);
    return value && allowed.includes(value as T) ? (value as T) : fallback;
  } catch {
    return fallback;
  }
}

export function writeStoredChoice(kind: BrowserStorageKind, key: string, value: string): void {
  try {
    browserStorage(kind)?.setItem(key, value);
  } catch {
    // Storage can be unavailable in private browsing or restricted web views.
  }
}
