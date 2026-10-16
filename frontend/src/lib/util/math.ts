// Small number helpers shared across the app.

export function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}
