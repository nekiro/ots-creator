import { describe, expect, it } from "vitest";
import type { SpriteCache } from "../../render/sprites";
import { composedKey, composeOutfit, packColors, unpackColors } from "./composed";

function sprites(pixels: Record<number, number[]>): SpriteCache {
  const entries = new Map(Object.entries(pixels).map(([id, rgba]) => [Number(id), { pixels: new Uint8ClampedArray(rgba) }]));
  return { get: (id: number) => entries.get(id) } as unknown as SpriteCache;
}

describe("packColors", () => {
  it("packs head, body, legs and feet into one byte each", () => {
    expect(packColors({ head: 1, body: 2, legs: 3, feet: 4 })).toBe(0x04030201);
  });

  it("round trips through unpackColors, the feet byte included", () => {
    const colors = { head: 132, body: 0, legs: 79, feet: 255 };
    expect(unpackColors(packColors(colors) >>> 0)).toEqual(colors);
  });
});

describe("composedKey", () => {
  it("names the sprite, the template and each color", () => {
    expect(composedKey(10, 11, packColors({ head: 1, body: 2, legs: 3, feet: 4 }))).toBe("o:10:11:1.2.3.4");
  });
});

describe("composeOutfit", () => {
  const colors = { head: 0, body: 0, legs: 0, feet: 0 };

  it("returns the sprite itself without a template", () => {
    const cache = sprites({ 10: [1, 2, 3, 255] });
    expect(composeOutfit(cache, 10, 0, colors)).toBe(cache.get(10)!.pixels);
  });

  it("returns a colored copy with a template, leaving the sprite as it was", () => {
    // A yellow template pixel marks the head, which gets colored.
    const cache = sprites({ 10: [200, 200, 200, 255], 11: [255, 255, 0, 255] });
    const out = composeOutfit(cache, 10, 11, { head: 94, body: 0, legs: 0, feet: 0 })!;
    expect(out).not.toBe(cache.get(10)!.pixels);
    expect(Array.from(cache.get(10)!.pixels)).toEqual([200, 200, 200, 255]);
    expect(Array.from(out)).not.toEqual([200, 200, 200, 255]);
  });

  it("is undefined while the sprite or the template is not loaded", () => {
    expect(composeOutfit(sprites({ 11: [0, 0, 0, 255] }), 10, 11, colors)).toBeUndefined();
    expect(composeOutfit(sprites({ 10: [0, 0, 0, 255] }), 10, 11, colors)).toBeUndefined();
  });
});
