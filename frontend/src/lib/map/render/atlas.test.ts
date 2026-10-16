import { describe, expect, it } from "vitest";
import { Atlas } from "./atlas";
import { ATLAS } from "../constants";
import { fakeGl } from "./testing/fakeGl";

// Cells as large as a layer: one cell per layer keeps the counts small.
const CELL = ATLAS.size;
const pixels = () => new Uint8ClampedArray(4);

function atlas() {
  const fake = fakeGl();
  return { atlas: new Atlas(fake.gl, CELL), fake };
}

describe("Atlas", () => {
  it("keeps slot 0 for white and hands out cells from 1", () => {
    const { atlas: sprites } = atlas();
    sprites.begin();
    expect(sprites.cellOf(10, pixels)).toBe(1);
    expect(sprites.cellOf(11, pixels)).toBe(2);
  });

  it("uploads a key once", () => {
    const { atlas: sprites, fake } = atlas();
    sprites.begin();
    const before = fake.callsOf("texSubImage3D").length;
    const cell = sprites.cellOf(10, pixels);
    expect(sprites.cellOf(10, pixels)).toBe(cell);
    expect(fake.callsOf("texSubImage3D").length).toBe(before + 1);
  });

  it("waits for pixels that are not there yet", () => {
    const { atlas: sprites } = atlas();
    sprites.begin();
    expect(sprites.cellOf(10, () => undefined)).toBe(-1);
    expect(sprites.cellOf(10, pixels)).toBe(1);
  });

  it("grows by doubling the layers", () => {
    const { atlas: sprites } = atlas();
    sprites.begin();
    for (let key = 0; key < ATLAS.startLayers; key++) sprites.cellOf(key, pixels);
    expect(sprites.layers).toBe(ATLAS.startLayers * 2);
  });

  it("reuses the least recently used cells when full", () => {
    const { atlas: sprites } = atlas();
    const released: unknown[] = [];
    sprites.onRelease = (key) => released.push(key);
    const capacity = ATLAS.maxLayers - 1;
    for (let key = 0; key < capacity; key++) {
      sprites.begin();
      sprites.cellOf(key, pixels);
    }
    expect(sprites.layers).toBe(ATLAS.maxLayers);
    sprites.begin();
    // Key 0 is used again, so key 1 is now the oldest.
    sprites.cellOf(0, pixels);
    const epoch = sprites.epoch;
    expect(sprites.cellOf("new", pixels)).toBeGreaterThan(0);
    expect(released[0]).toBe(1);
    expect(released).not.toContain(0);
    expect(sprites.epoch).toBeGreaterThan(epoch);
  });

  it("drops composed images of an invalidated sprite", () => {
    const { atlas: sprites } = atlas();
    const released: unknown[] = [];
    sprites.onRelease = (key) => released.push(key);
    sprites.begin();
    sprites.cellOf(5, pixels);
    sprites.cellOf("5:6:7", pixels);
    sprites.cellOf("8:9", pixels);
    sprites.invalidate([6]);
    expect(released).toEqual(["5:6:7"]);
    sprites.invalidate(null);
    expect(released.sort()).toEqual(["5:6:7", 5, "8:9"].sort());
  });
});
