import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ChunkStore, ItemFlags, TileFlags, chunkKey, decodeChunks, keyOf, splitKey, tileIndex } from "./chunks";

/** Writes the chunk response format (see app/map_http.go) little endian. */
class Writer {
  private bytes: number[] = [];
  u8(value: number) {
    this.bytes.push(value & 0xff);
    return this;
  }
  u16(value: number) {
    return this.u8(value).u8(value >> 8);
  }
  u32(value: number) {
    return this.u16(value).u16(value >>> 16);
  }
  text8(value: string) {
    const encoded = new TextEncoder().encode(value);
    this.u8(encoded.length);
    for (const byte of encoded) this.u8(byte);
    return this;
  }
  text16(value: string) {
    const encoded = new TextEncoder().encode(value);
    this.u16(encoded.length);
    for (const byte of encoded) this.u8(byte);
    return this;
  }
  buffer(): ArrayBuffer {
    return new Uint8Array(this.bytes).buffer;
  }
}

/** A chunk with one tile holding one item and nothing else. */
function simpleChunk(writer: Writer, key: number, index: number, itemId: number): Writer {
  return writer.u32(key).u16(1).u16(index).u16(0).u8(1).u16(itemId).u16(0).u8(0);
}

describe("chunk keys", () => {
  it("round trips chunk coordinates", () => {
    const key = chunkKey(100, 2000, 7);
    expect(splitKey(key)).toEqual([100, 2000, 7]);
  });

  it("finds the chunk of a tile", () => {
    expect(keyOf(32 * 3 + 5, 32 * 4 + 31, 8)).toBe(chunkKey(3, 4, 8));
    expect(keyOf(0, 0, 0)).toBe(0);
  });

  it("indexes tiles row by row", () => {
    expect(tileIndex(0, 0)).toBe(0);
    expect(tileIndex(1, 0)).toBe(1);
    expect(tileIndex(0, 1)).toBe(32);
    expect(tileIndex(32 + 2, 64 + 3)).toBe(3 * 32 + 2);
  });
});

describe("decodeChunks", () => {
  it("is empty for an empty response", () => {
    expect(decodeChunks(new ArrayBuffer(0))).toEqual([]);
  });

  it("decodes every tile part", () => {
    const flags = TileFlags.house | TileFlags.creatures | TileFlags.spawn | TileFlags.spawnNpc | TileFlags.zones | TileFlags.text | TileFlags.pz;
    const writer = new Writer().u32(chunkKey(1, 2, 7)).u16(1);
    writer.u16(33).u16(flags).u32(77);
    writer.u8(2).u16(100).u16(0).u8(0).u16(200).u16(5).u8(ItemFlags.text);
    // creatures: an npc facing 2, selected
    writer.u8(1).u8(1 | 2 | (2 << 2)).u16(128).u8(1).u8(2).u8(3).u8(4).u8(3).u16(9).u16(0).text8("Bob");
    writer.u16(5).u8(1);
    writer.u16(2).u8(0);
    writer.u8(2).u16(10).u16(11);
    writer.text16("Hello");
    const [chunk] = decodeChunks(writer.buffer());
    expect(chunk.key).toBe(chunkKey(1, 2, 7));
    expect(chunk.count).toBe(1);
    const tile = chunk.tiles[33]!;
    expect(tile.flags).toBe(flags);
    expect(tile.house).toBe(77);
    expect([...tile.ids]).toEqual([100, 200]);
    expect([...tile.subs]).toEqual([0, 5]);
    expect([...tile.iflags]).toEqual([0, ItemFlags.text]);
    expect(tile.creatures).toEqual([{ npc: true, selected: true, dir: 2, lookType: 128, head: 1, body: 2, legs: 3, feet: 4, addons: 3, mount: 9, lookItem: 0, name: "Bob" }]);
    expect(tile.spawn).toEqual({ radius: 5, selected: true });
    expect(tile.spawnNpc).toEqual({ radius: 2, selected: false });
    expect(tile.zones).toEqual([10, 11]);
    expect(tile.text).toBe("Hello");
  });

  it("decodes several chunks", () => {
    const writer = new Writer();
    simpleChunk(writer, 5, 0, 100);
    simpleChunk(writer, 6, 1023, 101);
    const chunks = decodeChunks(writer.buffer());
    expect(chunks.map((chunk) => chunk.key)).toEqual([5, 6]);
    expect([...chunks[1].tiles[1023]!.ids]).toEqual([101]);
    expect(chunks[0].tiles[1]).toBeUndefined();
  });

  it("gives tiles without items empty arrays", () => {
    const writer = new Writer().u32(1).u16(1).u16(0).u16(0).u8(0);
    const tile = decodeChunks(writer.buffer())[0].tiles[0]!;
    expect(tile.ids).toHaveLength(0);
    expect(tile.creatures).toBeNull();
    expect(tile.text).toBeNull();
  });
});

