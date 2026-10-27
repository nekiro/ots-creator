import { clamp } from "../util/math";

export interface ListLayout {
  columns?: number;
  pageRows?: number;
}

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

export function nextIndex(key: string, index: number, count: number, layout: ListLayout = {}): number | null {
  const step = keyStep(key, layout);
  if (step === null) return null;
  return clamp(index + step, 0, count - 1);
}
