import { describe, expect, it } from "vitest";
import { Category, type DiffEntry, type ObjectFile } from "../api";
import { mergeIds, mergeSummary } from "../compare";
import { formatDate, formatKb } from "./format";
import { LruCache } from "./lruCache";
import { licenseLabel } from "../market";
import { decodeObjectFile } from "../objectFile";
import { anchoredRange, idRange, sliceBetween, toggleId } from "./selection";
import { keyStep, nextIndex } from "../ui/listNav";

describe("selection", () => {
  it("toggles ids", () => {
    expect(toggleId([1, 2], 3)).toEqual([1, 2, 3]);
    expect(toggleId([1, 2, 3], 2)).toEqual([1, 3]);
  });

  it("slices between indexes in either order", () => {
    expect(sliceBetween(["a", "b", "c", "d"], 3, 1)).toEqual(["b", "c", "d"]);
    expect(sliceBetween(["a", "b"], 0, 0)).toEqual(["a"]);
  });

  it("lists whole numbers between two ids", () => {
    expect(idRange(5, 3)).toEqual([3, 4, 5]);
    expect(idRange(7, 7)).toEqual([7]);
  });

  it("keeps the anchor first", () => {
    expect(anchoredRange(4, idRange(2, 5))).toEqual([4, 2, 3, 5]);
  });
});

describe("listNav", () => {
  it("moves left and right in any list", () => {
    expect(keyStep("ArrowLeft")).toBe(-1);
    expect(keyStep("ArrowRight")).toBe(1);
    expect(keyStep("ArrowDown")).toBeNull();
    expect(keyStep("Enter", { columns: 4, pageRows: 2 })).toBeNull();
  });

  it("moves rows and pages in grids", () => {
    expect(keyStep("ArrowUp", { columns: 4 })).toBe(-4);
    expect(keyStep("ArrowDown", { columns: 4 })).toBe(4);
    expect(keyStep("PageDown", { columns: 4 })).toBeNull();
    expect(keyStep("PageUp", { columns: 4, pageRows: 3 })).toBe(-12);
  });

  it("keeps the index in the list", () => {
    expect(nextIndex("ArrowDown", 8, 10, { columns: 4 })).toBe(9);
    expect(nextIndex("ArrowLeft", 0, 10)).toBe(0);
    expect(nextIndex("ArrowRight", -1, 10)).toBe(0);
    expect(nextIndex("Tab", 3, 10)).toBeNull();
  });
});

describe("LruCache", () => {
  it("drops the least recently used entries past its capacity", () => {
    const cache = new LruCache<string, number>(2);
    cache.set("a", 1);
    cache.set("b", 2);
    expect(cache.get("a")).toBe(1);
    cache.set("c", 3);
    expect(cache.peek("b")).toBeUndefined();
    expect(cache.peek("a")).toBe(1);
    expect(cache.size).toBe(2);
  });

  it("does not refresh entries on peek", () => {
    const cache = new LruCache<string, number>(2);
    cache.set("a", 1);
    cache.set("b", 2);
    cache.peek("a");
    cache.set("c", 3);
    expect(cache.peek("a")).toBeUndefined();
  });

  it("forgets deleted entries", () => {
    const cache = new LruCache<string, number>(2);
    cache.set("a", 1);
    cache.delete("a");
    expect(cache.get("a")).toBeUndefined();
  });
});

describe("format", () => {
  it("writes sizes in bytes or KB", () => {
    expect(formatKb(512)).toBe("512 B");
    expect(formatKb(1536)).toBe("1.5 KB");
  });

  it("writes dates without the time", () => {
    expect(formatDate("2024-05-06T10:00:00Z")).toBe(new Date("2024-05-06T10:00:00Z").toLocaleDateString());
  });

  it("names licenses", () => {
    expect(licenseLabel("CC0-1.0")).toBe("CC0 (public domain)");
    expect(licenseLabel("custom")).toBe("custom");
  });
});

describe("decodeObjectFile", () => {
  it("rejects files without an object", () => {
    expect(() => decodeObjectFile(null, "empty")).toThrow("empty");
    expect(() => decodeObjectFile({ thing: null } as unknown as ObjectFile, "nothing")).toThrow("nothing");
  });

  it("decodes the object and its sprites", () => {
    const file = {
      spriteSize: 32,
      thing: { id: 100, category: Category.CategoryItem, frameGroups: [{ sprites: [1], durations: null }], npcSales: null },
      sprites: [btoa(String.fromCharCode(1, 2, 3, 4))],
    } as unknown as ObjectFile;
    const decoded = decodeObjectFile(file, "empty");
    expect(decoded.file).toBe(file);
    expect(decoded.thing.id).toBe(100);
    expect(decoded.thing.npcSales).toEqual([]);
    expect([...decoded.pixels[0]]).toEqual([1, 2, 3, 4]);
  });
});

describe("compare", () => {
  const entries = [
    { id: 1, status: "changed" },
    { id: 2, status: "onlyA" },
    { id: 3, status: "onlyB" },
    { id: 4, status: "onlyB", empty: true },
  ] as DiffEntry[];

  it("merges what A lacks or has differently", () => {
    expect(mergeIds(entries, false)).toEqual([1, 3, 4]);
    expect(mergeIds(entries, true)).toEqual([1, 3]);
    expect(mergeIds(null, true)).toEqual([]);
  });

  it("summarizes a merge plan", () => {
    expect(
      mergeSummary([
        { category: Category.CategoryItem, ids: [1, 2] },
        { category: Category.CategoryOutfit, ids: [3] },
      ]),
    ).toBe("2 items, 1 outfits");
  });
});