describe("ChunkStore", () => {
  let frames: (() => void)[] = [];
  let responses: ArrayBuffer[] = [];

  beforeEach(() => {
    frames = [];
    responses = [];
    vi.stubGlobal("requestAnimationFrame", (callback: () => void) => {
      frames.push(callback);
      return frames.length;
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(responses.shift() ?? new ArrayBuffer(0))),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  async function runFrames() {
    const pending = frames.splice(0);
    for (const frame of pending) frame();
    // Let the fetches and decoding settle.
    for (let i = 0; i < 5; i++) await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 0));
  }

  it("loads requested chunks in one batch", async () => {
    const onload = vi.fn();
    const store = new ChunkStore(1, onload);
    const key = keyOf(40, 40, 7);
    responses.push(simpleChunk(new Writer(), key, tileIndex(40, 40), 100).buffer());
    expect(store.tile(40, 40, 7)).toBeUndefined();
    await runFrames();
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(onload).toHaveBeenCalled();
    expect([...store.peekTile(40, 40, 7)!.ids]).toEqual([100]);
  });

  it("remembers empty chunks as null", async () => {
    const store = new ChunkStore(1, () => {});
    responses.push(new Writer().u32(9).u16(0).buffer());
    store.get(9);
    await runFrames();
    expect(store.peek(9)).toBeNull();
  });

  it("rejects tiles off the map", () => {
    const store = new ChunkStore(1, () => {});
    expect(store.tile(-1, 0, 7)).toBeUndefined();
    expect(store.peekTile(0, 0, 16)).toBeUndefined();
    expect(frames).toHaveLength(0);
  });

  it("drops every chunk when invalidated without keys", async () => {
    const store = new ChunkStore(1, () => {});
    responses.push(simpleChunk(new Writer(), 3, 0, 100).buffer());
    store.get(3);
    await runFrames();
    store.invalidate(null, 4);
    expect(store.rev).toBe(4);
    expect(store.peek(3)).toBeUndefined();
  });

  it("refetches loaded chunks a change touched, keeping the old data meanwhile", async () => {
    const store = new ChunkStore(1, () => {});
    responses.push(simpleChunk(new Writer(), 3, 0, 100).buffer());
    store.get(3);
    await runFrames();
    responses.push(simpleChunk(new Writer(), 3, 0, 200).buffer());
    store.invalidate([3, 99], 1);
    expect([...store.peek(3)!.tiles[0]!.ids]).toEqual([100]);
    await runFrames();
    expect(fetch).toHaveBeenCalledTimes(2);
    expect([...store.peek(3)!.tiles[0]!.ids]).toEqual([200]);
  });

  it("trims chunks the view does not keep", async () => {
    const store = new ChunkStore(1, () => {});
    const writer = new Writer();
    for (let key = 0; key < 8; key++) simpleChunk(writer, key, 0, 100);
    responses.push(writer.buffer());
    for (let key = 0; key < 8; key++) store.get(key);
    await runFrames();
    store.trim((key) => key >= 6, 4);
    expect(store.peek(0)).toBeUndefined();
    expect(store.peek(6)).toBeDefined();
    expect(store.peek(7)).toBeDefined();
  });

  it("keeps everything under the limit", async () => {
    const store = new ChunkStore(1, () => {});
    responses.push(simpleChunk(new Writer(), 1, 0, 100).buffer());
    store.get(1);
    await runFrames();
    store.trim(() => false, 4);
    expect(store.peek(1)).toBeDefined();
  });
});
