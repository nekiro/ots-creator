export function formatKb(bytes: number): string {
  return bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KB`;
}

export function plural(count: number, word: string): string {
  return `${count.toLocaleString()} ${word}${count === 1 ? "" : "s"}`;
}

export function formatSize(bytes: number): string {
  return bytes < 1048576 ? `${(bytes / 1024).toFixed(1)} KB` : `${(bytes / 1048576).toFixed(1)} MB`;
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString();
}

export function formatIdList(ids: number[]): string {
  const sorted = [...ids].sort((first, second) => first - second);
  if (sorted.every((id, i) => id === sorted[0] + i)) return sorted.length === 1 ? `#${sorted[0]}` : `#${sorted[0]}-#${sorted.at(-1)}`;
  const head = sorted.slice(0, 3).map((id) => `#${id}`).join(", ");
  return sorted.length > 3 ? `${head} (+${sorted.length - 3})` : head;
}
