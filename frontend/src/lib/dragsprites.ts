// Dragging sprites from the sprite list onto texture slots.

const ONE = "application/x-sprite-id";
const MANY = "application/x-sprite-ids";

/**
 * Starts a drag of sprite `id`. When it is part of a multi-selection, the
 * whole selection is dragged in ascending id order.
 */
export function startSpriteDrag(e: DragEvent, id: number, selection: number[]): void {
  const ids = selection.length > 1 && selection.includes(id) ? [...selection].sort((a, b) => a - b) : [id];
  e.dataTransfer?.setData(ONE, String(ids[0]));
  e.dataTransfer?.setData(MANY, ids.join(","));
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "copy";
}

/** Sprite ids carried by a drop, in order; empty when it is not a sprite drag. */
export function droppedSprites(e: DragEvent): number[] {
  const raw = e.dataTransfer?.getData(MANY) || e.dataTransfer?.getData(ONE) || "";
  return raw
    .split(",")
    .filter((s) => s !== "")
    .map(Number)
    .filter((n) => Number.isInteger(n) && n >= 0);
}

/** Whether a drag carries several sprites (readable during dragover). */
export function dragsMany(e: DragEvent): boolean {
  return !!e.dataTransfer?.types.includes(MANY);
}
