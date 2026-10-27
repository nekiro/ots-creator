import { Category, Format, type Features, type FrameGroup, type Thing } from "./api";

export function frameGroupAt(thing: Pick<Thing, "frameGroups"> | null | undefined, index: number): FrameGroup | null {
  if (!thing) return null;
  return thing.frameGroups[Math.min(index, thing.frameGroups.length - 1)] ?? null;
}

/** Clients with per-frame durations: asset clients and dat clients with the feature. */
export function hasImprovedAnimations(info: { format: Format; features?: Pick<Features, "improvedAnimations"> | null } | null | undefined): boolean {
  return !!info && (info.format === Format.FormatAssets || !!info.features?.improvedAnimations);
}

export function defaultDirection(thing: Pick<Thing, "category" | "frameGroups"> | null | undefined): number {
  return thing?.category === Category.CategoryOutfit && (thing.frameGroups[0]?.patternX ?? 1) > 2 ? 2 : 0;
}
