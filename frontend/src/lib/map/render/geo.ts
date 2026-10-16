// Chunk geometry built by the backend (/res/map/{id}/geo, see
// app/map_http.go): the quads of a 32x32 chunk ready for the GPU, fetched
// per chunk and the view options they depend on.
import { res } from "../data/api";
import { chunkKey, inBatches, splitKey, trimStore } from "../data/chunks";
import { BLOCK_CHUNKS } from "../constants";

/** View option bits of the geometry (app/map_http.go). */
export const GEO = {
  pathing: 1,
  houses: 2,
  special: 4,
  zones: 8,
  spawns: 16,
  highlight: 32,
  ghostItems: 64,
  creatures: 128,
  onlyModified: 256,
  animation: 512,
  /** Leave out movable and pickupable items (seen from far away). */
  far: 1024,
} as const;

/**
 * Sprite values of a quad besides sprite ids (sprite quads are one sprite
 * size a side): hidden, a composed outfit piece (low bits: its index) and a
 * colored rectangle (width in bits 15-29, height in bits 0-14).
 */
export const GEO_HIDDEN = 0xffffffff;
export const GEO_COMPOSED = 0x80000000;
export const GEO_RECT = 0x40000000;
/** Added to the key of a block's top-left chunk: the key of the block (blocks and chunks share a store). */
export const GEO_BLOCK = 1 << 28;
/** Bytes per light source (see GeoChunk.lights). */
export const LIGHT_BYTES = 8;
/** Bytes per quad: int16 x, y, uint32 sprite, rgba8 color, uint16 x, y of its tile (from the chunk's corner). */
export const QUAD_BYTES = 16;

export interface GeoView {
  pathing: boolean;
  houses: boolean;
  special: boolean;
  zones: boolean;
  spawns: boolean;
  highlightItems: boolean;
  ghostItems: boolean;
  creatures: boolean;
  onlyModified: boolean;
  animation: boolean;
}

export function geoBits(view: GeoView): number {
  return (
    (view.pathing ? GEO.pathing : 0) |
    (view.houses ? GEO.houses : 0) |
    (view.special ? GEO.special : 0) |
    (view.zones ? GEO.zones : 0) |
    (view.spawns ? GEO.spawns : 0) |
    (view.highlightItems ? GEO.highlight : 0) |
    (view.ghostItems ? GEO.ghostItems : 0) |
    (view.creatures ? GEO.creatures : 0) |
    (view.onlyModified ? GEO.onlyModified : 0) |
    (view.animation ? GEO.animation : 0)
  );
}

/** What chunk geometry is built for: the view option bits, the highlighted house and the block size. */
export interface GeoSignature {
  bits: number;
  house: number;
  /** Chunks a side (above 1: keys are block keys, see GEO_BLOCK). */
  blockSize: number;
}

/** The signature as a string ("bits|house|blockSize"), the key geometry is stored under. */
export function encodeGeoSignature({ bits, house, blockSize }: GeoSignature): string {
  return `${bits}|${house}|${blockSize}`;
}

export function decodeGeoSignature(signature: string): GeoSignature {
  const [bits, house, blockSize] = signature.split("|").map(Number);
  return { bits, house, blockSize };
}

export interface GeoChunk {
  key: number;
  /** Quads, QUAD_BYTES each, in world pixels from the chunk's corner. */
  quads: Uint8Array;
  quadCount: number;
  /** Animated pieces, three values each: quad, client item id, frame 0 sprite index. */
  animated: Uint32Array;
  /** Colored outfit pieces, four values each: quad, sprite, template sprite, colors (head, body, legs, feet bytes). */
  composed: Uint32Array;
  /** Every plain sprite the quads use. */
  sprites: Uint32Array;
  /** Light sources, LIGHT_BYTES each: int16 x, y (world pixels from the chunk's corner), uint8 level, uint8 color (8-bit client color). */
  lights: DataView;
  /** Light sources in lights. */
  lightCount: number;
  /** Tiles with a ground, one bit each row by row (null: none): they hide the light of the floors under them. */
  shades: Uint8Array | null;
  /** What it was built for. */
  signature: string;
}

