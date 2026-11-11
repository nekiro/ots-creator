import { describe, expect, it } from "vitest";
import type { TileInfo, TileItem } from "../data/api";
import { tilePicks } from "./menus";

const item = (id: number, name: string, brush = "", brushKind = "", topOrder = 0) => ({ id, name, brush, brushKind, topOrder }) as TileItem;
const tile = (ground: TileItem | null, ...items: TileItem[]) => ({ ground, items }) as TileInfo;

describe("tilePicks", () => {
  it("picks the brush of an item instead of the raw item", () => {
    const picks = tilePicks(tile(item(103, "earth tile", "earth", "ground")));
    expect(picks.click).toMatchObject({ kind: "ground", name: "earth" });
    expect(picks.brushes.map((brush) => brush.name)).toEqual(["earth"]);
    expect(picks.raw).toMatchObject({ kind: "raw", item: 103 });
  });

  it("looks under brushless borders for the ground brush", () => {
    const picks = tilePicks(tile(item(103, "earth tile", "earth", "ground"), item(4500, "grass border", "", "", 1)));
    expect(picks.click).toMatchObject({ kind: "ground", name: "earth" });
    expect(picks.raw).toMatchObject({ kind: "raw", item: 4500 });
  });

  it("picks a brushless loose item as is", () => {
    const picks = tilePicks(tile(item(103, "earth tile", "earth", "ground"), item(3031, "gold coin")));
    expect(picks.click).toMatchObject({ kind: "raw", item: 3031 });
    expect(picks.brushes.map((brush) => brush.name)).toEqual(["earth"]);
  });

  it("lists each brush once, topmost first, and paints decoration as a wall", () => {
    const picks = tilePicks(tile(item(103, "earth tile", "earth", "ground"), item(1, "fence", "fence", "decoration", 2), item(2, "fence", "fence", "decoration", 2)));
    expect(picks.brushes.map((brush) => [brush.kind, brush.name])).toEqual([
      ["wall", "fence"],
      ["ground", "earth"],
    ]);
  });

  it("picks nothing on an empty tile", () => {
    expect(tilePicks(null)).toEqual({ brushes: [], raw: null, click: null });
  });
});
