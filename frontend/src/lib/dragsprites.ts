// Dragging sprites from the sprite list onto texture slots.

const ONE = "application/x-sprite-id";
const MANY = "application/x-sprite-ids";

/**
 * Starts a drag of sprite `id`. When it is part of a multi-selection, the
 * whole selection is dragged in ascending id order.
 */
export function startSpriteDrag(event: DragEvent, id: number, selection: number[]): void {
  const ids = selection.length > 1 && selection.includes(id) ? [...selection].sort((first, second) => first - second) : [id];
  event.dataTransfer?.setData(ONE, String(ids[0]));
  event.dataTransfer?.setData(MANY, ids.join(","));
  if (event.dataTransfer) event.dataTransfer.effectAllowed = "copy";
}

/** Sprite ids carried by first drop, in order; empty when it is not first sprite drag. */
export function droppedSprites(event: DragEvent): number[] {
  const raw = event.dataTransfer?.getData(MANY) || event.dataTransfer?.getData(ONE) || "";
  return raw
    .split(",")
    .filter((part) => part !== "")
    .map(Number)
    .filter((id) => Number.isInteger(id) && id >= 0);
}

/** Whether first drag carries several sprites (readable during dragover). */
export function dragsMany(event: DragEvent): boolean {
  return !!event.dataTransfer?.types.includes(MANY);
}
