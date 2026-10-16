import { describe, expect, it } from "vitest";
import { decodeGeo, decodeGeoSignature, encodeGeoSignature, geoBits, GEO, LIGHT_BYTES, QUAD_BYTES } from "./geo";

describe("decodeGeo", () => {
  it("decodes chunks with their parts", () => {
    const q = 24;
    const an = q + QUAD_BYTES;
    const co = an + 12;
    const sp = co + 16;
    const li = sp + 4;
    const sh = li + LIGHT_BYTES;
    const buf = new ArrayBuffer(sh + 4 + 128 + 28);
    const v = new DataView(buf);
    v.setUint32(0, 1234, true);
    v.setUint32(4, 1, true); // quads
    v.setUint32(8, 1, true); // animated
    v.setUint32(12, 1, true); // composed
    v.setUint32(16, 1, true); // sprites
    v.setUint32(20, 1, true); // lights
    v.setInt16(q, -8, true);
    v.setUint32(q + 4, 77, true);
    v.setUint32(an, 0, true); // anim: quad 0
    v.setUint32(an + 4, 500, true);
    v.setUint32(an + 8, 3, true);
    v.setUint32(co, 0, true); // comp: quad 0
    v.setUint32(co + 4, 9, true);
    v.setUint32(co + 8, 10, true);
    v.setUint32(co + 12, 0x04030201, true);
    v.setUint32(sp, 77, true); // sprite list
    v.setInt16(li, -16, true); // light at (-16, 48), level 5, color 215
    v.setInt16(li + 2, 48, true);
    v.setUint8(li + 4, 5);
    v.setUint8(li + 5, 215);
    v.setUint32(sh, 128, true); // shade bits: tile (1, 0)
    v.setUint8(sh + 4, 2);
    v.setUint32(sh + 4 + 128, 99, true); // second, empty chunk
    const [a, b] = decodeGeo(buf, "s");
    expect(a.key).toBe(1234);
    expect(a.n).toBe(1);
    expect(new DataView(a.quads.buffer, a.quads.byteOffset).getInt16(0, true)).toBe(-8);
    expect([...a.anim]).toEqual([0, 500, 3]);
    expect([...a.comp]).toEqual([0, 9, 10, 0x04030201]);
    expect([...a.sprites]).toEqual([77]);
    expect(a.lightCount).toBe(1);
    expect([a.lights.getInt16(0, true), a.lights.getInt16(2, true), a.lights.getUint8(4), a.lights.getUint8(5)]).toEqual([-16, 48, 5, 215]);
    expect(b.key).toBe(99);
    expect(b.n).toBe(0);
    expect(a.shades?.length).toBe(128);
    expect(a.shades?.[0]).toBe(2);
    expect(b.lightCount).toBe(0);
    expect(b.shades).toBeNull();
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
