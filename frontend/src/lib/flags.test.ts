import { describe, expect, it } from "vitest";
import { FLAG_GROUPS, buildPatch, clampField, flagVisible, getPath, setPath } from "./flags";

describe("flagVisible", () => {
  it("shows item flags only for items", () => {
    expect(flagVisible("container", 1, false)).toBe(true);
    expect(flagVisible("container", 2, false)).toBe(false);
    expect(flagVisible("container", 3, false)).toBe(false);
    expect(flagVisible("hasBones", 1, false)).toBe(false);
    expect(flagVisible("hasBones", 2, false)).toBe(true);
    expect(flagVisible("hasLight", 4, false)).toBe(true);
    expect(flagVisible("topEffect", 3, false)).toBe(true);
    expect(flagVisible("topEffect", 4, false)).toBe(false);
  });

  it("always shows a flag that is set", () => {
    expect(flagVisible("container", 2, true)).toBe(true);
  });
});

describe("flag metadata", () => {
  it("has unique keys", () => {
    const keys = FLAG_GROUPS.flatMap((g) => g.flags.map((f) => f.key));
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("covers every property the backend can report", () => {
    // Names from internal/dat/flags.go attrNames.
    const backend = [
      "ground", "groundBorder", "onBottom", "onTop", "container", "stackable", "forceUse", "multiUse", "hasCharges",
      "writable", "writableOnce", "fluidContainer", "fluid", "unpassable", "unmoveable", "blockMissile", "blockPathfind",
      "noMoveAnimation", "pickupable", "hangable", "hookSouth", "hookEast", "rotatable", "hasLight", "dontHide",
      "translucent", "floorChange", "hasOffset", "hasElevation", "lyingObject", "animateAlways", "miniMap", "lensHelp",
      "fullGround", "ignoreLook", "cloth", "isMarket", "hasDefaultAction", "wrappable", "unwrappable", "topEffect",
      "usable", "hasBones",
    ];
    const keys = new Set(FLAG_GROUPS.flatMap((g) => g.flags.map((f) => f.key as string)));
    expect(backend.filter((k) => !keys.has(k))).toEqual([]);
  });
});

describe("paths", () => {
  it("reads and writes nested values", () => {
    const o = { a: 1, market: { name: "x" } };
    expect(getPath(o, "market.name")).toBe("x");
    setPath(o, "market.name", "y");
    setPath(o, "a", 5);
    expect(o).toEqual({ a: 5, market: { name: "y" } });
  });

  it("clamps numeric fields", () => {
    expect(clampField("u16", -5)).toBe(0);
    expect(clampField("u16", 70000)).toBe(65535);
    expect(clampField("i16", -40000)).toBe(-32768);
    expect(clampField("i16", 3.7)).toBe(3);
    expect(clampField("u16", NaN)).toBe(0);
  });
});

describe("buildPatch", () => {
  it("writes chosen flags and fields of flags switched on", () => {
    const patch = buildPatch(
      { pickupable: true, stackable: false, ground: undefined, hasLight: true, isMarket: true, writable: false },
      { lightLevel: 7, "market.name": "Sword", maxTextLength: 5 },
    );
    expect(patch).toEqual({
      pickupable: true,
      stackable: false,
      hasLight: true,
      lightLevel: 7,
      lightColor: 0,
      isMarket: true,
      market: { name: "Sword", category: 0, tradeAs: 0, showAs: 0, restrictProfession: 0, restrictLevel: 0 },
      writable: false,
    });
  });
});
