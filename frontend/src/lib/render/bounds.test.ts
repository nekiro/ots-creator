import { describe, expect, it } from "vitest";
import { contentBox, union } from "./bounds";

describe("bounds", () => {
  it("finds visible pixels", () => {
    const data = new Uint8ClampedArray(4 * 4 * 4);
    data[(1 * 4 + 2) * 4 + 3] = 255;
    data[(3 * 4 + 3) * 4 + 3] = 1;
    expect(contentBox({ width: 4, height: 4, data })).toEqual({ x: 2, y: 1, w: 2, h: 3 });
    expect(contentBox({ width: 4, height: 4, data: new Uint8ClampedArray(64) })).toBeNull();
  });

  it("joins boxes", () => {
    expect(union({ x: 0, y: 5, w: 2, h: 2 }, { x: 4, y: 1, w: 1, h: 1 })).toEqual({ x: 0, y: 1, w: 5, h: 6 });
    expect(union(null, { x: 1, y: 1, w: 1, h: 1 })).toEqual({ x: 1, y: 1, w: 1, h: 1 });
  });
});
