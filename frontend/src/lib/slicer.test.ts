import { describe, expect, it } from "vitest";
import { cutCell, fitGrid, isEmpty, type Pixels } from "./slicer";

function image(w: number, h: number, fill: (x: number, y: number) => number[]): Pixels {
  const data = new Uint8ClampedArray(w * h * 4);
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) data.set(fill(x, y), (y * w + x) * 4);
  return { width: w, height: h, data };
}

describe("slicer", () => {
  it("fits whole cells after the offset", () => {
    expect(fitGrid(70, 33, 32)).toEqual({ offsetX: 0, offsetY: 0, columns: 2, rows: 1 });
    expect(fitGrid(70, 33, 32, 8, 2)).toEqual({ offsetX: 8, offsetY: 2, columns: 1, rows: 0 });
  });

  it("cuts cells with offsets and clears magenta", () => {
    // 4x4 image, 2px cells: left half red, right half magenta.
    const img = image(4, 4, (x) => (x < 2 ? [255, 0, 0, 255] : [255, 0, 255, 255]));
    const grid = fitGrid(4, 4, 2);
    expect(Array.from(cutCell(img, grid, 0, 1, 2, true).slice(0, 4))).toEqual([255, 0, 0, 255]);
    expect(isEmpty(cutCell(img, grid, 1, 0, 2, true))).toBe(true);
    expect(isEmpty(cutCell(img, grid, 1, 0, 2, false))).toBe(false);
    // Offset by one pixel: the cell straddles red and magenta, the last column is outside.
    const shifted = cutCell(img, { offsetX: 1, offsetY: 0, columns: 2, rows: 2 }, 1, 0, 2, true);
    expect(Array.from(shifted.slice(0, 8))).toEqual([0, 0, 0, 0, 0, 0, 0, 0]);
  });
});
