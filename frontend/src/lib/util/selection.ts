// Multi-selection of list items by id: Ctrl toggles, Shift selects a range.

/** The selection with id added, or removed when it was selected. */
export function toggleId(selection: number[], id: number): number[] {
  return selection.includes(id) ? selection.filter((selected) => selected !== id) : [...selection, id];
}

/** Items from one index to the other, both included, in list order. */
export function sliceBetween<T>(items: T[], fromIndex: number, toIndex: number): T[] {
  return items.slice(Math.min(fromIndex, toIndex), Math.max(fromIndex, toIndex) + 1);
}

/** Every whole number from one to the other, both included, ascending. */
export function idRange(from: number, to: number): number[] {
  const [low, high] = from < to ? [from, to] : [to, from];
  return Array.from({ length: high - low + 1 }, (_, i) => low + i);
}

/** A range selection that keeps its anchor first. */
export function anchoredRange(anchor: number, range: number[]): number[] {
  return [anchor, ...range.filter((id) => id !== anchor)];
}
