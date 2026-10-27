// Per-user view preferences in localStorage. Storage can be unavailable
// (private windows, blocked site data), so reads fall back and writes are
// best effort.

export function readStored(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function writeStored(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    /* ignore */
  }
}

export function readPositive(key: string): number | null {
  const value = Number(readStored(key));
  return value > 0 ? value : null;
}

export function readJson(key: string): unknown {
  try {
    return JSON.parse(readStored(key) ?? "null");
  } catch {
    return null;
  }
}

export function loadJson<T>(key: string, fallback: T): T {
  try {
    const stored = readStored(key);
    if (stored) return { ...fallback, ...JSON.parse(stored) } as T;
  } catch {
    /* unreadable */
  }
  return fallback;
}

export function saveJson(key: string, value: unknown): void {
  writeStored(key, JSON.stringify(value));
}
