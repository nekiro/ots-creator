import { describe, expect, it } from "vitest";
import type { MapInfo } from "./api";
import { ZOOMS, panTiles, startPosition, stepZoom, zoomAround } from "./camera";

describe("stepZoom", () => {
  it("steps through the zoom levels", () => {
    expect(stepZoom(1, 1)).toBe(1.5);
    expect(stepZoom(1, -1)).toBe(0.75);
  });

  it("stops at the first and last level", () => {
    expect(stepZoom(ZOOMS[0], -1)).toBe(ZOOMS[0]);
    expect(stepZoom(ZOOMS.at(-1)!, 1)).toBe(ZOOMS.at(-1));
  });

  it("counts a zoom between levels as the next level up", () => {
    expect(stepZoom(1.2, 0)).toBe(1.5);
    expect(stepZoom(100, -1)).toBe(ZOOMS.at(-2));
  });
});

describe("zoomAround", () => {
  it("keeps the center when zooming at the center", () => {
    expect(zoomAround(100, 200, 1, 2, 0, 0)).toEqual({ x: 100, y: 200 });
  });

  it("keeps the point under the cursor in place", () => {
    const offsetX = 64;
    const next = zoomAround(100, 100, 1, 2, offsetX, 0);
    // World x under the cursor before and after.
    expect(100 + offsetX / 1).toBe(next.x + offsetX / 2);
  });
});

describe("panTiles", () => {
  it("moves more tiles when zoomed out", () => {
    expect(panTiles(3, 1)).toBe(3);
    expect(panTiles(3, 0.5)).toBe(6);
    expect(panTiles(-3, 0.25)).toBe(-12);
  });

  it("moves at least one tile", () => {
    expect(panTiles(1, 8)).toBe(1);
    expect(panTiles(0, 1)).toBe(0);
  });
});

describe("startPosition", () => {
  const props = { width: 500, height: 4000 } as MapInfo["props"];

  it("prefers the first temple", () => {
    const info = { hasBounds: true, bounds: [0, 0, 10, 10], towns: [{ temple: { x: 50, y: 60, z: 7 } }], props } as unknown as MapInfo;
    expect(startPosition(info)).toEqual({ x: 50, y: 60, z: 7 });
  });

  it("takes the middle of the tiles", () => {
    const info = { hasBounds: true, bounds: [100, 200, 301, 400], towns: [], props } as unknown as MapInfo;
    expect(startPosition(info)).toEqual({ x: 201, y: 300, z: 7 });
  });

  it("takes near the middle of an empty map", () => {
    expect(startPosition({ hasBounds: false, bounds: [], towns: [], props } as unknown as MapInfo)).toEqual({ x: 250, y: 1000, z: 7 });
    expect(startPosition(undefined)).toEqual({ x: 1000, y: 1000, z: 7 });
  });
});
