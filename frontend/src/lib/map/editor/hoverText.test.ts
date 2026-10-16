import { describe, expect, it } from "vitest";
import type { HouseInfo } from "../data/api";
import { TileFlags, type TileData } from "../data/chunks";
import { describeTile } from "./hoverText";

const position = { x: 100, y: 200, z: 7 };
const itemName = (id: number) => `item${id}`;

function tile(change: Partial<TileData>): TileData {
  return { flags: 0, house: 0, ids: new Uint16Array(0), subs: new Uint16Array(0), iflags: new Uint8Array(0), creatures: null, spawn: null, spawnNpc: null, zones: null, text: null, ...change };
}

describe("describeTile", () => {
  it("is the position for an empty tile", () => {
    expect(describeTile(position, undefined, itemName, null)).toBe("100, 200, 7");
  });

  it("names the top item, creatures, house and PZ", () => {
    const houses = [{ id: 3, name: "Flat 1" }] as HouseInfo[];
    const data = tile({
      ids: Uint16Array.of(10, 20),
      creatures: [{ name: "Rat" }, { name: "Cat" }] as TileData["creatures"],
      house: 3,
      flags: TileFlags.pz,
    });
    expect(describeTile(position, data, itemName, houses)).toBe("100, 200, 7 · item20 (20) · Rat, Cat · house Flat 1 · PZ");
  });

  it("shows the house id when the house is unknown", () => {
    expect(describeTile(position, tile({ house: 9 }), itemName, [])).toBe("100, 200, 7 · house 9");
  });
});
