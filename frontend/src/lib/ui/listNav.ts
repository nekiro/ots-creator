// Arrow key navigation of lists and grids.
import { clamp } from "../util/math";

export interface ListLayout {
  /** Items per row: Up and Down move a row. Without it only Left and Right move. */
  columns?: number;
  /** Rows per page: Page Up and Page Down move a page. */
  pageRows?: number;
}

/** How far a key moves through the list, or null when it is not a navigation key. */
export function keyStep(key: string, layout: ListLayout = {}): number | null {
  const { columns, pageRows } = layout;
  switch (key) {
    case "ArrowLeft":
      return -1;
    case "ArrowRight":
      return 1;
    case "ArrowUp":
      return columns === undefined ? null : -columns;
    case "ArrowDown":
      return columns === undefined ? null : columns;
    case "PageUp":
      return columns === undefined || pageRows === undefined ? null : -columns * pageRows;
    case "PageDown":
      return columns === undefined || pageRows === undefined ? null : columns * pageRows;
  }
  return null;
}

/** The index a key moves to from index, kept in the list; null when the key does not navigate. */
export function nextIndex(key: string, index: number, count: number, layout: ListLayout = {}): number | null {
  const step = keyStep(key, layout);
  if (step === null) return null;
  return clamp(index + step, 0, count - 1);
}
