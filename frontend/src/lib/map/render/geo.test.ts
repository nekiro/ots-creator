import { describe, expect, it } from "vitest";
import { decodeGeo, decodeGeoSignature, encodeGeoSignature, geoBits, GEO, LIGHT_BYTES, QUAD_BYTES } from "./geo";

describe("decodeGeo", () => {
  it("decodes chunks with their parts", () => {
    const quadsAt = 24;
    const animatedAt = quadsAt + QUAD_BYTES;
    const composedAt = animatedAt + 12;
    const spritesAt = composedAt + 16;
    const lightsAt = spritesAt + 4;
    const shadesAt = lightsAt + LIGHT_BYTES;
    const buffer = new ArrayBuffer(shadesAt + 4 + 128 + 28);
    const view = new DataView(buffer);
    view.setUint32(0, 1234, true);
    view.setUint32(4, 1, true); // quads
    view.setUint32(8, 1, true); // animated
    view.setUint32(12, 1, true); // composed
    view.setUint32(16, 1, true); // sprites
    view.setUint32(20, 1, true); // lights
    view.setInt16(quadsAt, -8, true);
    view.setUint32(quadsAt + 4, 77, true);
    view.setUint32(animatedAt, 0, true); // animated: quad 0
    view.setUint32(animatedAt + 4, 500, true);
    view.setUint32(animatedAt + 8, 3, true);
    view.setUint32(composedAt, 0, true); // composed: quad 0
    view.setUint32(composedAt + 4, 9, true);
    view.setUint32(composedAt + 8, 10, true);
    view.setUint32(composedAt + 12, 0x04030201, true);
    view.setUint32(spritesAt, 77, true); // sprite list
    view.setInt16(lightsAt, -16, true); // light at (-16, 48), level 5, color 215
    view.setInt16(lightsAt + 2, 48, true);
    view.setUint8(lightsAt + 4, 5);
    view.setUint8(lightsAt + 5, 215);
    view.setUint32(shadesAt, 128, true); // shade bits: tile (1, 0)
    view.setUint8(shadesAt + 4, 2);
    view.setUint32(shadesAt + 4 + 128, 99, true); // second, empty chunk
    const [first, second] = decodeGeo(buffer, "s");
    expect(first.key).toBe(1234);
    expect(first.quadCount).toBe(1);
    expect(new DataView(first.quads.buffer, first.quads.byteOffset).getInt16(0, true)).toBe(-8);
    expect([...first.animated]).toEqual([0, 500, 3]);
    expect([...first.composed]).toEqual([0, 9, 10, 0x04030201]);
    expect([...first.sprites]).toEqual([77]);
    expect(first.lightCount).toBe(1);
    expect([first.lights.getInt16(0, true), first.lights.getInt16(2, true), first.lights.getUint8(4), first.lights.getUint8(5)]).toEqual([-16, 48, 5, 215]);
    expect(second.key).toBe(99);
    expect(second.quadCount).toBe(0);
    expect(first.shades?.length).toBe(128);
    expect(first.shades?.[0]).toBe(2);
    expect(second.lightCount).toBe(0);
    expect(second.shades).toBeNull();
  });

  it("packs view options", () => {
    const off = { pathing: false, houses: false, special: false, zones: false, spawns: false, highlightItems: false, ghostItems: false, creatures: false, onlyModified: false, animation: false };
    expect(geoBits(off)).toBe(0);
    expect(geoBits({ ...off, houses: true, animation: true })).toBe(GEO.houses | GEO.animation);
  });
});

describe("geo signature", () => {
  it("round trips", () => {
    const signature = { bits: GEO.houses | GEO.zones, house: 12, blockSize: 4 };
    expect(encodeGeoSignature(signature)).toBe(`${GEO.houses | GEO.zones}|12|4`);
    expect(decodeGeoSignature(encodeGeoSignature(signature))).toEqual(signature);
  });
});
