import { describe, expect, it } from "vitest";
import type { Scene, ViewOptions } from "../render/scene";
import { Camera, floorsToDraw } from "./view";

/** A scene with only what the camera reads: 32 px sprites, an 800x600 view. */
function scene(change: Partial<Scene> = {}, opts: Partial<ViewOptions> = {}): Scene {
  return {
    width: 800,
    height: 600,
    spriteSize: 32,
    camX: 1000 * 32,
    camY: 1000 * 32,
    scale: 1,
    floor: 7,
    opts: { lowerFloors: false, ...opts } as ViewOptions,
    ...change,
  } as Scene;
}

describe("Camera", () => {
  it("has the tile size in device pixels", () => {
    expect(new Camera(scene({ scale: 2 })).tilePx).toBe(64);
  });

  it("puts the camera position at the view center", () => {
    const camera = new Camera(scene());
    expect(camera.tileX(1000, 7)).toBe(400);
    expect(camera.tileY(1000, 7)).toBe(300);
  });

  it("scales tile positions around the view center", () => {
    const camera = new Camera(scene({ scale: 2 }));
    expect(camera.tileX(1001, 7) - camera.tileX(1000, 7)).toBe(64);
  });

  it("rounds positions to whole pixels", () => {
    const camera = new Camera(scene({ scale: 0.3 }));
    expect(Number.isInteger(camera.tileX(1003, 7))).toBe(true);
  });

  it.each([
    ["the ground floor in view", 7, 7, 0],
    ["a floor above the ground", 7, 5, 2],
    ["the floor in view above the ground", 5, 5, 2],
    ["the floor in view underground", 9, 9, 0],
    ["a floor under the underground floor in view", 9, 10, -1],
  ])("shifts %s like the client stacks floors", (_, floor, z, shift) => {
    expect(new Camera(scene({ floor })).shift(z)).toBe(shift);
  });

  it("draws higher floors up and left by their shift", () => {
    const camera = new Camera(scene());
    expect(camera.tileX(1000, 6)).toBe(camera.tileX(1000, 7) - 32);
  });

  it("lists the tiles in view with the margins asked for", () => {
    // 800x600 px at 32 px per tile: 12.5 tiles each side of x 1000, 9.375 of y 1000.
    expect(new Camera(scene()).tiles(7, 0, 0)).toEqual({ x0: 987, y0: 990, x1: 1013, y1: 1010 });
    expect(new Camera(scene()).tiles(7, 1, 3)).toEqual({ x0: 986, y0: 989, x1: 1016, y1: 1013 });
  });

  it("has the view center and half size in tiles", () => {
    const camera = new Camera(scene({ floor: 5 }));
    expect(camera.centerX(5)).toBe(1002);
    expect(camera.centerY(5)).toBe(1002);
    expect(camera.halfWidthTiles).toBe(12.5);
    expect(camera.halfHeightTiles).toBe(9.375);
  });

  it("counts the visible tiles with a three tile margin", () => {
    expect(new Camera(scene()).visibleTiles).toBe(Math.floor((800 / 32 + 6) * (600 / 32 + 6)));
  });
});

describe("floorsToDraw", () => {
  it("draws only the floor in view without lower floors", () => {
    expect(floorsToDraw(scene({ floor: 5 }), 100)).toEqual([5]);
  });

  it("draws down to the ground floor above it, bottom up", () => {
    expect(floorsToDraw(scene({ floor: 5 }, { lowerFloors: true }), 100)).toEqual([7, 6, 5]);
  });

  it("draws two floors down underground", () => {
    expect(floorsToDraw(scene({ floor: 9 }, { lowerFloors: true }), 100)).toEqual([11, 10, 9]);
  });

  it("stops at the deepest floor", () => {
    expect(floorsToDraw(scene({ floor: 15 }, { lowerFloors: true }), 100)).toEqual([15]);
  });

  it("leaves out the lowest floors when there are too many tiles", () => {
    // 1.5M tiles a floor: two floors fit in the 3M tile budget.
    expect(floorsToDraw(scene({ floor: 3 }, { lowerFloors: true }), 1_500_000)).toEqual([4, 3]);
  });

  it("always keeps the floor in view", () => {
    expect(floorsToDraw(scene({ floor: 3 }, { lowerFloors: true }), 10_000_000)).toEqual([3]);
  });
});
