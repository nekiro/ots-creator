import { describe, expect, it } from "vitest";
import { decodeGeo, geoBits, GEO, QUAD_BYTES } from "./geo";

describe("decodeGeo", () => {
  it("decodes chunks with their parts", () => {
    const q = 20;
    const an = q + QUAD_BYTES;
    const co = an + 12;
    const sp = co + 16;
    const buf = new ArrayBuffer(sp + 4 + 20);
    const v = new DataView(buf);
    v.setUint32(0, 1234, true);
    v.setUint32(4, 1, true); // quads
    v.setUint32(8, 1, true); // animated
    v.setUint32(12, 1, true); // composed
    v.setUint32(16, 1, true); // sprites
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
    v.setUint32(sp + 4, 99, true); // second, empty chunk
    const [a, b] = decodeGeo(buf, "s");
    expect(a.key).toBe(1234);
    expect(a.n).toBe(1);
    expect(new DataView(a.quads.buffer, a.quads.byteOffset).getInt16(0, true)).toBe(-8);
    expect([...a.anim]).toEqual([0, 500, 3]);
    expect([...a.comp]).toEqual([0, 9, 10, 0x04030201]);
    expect([...a.sprites]).toEqual([77]);
    expect(b.key).toBe(99);
    expect(b.n).toBe(0);
  });

  it("packs view options", () => {
    const off = { pathing: false, houses: false, special: false, zones: false, spawns: false, highlightItems: false, ghostItems: false, creatures: false, onlyModified: false, animation: false };
    expect(geoBits(off)).toBe(0);
    expect(geoBits({ ...off, houses: true, animation: true })).toBe(GEO.houses | GEO.animation);
  });
});
