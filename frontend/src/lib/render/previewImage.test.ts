import { describe, expect, it } from "vitest";
import type { FrameGroup } from "../api";
import { mountedPattern, renderPreviewImage, type PreviewImageInput } from "./previewImage";

// ImageData stand-in for node.
class FakeImageData {
  data: Uint8ClampedArray;
  constructor(
    public width: number,
    public height: number,
  ) {
    this.data = new Uint8ClampedArray(width * height * 4);
  }
}
(globalThis as { ImageData?: unknown }).ImageData ??= FakeImageData;

const SPRITE_SIZE = 1;

function group(fields: Partial<FrameGroup>): FrameGroup {
  return { width: 1, height: 1, layers: 1, patternX: 1, patternY: 1, patternZ: 1, frames: 1, sprites: [1], durations: [], ...fields } as FrameGroup;
}

/** Sprite n is one opaque pixel with red n. */
const get = (id: number) => (id ? new Uint8ClampedArray([id, 0, 0, 255]) : undefined);

function input(change: Partial<PreviewImageInput>): PreviewImageInput {
  return {
    frameGroup: group({}),
    mountGroup: null,
    size: SPRITE_SIZE,
    pos: { layer: 0, x: 0, y: 0, z: 0, frame: 0 },
    isOutfit: false,
    mount: false,
    addon1: false,
    addon2: false,
    colors: null,
    get,
    ...change,
  };
}

describe("mountedPattern", () => {
  it("needs an outfit with a second pattern Z and the mount chosen", () => {
    expect(mountedPattern(group({ patternZ: 2 }), true, true)).toBe(true);
    expect(mountedPattern(group({ patternZ: 1 }), true, true)).toBe(false);
    expect(mountedPattern(group({ patternZ: 2 }), false, true)).toBe(false);
    expect(mountedPattern(group({ patternZ: 2 }), true, false)).toBe(false);
  });
});

describe("renderPreviewImage", () => {
  it("draws the texture at the position", () => {
    const { image, riderOffset } = renderPreviewImage(input({ frameGroup: group({ patternX: 2, sprites: [5, 6] }), pos: { layer: 0, x: 1, y: 0, z: 0, frame: 0 } }));
    expect(image.data[0]).toBe(6);
    expect(riderOffset).toEqual([0, 0]);
  });

  it("draws the mounted pattern on the mount, bottom-right aligned", () => {
    const rider = group({ patternZ: 2, sprites: [5, 7] });
    const mount = group({ width: 2, height: 2, sprites: [9, 9, 9, 9] });
    const { image, riderOffset } = renderPreviewImage(input({ frameGroup: rider, mountGroup: mount, isOutfit: true, mount: true }));
    expect(image.width).toBe(2);
    expect(riderOffset).toEqual([1, 1]);
    // The rider's mounted sprite sits over the mount's bottom-right pixel.
    expect(image.data[(1 * 2 + 1) * 4]).toBe(7);
  });

  it("draws the rider alone when the mount is not loaded", () => {
    const rider = group({ patternZ: 2, sprites: [5, 7] });
    const { image } = renderPreviewImage(input({ frameGroup: rider, isOutfit: true, mount: true }));
    expect(image.width).toBe(1);
    expect(image.data[0]).toBe(7);
  });
});
