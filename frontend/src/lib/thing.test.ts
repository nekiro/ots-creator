import { describe, expect, it } from "vitest";
import { Category, Format, type FrameGroup, type Thing } from "./api";
import { defaultDirection, frameGroupAt, hasImprovedAnimations } from "./thing";

const group = (patternX: number) => ({ patternX }) as FrameGroup;

describe("frameGroupAt", () => {
  const thing = { frameGroups: [group(1), group(4)] };

  it("picks the group at the index", () => {
    expect(frameGroupAt(thing, 1)).toBe(thing.frameGroups[1]);
  });

  it("falls back to the last group", () => {
    expect(frameGroupAt(thing, 5)).toBe(thing.frameGroups[1]);
  });

  it("is null without an object or groups", () => {
    expect(frameGroupAt(null, 0)).toBeNull();
    expect(frameGroupAt({ frameGroups: [] }, 0)).toBeNull();
  });
});

describe("hasImprovedAnimations", () => {
  it("is true for asset clients and dat clients with the feature", () => {
    expect(hasImprovedAnimations({ format: Format.FormatAssets, features: null })).toBe(true);
    expect(hasImprovedAnimations({ format: Format.FormatDat, features: { improvedAnimations: true } })).toBe(true);
  });

  it("is false otherwise", () => {
    expect(hasImprovedAnimations({ format: Format.FormatDat, features: { improvedAnimations: false } })).toBe(false);
    expect(hasImprovedAnimations(null)).toBe(false);
  });
});

describe("defaultDirection", () => {
  const outfit = (patternX: number) => ({ category: Category.CategoryOutfit, frameGroups: [group(patternX)] }) as Pick<Thing, "category" | "frameGroups">;

  it("faces outfits with four directions south", () => {
    expect(defaultDirection(outfit(4))).toBe(2);
  });

  it("keeps the first pattern otherwise", () => {
    expect(defaultDirection(outfit(1))).toBe(0);
    expect(defaultDirection({ category: Category.CategoryItem, frameGroups: [group(4)] } as Thing)).toBe(0);
    expect(defaultDirection(null)).toBe(0);
  });
});
