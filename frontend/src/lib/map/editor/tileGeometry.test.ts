import { describe, expect, it } from "vitest";
import { bresenhamLine, brushArea, screenToTile, squareOutline } from "./tileGeometry";

const at = (x: number, y: number, z = 7) => ({ x, y, z });

describe("brushArea", () => {
  it("is the center alone at size 0", () => {
    expect(brushArea(at(5, 5), 0, "square")).toEqual([at(5, 5)]);
  });

  it("covers a (2n+1) square", () => {
    expect(brushArea(at(5, 5), 1, "square")).toHaveLength(9);
    expect(brushArea(at(5, 5), 2, "square")).toHaveLength(25);
  });

  it("leaves the corners out of a circle", () => {
    const tiles = brushArea(at(5, 5), 1, "circle");
    expect(tiles).toHaveLength(5);
    expect(tiles).not.toContainEqual(at(4, 4));
  });

  it("skips tiles below 0", () => {
    expect(brushArea(at(0, 0), 1, "square")).toEqual([at(0, 0), at(1, 0), at(0, 1), at(1, 1)]);
  });
});

describe("squareOutline", () => {
  it("is the border of the square", () => {
    const tiles = squareOutline(at(5, 5), 1);
    expect(tiles).toHaveLength(8);
    expect(tiles).not.toContainEqual(at(5, 5));
  });
});

describe("bresenhamLine", () => {
  it("leaves out the start tile", () => {
    expect(bresenhamLine(at(0, 0), at(3, 0))).toEqual([at(1, 0), at(2, 0), at(3, 0)]);
  });

  it("is empty between the same tile", () => {
    expect(bresenhamLine(at(2, 2), at(2, 2))).toEqual([]);
  });

  it("steps diagonally without gaps", () => {
    expect(bresenhamLine(at(3, 3), at(0, 0))).toEqual([at(2, 2), at(1, 1), at(0, 0)]);
  });

  it("keeps every step to a neighbor", () => {
    const line = [at(0, 0), ...bresenhamLine(at(0, 0), at(7, 3))];
    for (let i = 1; i < line.length; i++) {
      expect(Math.max(Math.abs(line[i].x - line[i - 1].x), Math.abs(line[i].y - line[i - 1].y))).toBe(1);
    }
    expect(line.at(-1)).toEqual(at(7, 3));
  });
});

describe("screenToTile", () => {
  const view = { width: 800, height: 600, camX: 100 * 32, camY: 100 * 32, scale: 1, floor: 7, tileSize: 32 };

  it("finds the tile at the center of the view", () => {
    expect(screenToTile(400, 300, view)).toEqual(at(100, 100));
  });

  it("scales by the zoom", () => {
    expect(screenToTile(400 + 64, 300, { ...view, scale: 2 })).toEqual(at(101, 100));
  });

  it("offsets underground floors", () => {
    expect(screenToTile(400, 300, { ...view, floor: 8 })).toEqual(at(100, 100, 8));
    expect(screenToTile(400, 300, { ...view, floor: 5 })).toEqual(at(102, 102, 5));
  });

  it("clamps at 0", () => {
    expect(screenToTile(0, 0, { ...view, camX: 0, camY: 0 })).toEqual(at(0, 0));
  });
});
