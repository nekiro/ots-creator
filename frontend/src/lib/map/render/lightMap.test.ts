import { describe, expect, it } from "vitest";
import { chunkKey } from "../data/chunks";
import { LIGHT_BYTES, type GeoChunk } from "./geo";
import { ambientIntensity, buildLightMap, lightColor } from "./lightMap";

const SPRITE = 32;

interface Shades {
  shaded?: number[][];
  translucent?: number[][];
  grounds?: number[][];
}

const ROW = Array.from({ length: 32 }, (_, x) => [x, 0]);

/** A chunk of floor z with lights ([x, y, level, color] in world pixels) and shade bits on tiles. */
function chunk(chunkX: number, chunkY: number, z: number, lights: number[][], { shaded = [], translucent = [], grounds = [] }: Shades = {}): GeoChunk {
  const view = new DataView(new ArrayBuffer(lights.length * LIGHT_BYTES));
  lights.forEach(([x, y, level, color], index) => {
    view.setInt16(index * LIGHT_BYTES, x, true);
    view.setInt16(index * LIGHT_BYTES + 2, y, true);
    view.setUint8(index * LIGHT_BYTES + 4, level);
    view.setUint8(index * LIGHT_BYTES + 5, color);
  });
  let shades: Uint8Array | null = null;
  if (shaded.length || grounds.length) {
    shades = new Uint8Array(3 * 128);
    [shaded, translucent, grounds].forEach((tiles, plane) => {
      for (const [x, y] of tiles) shades![plane * 128 + ((y * 32 + x) >> 3)] |= 1 << ((y * 32 + x) & 7);
    });
  }
  return { key: chunkKey(chunkX, chunkY, z), quads: new Uint8Array(0), quadCount: 0, animated: new Uint32Array(0), composed: new Uint32Array(0), sprites: new Uint32Array(0), lights: view, lightCount: lights.length, shades, signature: "" };
}

function reds(pixels: Uint8Array, width: number): number[][] {
  const rows: number[][] = [];
  for (let offset = 0; offset < pixels.length; offset += 4) {
    const index = offset / 4;
    (rows[Math.floor(index / width)] ??= []).push(pixels[offset]);
  }
  return rows;
}

describe("lightColor", () => {
  it("reads the 6x6x6 color cube", () => {
    expect(lightColor(215)).toEqual([255, 255, 255]);
    expect(lightColor(180)).toEqual([255, 0, 0]);
    expect(lightColor(206)).toEqual([255, 204, 102]);
    expect(lightColor(0)).toEqual([0, 0, 0]);
    expect(lightColor(216)).toEqual([0, 0, 0]);
    expect(lightColor(215, 0.5)).toEqual([127, 127, 127]);
  });
});

describe("ambientIntensity", () => {
  it("maps percent to 0..255 as the client does", () => {
    expect(ambientIntensity(75)).toBe(191);
    expect(ambientIntensity(100)).toBe(255);
    expect(ambientIntensity(-5)).toBe(0);
  });
});

describe("buildLightMap", () => {
  const grid = { x: 0, y: 0, width: 9, height: 1 };
  /** A level 3 light at the center of tile 4 of floor 7, on grounds. */
  const lit = (shades: Shades = { grounds: ROW }) => ({ z: 7, chunks: [chunk(0, 0, 7, [[4 * SPRITE + 16, 16, 3, 215]], shades)], shift: 0 });

  it("fades a light out over its last five tiles", () => {
    const pixels = new Uint8Array(9 * 4);
    // 0.6 at its tile, 0.4 a tile away, 0.2 two tiles away.
    buildLightMap(pixels, grid, [lit()], 0, SPRITE, 7);
    expect(reds(pixels, 9)[0]).toEqual([0, 0, 51, 102, 153, 102, 51, 0, 0]);
  });

  it("keeps the ambient light where it is brighter", () => {
    const pixels = new Uint8Array(9 * 4);
    buildLightMap(pixels, grid, [lit()], 127, SPRITE, 7);
    expect(reds(pixels, 9)[0]).toEqual([127, 127, 127, 127, 153, 127, 127, 127, 127]);
    expect(pixels[3]).toBe(255);
  });

  it("dims a light to 4/5 where its floor has no ground", () => {
    const pixels = new Uint8Array(9 * 4);
    buildLightMap(pixels, grid, [lit({ grounds: ROW.filter(([x]) => x !== 3) })], 0, SPRITE, 7);
    expect(reds(pixels, 9)[0]).toEqual([0, 0, 51, 81, 153, 102, 51, 0, 0]);
  });

  it("hides the light of lower floors under shading tiles, dims it under translucent ones", () => {
    const pixels = new Uint8Array(9 * 4);
    // Floor 6 is drawn a tile up and left: its tiles (6, 1) and (4, 1) cover tiles 5 and 3 on screen; its own light still shines on 5.
    const above = { z: 6, chunks: [chunk(0, 0, 6, [[6 * SPRITE + 16, SPRITE + 16, 1, 215]], { shaded: [[6, 1], [4, 1]], translucent: [[4, 1]], grounds: [[6, 1], [4, 1]] })], shift: 1 };
    buildLightMap(pixels, grid, [lit(), above], 0, SPRITE, 6);
    // The light of floor 7 is dimmed once as a lower floor's (153 -> 122), twice under the translucent tile 3 (102 -> 65).
    expect(reds(pixels, 9)[0]).toEqual([0, 0, 40, 65, 122, 51, 40, 0, 0]);
  });

  it("places lights of chunks by their position and the floor's shift", () => {
    const pixels = new Uint8Array(4 * 4);
    // Tile (33, 1) of floor 6, drawn a tile up and left: grid tile (32, 0), the third of a grid from x 30.
    buildLightMap(pixels, { x: 30, y: 0, width: 4, height: 1 }, [{ z: 6, chunks: [chunk(1, 0, 6, [[SPRITE + 16, SPRITE + 16, 1, 180]], { grounds: [[1, 1]] })], shift: 1 }], 0, SPRITE, 6);
    expect(reds(pixels, 4)[0]).toEqual([0, 0, 51, 0]);
    expect(pixels[2 * 4 + 1]).toBe(0);
  });

  it("adds extra lights on their floor", () => {
    const pixels = new Uint8Array(9 * 4);
    buildLightMap(pixels, grid, [{ z: 7, chunks: [chunk(0, 0, 7, [], { grounds: ROW })], shift: 0, extra: [{ x: 4.5, y: 0.5, intensity: 2, color: 215 }] }], 0, SPRITE, 7);
    expect(reds(pixels, 9)[0]).toEqual([0, 0, 0, 51, 102, 51, 0, 0, 0]);
  });
});
