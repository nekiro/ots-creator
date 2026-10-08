// Geometry of an OTClient HorizontalScrollBar used as a value slider.

export const THUMB_MIN = 12;

/** Thumb width: one step of the track per value, never below THUMB_MIN. */
export function thumbWidth(track: number, min: number, max: number): number {
  const steps = Math.max(1, max - min + 1);
  return Math.min(track, Math.max(THUMB_MIN, Math.round(track / steps)));
}

/** Thumb offset inside the track for a value. */
export function thumbOffset(value: number, track: number, min: number, max: number): number {
  if (max <= min) return 0;
  const free = track - thumbWidth(track, min, max);
  return Math.round(((clamp(value, min, max) - min) / (max - min)) * free);
}

/** Value for a pointer at x (track coordinates), centred on the thumb. */
export function valueAt(x: number, track: number, min: number, max: number): number {
  if (max <= min) return min;
  const w = thumbWidth(track, min, max);
  const free = track - w;
  if (free <= 0) return min;
  return clamp(Math.round(min + ((x - w / 2) / free) * (max - min)), min, max);
}

export function clamp(v: number, min: number, max: number): number {
  return Math.min(Math.max(v, min), max);
}
