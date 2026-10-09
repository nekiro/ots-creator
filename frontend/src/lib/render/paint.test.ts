import { describe, expect, it } from "vitest";
import { changedSlots, drawLine, getPixel, hexToRGBA, rgbaToHex } from "./paint";

// jsdom-free ImageData stand-in for node.
class FakeImageData {
  data: Uint8ClampedArray;
  constructor(
    public width: number,
    public height: number,
  ) {
    this.data = new Uint8ClampedArray(width * height * 4);
  }
}
const image = (w: number, h: number) => new FakeImageData(w, h) as unknown as ImageData;

describe("paint", () => {
  it("draws connected lines", () => {
    const img = image(8, 8);
    drawLine(img, 0, 0, 7, 3, [255, 0, 0, 255]);
    let n = 0;
    for (let i = 3; i < img.data.length; i += 4) if (img.data[i]) n++;
    expect(n).toBe(8); // one pixel per column
    expect(getPixel(img, 7, 3)).toEqual([255, 0, 0, 255]);
    drawLine(img, -5, 0, 2, 0, [0, 255, 0, 255]); // clipped, no throw
    expect(getPixel(img, 0, 0)).toEqual([0, 255, 0, 255]);
  });

  it("finds changed slots of a 2x1 texture", () => {
    const g = { width: 2, height: 1, layers: 1, patternX: 1, patternY: 1, patternZ: 1, frames: 1 };
    const before = image(4, 2);
    const after = image(4, 2);
    after.data.set([9, 9, 9, 255], (1 * 4 + 0) * 4); // pixel (0, 1): left tile
    const { slots, pixels } = changedSlots(before, after, g, { layer: 0, x: 0, y: 0, z: 0, frame: 0 }, 2);
    // Tile (0,0) is the bottom-right one, so the left tile is slot 1.
    expect(slots).toEqual([1]);
    expect(Array.from(pixels[0].subarray(8, 12))).toEqual([9, 9, 9, 255]);
  });

  it("converts colors", () => {
    expect(hexToRGBA("#ff8000")).toEqual([255, 128, 0, 255]);
    expect(rgbaToHex([1, 2, 3, 255])).toBe("#010203");
  });
});