/** Decodes a geometry response. */
export function decodeGeo(buffer: ArrayBuffer, signature: string): GeoChunk[] {
  const view = new DataView(buffer);
  const out: GeoChunk[] = [];
  let offset = 0;
  while (offset + 24 <= buffer.byteLength) {
    const key = view.getUint32(offset, true);
    const quadCount = view.getUint32(offset + 4, true);
    const animCount = view.getUint32(offset + 8, true);
    const composedCount = view.getUint32(offset + 12, true);
    const spriteCount = view.getUint32(offset + 16, true);
    const lightCount = view.getUint32(offset + 20, true);
    offset += 24;
    const quads = new Uint8Array(buffer, offset, quadCount * QUAD_BYTES);
    offset += quadCount * QUAD_BYTES;
    // Every part is a multiple of 4 bytes long, so the views stay aligned.
    const animated = new Uint32Array(buffer, offset, animCount * 3);
    offset += animCount * 12;
    const composed = new Uint32Array(buffer, offset, composedCount * 4);
    offset += composedCount * 16;
    const sprites = new Uint32Array(buffer, offset, spriteCount);
    offset += spriteCount * 4;
    const lights = new DataView(buffer, offset, lightCount * LIGHT_BYTES);
    offset += lightCount * LIGHT_BYTES;
    const shadeBytes = view.getUint32(offset, true);
    offset += 4;
    const shades = shadeBytes ? new Uint8Array(buffer, offset, shadeBytes) : null;
    offset += shadeBytes;
    out.push({ key, quads, quadCount, animated, composed, sprites, lights, lightCount, shades, signature });
  }
  return out;
}

/** Chunks per request. */
const BATCH = 64;

interface Entry {
  chunk: GeoChunk | null;
  signature: string;
  /** Changed on the map since it was fetched. */
  stale: boolean;
}

/**
 * Chunk geometry of one map. get() returns what is loaded (also when it
 * was built for other options or is stale) and requests a fresh version.
 */
export class GeoStore {
  private chunks = new Map<number, Entry>();
  /** Signature of the newest request of each chunk in flight. */
  private inflight = new Map<number, string>();
  private queued = new Map<string, Set<number>>();
  /** Request number of the newest request of each chunk: older answers are dropped. */
  private latest = new Map<number, number>();
  private requestNumber = 0;
  private timer = 0;
  /** Recent requests: when each ended, its whole time and the backend's build time (ms). */
  private times: { at: number; total: number; build: number }[] = [];
  /** The loading burst under way (from the first request until nothing loads), logged when it ends. */
  private burst: { start: number; requests: number; chunks: number; bytes: number; packed: number; total: number; max: number; build: number; wait: number } | null = null;

  constructor(
    readonly mapId: number,
    private onload: () => void,
  ) {}

  /**
   * The geometry of a chunk: null when it is empty, undefined when none is
   * loaded yet. signature is "opts|house|block" (block > 1: key is a block
   * key, see GEO_BLOCK).
   */
  get(key: number, signature: string): GeoChunk | null | undefined {
    const entry = this.chunks.get(key);
    const fresh = entry && !entry.stale && entry.signature === signature;
    if (!fresh && this.inflight.get(key) !== signature) this.queue(key, signature);
    return entry?.chunk;
  }

  private queue(key: number, signature: string): void {
    this.burst ??= { start: performance.now(), requests: 0, chunks: 0, bytes: 0, packed: 0, total: 0, max: 0, build: 0, wait: 0 };
    this.inflight.set(key, signature);
    let keys = this.queued.get(signature);
    if (!keys) this.queued.set(signature, (keys = new Set()));
    keys.add(key);
    if (!this.timer) this.timer = requestAnimationFrame(() => this.flush());
  }

  private flush(): void {
    this.timer = 0;
    for (const [signature, keys] of this.queued) inBatches([...keys], BATCH, (batch) => void this.fetch(batch, signature));
    this.queued.clear();
  }

