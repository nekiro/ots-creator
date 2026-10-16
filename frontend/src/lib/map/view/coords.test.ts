import { describe, expect, it } from "vitest";
import { floorOffset, floorShift, tileToWorld, worldToTile } from "./coords";

describe("floor shift", () => {
  it("shifts floors above ground by their distance to the ground floor", () => {
    expect(floorShift(7, 7)).toBe(0);
    expect(floorShift(6, 7)).toBe(1);
    expect(floorShift(5, 3)).toBe(2);
  });

  it("shifts underground floors by their distance to the floor in view", () => {
    expect(floorShift(10, 8)).toBe(-2);
    expect(floorShift(8, 8)).toBe(0);
  });

  it("has no offset for the ground floor or underground", () => {
    expect(floorOffset(7)).toBe(0);
    expect(floorOffset(0)).toBe(7);
    expect(floorOffset(12)).toBe(0);
  });
});

describe("tile and world positions", () => {
  it("finds the tile under a world position", () => {
    expect(worldToTile(32 * 100 + 5, 32 * 50, 7, 32)).toEqual({ x: 100, y: 50 });
    expect(worldToTile(32 * 100, 32 * 50 - 1, 5, 32)).toEqual({ x: 102, y: 51 });
  });

  it("puts a tile center where worldToTile finds it again", () => {
    for (const z of [0, 5, 7, 9]) {
      const world = tileToWorld({ x: 300, y: 400, z }, 32);
      expect(worldToTile(world.x, world.y, z, 32)).toEqual({ x: 300, y: 400 });
    }
  });

  it("centers the camera on a tile", () => {
    expect(tileToWorld({ x: 10, y: 20, z: 7 }, 32)).toEqual({ x: 336, y: 656 });
  });
});
