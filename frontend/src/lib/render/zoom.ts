// Zoom levels of image views: the object preview, the sheet editor and the
// export and viewer dialogs.

/** Zoom steps of image views: fine below 1x, whole pixels above 2x. */
export const ZOOM_LEVELS = [0.1, 0.15, 0.2, 0.25, 0.33, 0.5, 0.67, 0.75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10, 12, 16];

/** The next zoom level from zoom in a direction (+1 in, -1 out). */
export function stepZoom(zoom: number, direction: number): number {
  const epsilon = 1e-6;
  if (direction > 0) return ZOOM_LEVELS.find((level) => level > zoom + epsilon) ?? ZOOM_LEVELS[ZOOM_LEVELS.length - 1];
  return [...ZOOM_LEVELS].reverse().find((level) => level < zoom - epsilon) ?? ZOOM_LEVELS[0];
}

/** "3×" from 1x up, "50%" below. */
export function zoomLabel(zoom: number): string {
  return zoom >= 1 ? `${Math.round(zoom * 100) / 100}×` : `${Math.round(zoom * 100)}%`;
}

/**
 * Zoom that fits content into a stage (minus a margin): whole steps up to
 * max when it is 1x or more, so pixels stay even; below that the exact
 * ratio, but not under min.
 */
export function fitZoom(contentWidth: number, contentHeight: number, stageWidth: number, stageHeight: number, max = 8, margin = 16, min = ZOOM_LEVELS[0]): number {
  if (!contentWidth || !contentHeight || !stageWidth || !stageHeight) return 1;
  const ratio = Math.min((stageWidth - margin) / contentWidth, (stageHeight - margin) / contentHeight);
  return ratio >= 1 ? Math.min(max, Math.floor(ratio)) : Math.max(min, ratio);
}
