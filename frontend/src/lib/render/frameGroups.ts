// Edits of a frame group's layout and frame durations. Changing the layout
// moves the sprites to their new slots; changing the frame count keeps one
// duration per frame.
import type { FrameGroup } from "../api";
import { resizeSprites, type Layout } from "./layout";

/** Duration (ms) of new frames by object category. */
export const DEFAULT_DURATION: Record<number, number> = { 1: 500, 2: 300, 3: 100, 4: 75 };
const FALLBACK_DURATION = 100;

export type LayoutKey = keyof Layout | "exactSize";

/** Gives the group one duration per frame: new frames get the category's default. */
export function syncDurations(group: FrameGroup, category: number): void {
  if (group.frames <= 1) {
    group.durations = [];
    return;
  }
  const duration = DEFAULT_DURATION[category] ?? FALLBACK_DURATION;
  const list = [...(group.durations ?? [])];
  while (list.length < group.frames) list.push({ min: duration, max: duration });
  group.durations = list.slice(0, group.frames);
  if (group.startFrame >= group.frames) group.startFrame = 0;
}

/** Sets every frame to the same duration range (max at least min). */
export function setAllDurations(group: FrameGroup, min: number, max: number): void {
  if (!group.durations) return;
  group.durations = group.durations.map(() => ({ min, max: Math.max(min, max) }));
}

/** Changes one layout value of the group, moving its sprites to match. */
export function setLayout(group: FrameGroup, key: LayoutKey, value: number, category: number): void {
  if (key === "exactSize") {
    group.exactSize = value;
    return;
  }
  const old: Layout = { ...group };
  const next: Layout = { ...old, [key]: value };
  group.sprites = resizeSprites(old, group.sprites, next);
  (group as unknown as Record<string, number>)[key] = value;
  if (key === "frames") syncDurations(group, category);
}
