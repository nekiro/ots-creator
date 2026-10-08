import { describe, expect, it } from "vitest";
import { blendOver, compose, stackBottomRight } from "./compose";
import type { Layout } from "./layout";

// jsdom-free ImageData stand-in for node.
class FakeImageData {
  data: Uint8ClampedArray;
  constructor(public width: number, public height: number) {
    this.data = new Uint8ClampedArray(width * height * 4);
  }
}
(globalThis as any).ImageData ??= FakeImageData;

const S = 2; // tiny sprites keep the test readable
const solid = (r: number, g: number, b: number, a = 255) => new Uint8ClampedArray(Array.from({ length: S * S }, () => [r, g, b, a]).flat());

describe("blendOver", () => {
  it("copies opaque, skips transparent, mixes partial", () => {
    const dst = new Uint8ClampedArray([10, 10, 10, 255]);
    blendOver(dst, 0, new Uint8ClampedArray([0, 0, 0, 0]), 0);
    expect(Array.from(dst)).toEqual([10, 10, 10, 255]);
    blendOver(dst, 0, new Uint8ClampedArray([200, 0, 0, 128]), 0);
    expect(dst[0]).toBeGreaterThan(90);
    expect(dst[0]).toBeLessThan(110);
    expect(dst[3]).toBe(255);
  });
});

describe("compose", () => {
  const g: Layout = { width: 2, height: 1, layers: 2, patternX: 1, patternY: 2, patternZ: 1, frames: 1 };
  // slots: (w,h,layer,py) -> index; w fastest, then h, layer, x, y
  const sprites = [1, 2, 3, 4, 5, 0, 0, 0];
  const pix: Record<number, Uint8ClampedArray> = {
    1: solid(100, 100, 100), // w0 layer0 py0 (right tile)
    2: solid(50, 50, 50), // w1 layer0 py0 (left tile)
    3: solid(255, 0, 0), // template right tile: body
    4: solid(0, 0, 0, 0),
    5: solid(1, 2, 3), // addon py1 right tile
  };
  const get = (id: number) => pix[id];

  it("places tile 0 on the right", () => {
    const img = compose(g, sprites, S, { pos: { layer: 0, x: 0, y: 0, z: 0, frame: 0 } }, get);
    expect(img.width).toBe(4);
    expect(img.data[0]).toBe(50); // left pixel from sprite 2
    expect(img.data[2 * 4]).toBe(100); // right half from sprite 1
  });

  it("draws addons on top", () => {
    const img = compose(g, sprites, S, { pos: { layer: 0, x: 0, y: 0, z: 0, frame: 0 }, addons: [1] }, get);
    expect(Array.from(img.data.slice(8, 12))).toEqual([1, 2, 3, 255]);
  });

  it("colorizes only template areas", () => {
    const img = compose(g, sprites, S, { pos: { layer: 0, x: 0, y: 0, z: 0, frame: 0 }, colors: { head: 0, body: 132, legs: 0, feet: 0 } }, get);
    expect(img.data[8]).toBeLessThan(100); // right tile tinted
    expect(img.data[0]).toBe(50); // left tile untouched
  });
});

describe("stackBottomRight", () => {
  const img = (w: number, h: number, r: number) => {
    const i = new ImageData(w, h);
    for (let k = 0; k < w * h; k++) i.data.set([r, 0, 0, 255], k * 4);
    return i;
  };

  it("anchors smaller images at the bottom-right, later ones on top", () => {
    const { image, offsets } = stackBottomRight([img(4, 4, 10), img(2, 2, 200)]);
    expect([image.width, image.height]).toEqual([4, 4]);
    expect(offsets).toEqual([[0, 0], [2, 2]]);
    expect(image.data[0]).toBe(10); // top-left: mount only
    expect(image.data[(3 * 4 + 3) * 4]).toBe(200); // bottom-right: rider
  });
});