  private async fetch(keys: number[], signature: string): Promise<void> {
    const requestNumber = ++this.requestNumber;
    for (const key of keys) this.latest.set(key, requestNumber);
    const { bits, house, blockSize } = decodeGeoSignature(signature);
    const base = blockSize > 1 ? GEO_BLOCK : 0;
    const started = performance.now();
    try {
      const response = await fetch(res.geo(this.mapId, keys.map((key) => key - base), bits, house, blockSize) + `&q=${requestNumber}`);
      if (!response.ok) throw new Error(await response.text());
      const firstByteAt = performance.now();
      // The backend sends raw deflate (see app/map_http.go buildGeo).
      const buffer = await new Response(response.body!.pipeThrough(new DecompressionStream("deflate-raw"))).arrayBuffer();
      const chunks = decodeGeo(buffer, signature);
      const now = performance.now();
      const build = Number(response.headers.get("X-Build-Ms")) || 0;
      this.times.push({ at: now, total: now - started, build });
      const burst = this.burst;
      if (burst) {
        burst.requests++;
        burst.chunks += keys.length;
        burst.bytes += buffer.byteLength;
        burst.packed += Number(response.headers.get("X-Packed-Bytes")) || 0;
        burst.total += now - started;
        burst.max = Math.max(burst.max, now - started);
        burst.build += build;
        // Until the answer started: queue, lock and build.
        burst.wait += firstByteAt - started;
      }
      for (const chunk of chunks) {
        chunk.key += base;
        if (this.latest.get(chunk.key) !== requestNumber) continue;
        this.chunks.set(chunk.key, { chunk: chunk.quadCount ? chunk : null, signature, stale: false });
      }
      this.onload();
    } catch {
      // The map was closed or the request failed: try again on demand.
    } finally {
      for (const key of keys) {
        if (this.latest.get(key) === requestNumber) {
          this.latest.delete(key);
          if (this.inflight.get(key) === signature) this.inflight.delete(key);
        }
      }
      this.logBurst();
    }
  }

  /** Logs the burst once nothing loads any more. */
  private logBurst(): void {
    const burst = this.burst;
    if (!burst || !this.idle()) return;
    this.burst = null;
    if (!burst.requests) return;
    const requests = burst.requests;
    const line =
      `[map geo] ${burst.chunks} chunks in ${requests} requests, ${(burst.bytes / 1048576).toFixed(1)} MB (${(burst.packed / 1048576).toFixed(1)} MB sent), ` +
        `done after ${Math.round(performance.now() - burst.start)} ms; per request: ` +
        `${Math.round(burst.total / requests)} ms avg, ${Math.round(burst.max)} ms max, ` +
        `first byte ${Math.round(burst.wait / requests)} ms, Go build ${(burst.build / requests).toFixed(1)} ms`;
    console.log(line);
    // Also to map-perf.log in the config directory (no devtools needed).
    void fetch("/res/perflog", { method: "POST", body: line }).catch(() => {});
  }

  /** Marks chunks (all when keys is null) changed: they load again, the old version shows meanwhile. */
  invalidate(keys: number[] | null): void {
    if (keys === null) {
      for (const entry of this.chunks.values()) entry.stale = true;
      // Answers in flight may hold the old data: drop them and ask again.
      this.inflight.clear();
      this.latest.clear();
      return;
    }
    const all = new Set<number>();
    for (const key of keys) {
      all.add(key);
      // The block holding the chunk, of the block size the renderer uses.
      const [chunkX, chunkY, z] = splitKey(key);
      all.add(GEO_BLOCK + chunkKey(chunkX - (chunkX % BLOCK_CHUNKS), chunkY - (chunkY % BLOCK_CHUNKS), z));
    }
    for (const key of all) {
      const entry = this.chunks.get(key);
      if (entry) entry.stale = true;
      this.inflight.delete(key);
      this.latest.delete(key);
    }
  }

  /** Requests that ended in the last second: count and average times (ms). */
  stats(): { count: number; total: number; build: number } {
    const now = performance.now();
    this.times = this.times.filter((time) => now - time.at < 1000);
    const count = this.times.length;
    if (!count) return { count: 0, total: 0, build: 0 };
    const average = (pick: (time: (typeof this.times)[number]) => number) => this.times.reduce((sum, time) => sum + pick(time), 0) / count;
    return { count, total: average((time) => time.total), build: average((time) => time.build) };
  }

  /** Nothing is requested or loading. */
  idle(): boolean {
    return this.inflight.size === 0 && this.queued.size === 0;
  }

  /** The geometry of a chunk without requesting it. */
  peek(key: number): GeoChunk | null | undefined {
    return this.chunks.get(key)?.chunk;
  }

  /** Forgets a chunk: the next get() fetches it again. */
  drop(key: number): void {
    this.chunks.delete(key);
  }

  /** Forgets chunks far from a view to bound memory. */
  trim(keep: (key: number) => boolean, max = 12000): void {
    trimStore(this.chunks, keep, max);
  }
}
