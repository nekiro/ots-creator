// Numbers and dates as text.

/** "512 B" or "1.5 KB". */
export function formatKb(bytes: number): string {
  return bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KB`;
}

/** A date (ISO text) in the user's locale, without the time. */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString();
}

/** Short text of ids: "#5", "#5-#8" or "#5, #9, #12 (+3)". */
export function formatIdList(ids: number[]): string {
  const sorted = [...ids].sort((first, second) => first - second);
  if (sorted.every((id, i) => id === sorted[0] + i)) return sorted.length === 1 ? `#${sorted[0]}` : `#${sorted[0]}-#${sorted.at(-1)}`;
  const head = sorted.slice(0, 3).map((id) => `#${id}`).join(", ");
  return sorted.length > 3 ? `${head} (+${sorted.length - 3})` : head;
}
