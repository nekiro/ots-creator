import { colorize } from "../../render/outfit";
import type { SpriteCache } from "../../render/sprites";
import type { CreatureData } from "../data/chunks";

export type OutfitColors = Pick<CreatureData, "head" | "body" | "legs" | "feet">;

/** Atlas key of a colored outfit piece (colors: head, body, legs, feet bytes). */
export function composedKey(spriteId: number, templateId: number, colors: number): string {
  return `o:${spriteId}:${templateId}:${colors & 255}.${(colors >> 8) & 255}.${(colors >> 16) & 255}.${colors >>> 24}`;
}

export function packColors(colors: OutfitColors): number {
  return colors.head | (colors.body << 8) | (colors.legs << 16) | (colors.feet << 24);
}

export function unpackColors(packed: number): OutfitColors {
  return { head: packed & 255, body: (packed >> 8) & 255, legs: (packed >> 16) & 255, feet: packed >>> 24 };
}

/** The pixels of an outfit piece colored with its template (undefined: a sprite is not loaded). */
export function composeOutfit(sprites: SpriteCache, spriteId: number, templateId: number, colors: OutfitColors): Uint8ClampedArray | undefined {
  const base = sprites.get(spriteId)?.pixels;
  if (!base) return undefined;
  if (!templateId) return base;
  const template = sprites.get(templateId)?.pixels;
  if (!template) return undefined;
  const out = new Uint8ClampedArray(base);
  colorize(out, template, { head: colors.head, body: colors.body, legs: colors.legs, feet: colors.feet });
  return out;
}
