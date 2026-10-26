import { describe, expect, it } from "vitest";
import { Category, type Thing } from "./api";
import { clipboardThing, clipboardThings, copyThings, pasteInto } from "./clipboard";

const thing = (id: number, category: Category, ground: boolean): Thing =>
  ({ id, category, props: { ground } as Thing["props"], frameGroups: [{ width: id, sprites: [id], durations: [] } as unknown as Thing["frameGroups"][0]] }) as Thing;

describe("clipboard", () => {
  it("copies a snapshot", () => {
    const t = thing(100, Category.CategoryItem, true);
    copyThings([t]);
    t.props.ground = false;
    expect(clipboardThing()?.props.ground).toBe(true);
  });

  it("keeps several copied objects", () => {
    copyThings([thing(100, Category.CategoryItem, true), thing(101, Category.CategoryItem, false)]);
    expect(clipboardThing()).toBeNull();
    expect(clipboardThings().map((copied) => copied.id)).toEqual([100, 101]);
  });

  it("pastes properties onto any category and keeps the target sprites", () => {
    const out = pasteInto(thing(5, Category.CategoryOutfit, false), thing(100, Category.CategoryItem, true), "properties");
    expect(out.id).toBe(5);
    expect(out.props.ground).toBe(true);
    expect(out.frameGroups[0].sprites).toEqual([5]);
  });

  it("pastes a whole object with the target id", () => {
    const out = pasteInto(thing(101, Category.CategoryItem, false), thing(100, Category.CategoryItem, true), "object");
    expect(out.id).toBe(101);
    expect(out.frameGroups[0].sprites).toEqual([100]);
    expect(() => pasteInto(thing(1, Category.CategoryOutfit, false), thing(100, Category.CategoryItem, true), "object")).toThrow();
  });
});
