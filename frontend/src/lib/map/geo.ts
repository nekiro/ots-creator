// Chunk geometry built by the backend (/res/map/{id}/geo, see
// app/map_geo.go): the quads of a 32x32 chunk ready for the GPU, fetched
// per chunk and the view options they depend on.
import { res } from "./api";

/** View option bits of the geometry (app/map_geo.go). */
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

export function geoBits(o: GeoView): number {
  return (
    (o.pathing ? GEO.pathing : 0) |
    (o.houses ? GEO.houses : 0) |
    (o.special ? GEO.special : 0) |
    (o.zones ? GEO.zones : 0) |
    (o.spawns ? GEO.spawns : 0) |
    (o.highlightItems ? GEO.highlight : 0) |
    (o.ghostItems ? GEO.ghostItems : 0) |
    (o.creatures ? GEO.creatures : 0) |
    (o.onlyModified ? GEO.onlyModified : 0) |
    (o.animation ? GEO.animation : 0)
  );
}

export interface GeoChunk {
  key: number;
  /** Quads, QUAD_BYTES each, in world pixels from the chunk's corner. */
  quads: Uint8Array;
  n: number;
  /** Animated pieces, three values each: quad, client item id, frame 0 sprite index. */
  anim: Uint32Array;
  /** Colored outfit pieces, four values each: quad, sprite, template sprite, colors (head, body, legs, feet bytes). */
  comp: Uint32Array;
  /** Every plain sprite the quads use. */
  sprites: Uint32Array;
  /** What it was built for. */
  sig: string;
}

/** Decodes a geometry response. */
export function decodeGeo(buf: ArrayBuffer, sig: string): GeoChunk[] {
  const v = new DataView(buf);
  const out: GeoChunk[] = [];
  let p = 0;
  while (p + 20 <= buf.byteLength) {
    const key = v.getUint32(p, true);
    const n = v.getUint32(p + 4, true);
    const na = v.getUint32(p + 8, true);
    const nc = v.getUint32(p + 12, true);
    const ns = v.getUint32(p + 16, true);
    p += 20;
    const quads = new Uint8Array(buf, p, n * QUAD_BYTES);
    p += n * QUAD_BYTES;
    // Every part is a multiple of 4 bytes long, so the views stay aligned.
    const anim = new Uint32Array(buf, p, na * 3);
    p += na * 12;
    const comp = new Uint32Array(buf, p, nc * 4);
    p += nc * 16;
    const sprites = new Uint32Array(buf, p, ns);
    p += ns * 4;
    out.push({ key, quads, n, anim, comp, sprites, sig });
  }
  return out;
}

const BATCH = 64;

interface Entry {
  chunk: GeoChunk | null;
  sig: string;
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
  private seq = 0;
  private timer = 0;
  /** Recent requests: when each ended, its whole time and the backend's build time (ms). */
  private times: { at: number; total: number; build: number }[] = [];
  /** The loading burst under way (from the first request until nothing loads), logged when it ends. */
  private burst: { start: number; reqs: number; chunks: number; bytes: number; packed: number; total: number; max: number; build: number; wait: number } | null = null;

  constructor(
    readonly mapId: number,
    private onload: () => void,
  ) {}

  /**
   * The geometry of a chunk: null when it is empty, undefined when none is
   * loaded yet. sig is "opts|house|block" (block > 1: key is a block key,
   * see GEO_BLOCK).
   */
  get(key: number, sig: string): GeoChunk | null | undefined {
    const e = this.chunks.get(key);
    const fresh = e && !e.stale && e.sig === sig;
    if (!fresh && this.inflight.get(key) !== sig) this.queue(key, sig);
    return e?.chunk;
  }

  private queue(key: number, sig: string): void {
    this.burst ??= { start: performance.now(), reqs: 0, chunks: 0, bytes: 0, packed: 0, total: 0, max: 0, build: 0, wait: 0 };
    this.inflight.set(key, sig);
    let s = this.queued.get(sig);
    if (!s) this.queued.set(sig, (s = new Set()));
    s.add(key);
    if (!this.timer) this.timer = requestAnimationFrame(() => this.flush());
  }

  private flush(): void {
    this.timer = 0;
    for (const [sig, set] of this.queued) {
      const keys = [...set];
      for (let i = 0; i < keys.length; i += BATCH) void this.fetch(keys.slice(i, i + BATCH), sig);
    }
    this.queued.clear();
  }

  private async fetch(keys: number[], sig: string): Promise<void> {
    const seq = ++this.seq;
    for (const k of keys) this.latest.set(k, seq);
    const [o, h, b] = sig.split("|");
    const base = +b > 1 ? GEO_BLOCK : 0;
    const started = performance.now();
    try {
      const r = await fetch(res.geo(this.mapId, keys.map((k) => k - base), +o, +h, +b) + `&q=${seq}`);
      if (!r.ok) throw new Error(await r.text());
      const head = performance.now();
      // The backend sends raw deflate (see app/map_geo.go buildGeo).
      const buf = await new Response(r.body!.pipeThrough(new DecompressionStream("deflate-raw"))).arrayBuffer();
      const chunks = decodeGeo(buf, sig);
      const now = performance.now();
      const build = Number(r.headers.get("X-Build-Ms")) || 0;
      this.times.push({ at: now, total: now - started, build });
      const st = this.burst;
      if (st) {
        st.reqs++;
        st.chunks += keys.length;
        st.bytes += buf.byteLength;
        st.packed += Number(r.headers.get("X-Packed-Bytes")) || 0;
        st.total += now - started;
        st.max = Math.max(st.max, now - started);
        st.build += build;
        // Until the answer started: queue, lock and build.
        st.wait += head - started;
      }
      for (const c of chunks) {
        c.key += base;
        if (this.latest.get(c.key) !== seq) continue;
        this.chunks.set(c.key, { chunk: c.n ? c : null, sig, stale: false });
      }
      this.onload();
    } catch {
      // The map was closed or the request failed: try again on demand.
    } finally {
      for (const k of keys) {
        if (this.latest.get(k) === seq) {
          this.latest.delete(k);
          if (this.inflight.get(k) === sig) this.inflight.delete(k);
        }
      }
      this.logBurst();
    }
  }

  /** Logs the burst once nothing loads any more. */
  private logBurst(): void {
    const b = this.burst;
    if (!b || !this.idle()) return;
    this.burst = null;
    if (!b.reqs) return;
    const n = b.reqs;
    const line =
      `[map geo] ${b.chunks} chunks in ${n} requests, ${(b.bytes / 1048576).toFixed(1)} MB (${(b.packed / 1048576).toFixed(1)} MB sent), ` +
        `done after ${Math.round(performance.now() - b.start)} ms; per request: ` +
        `${Math.round(b.total / n)} ms avg, ${Math.round(b.max)} ms max, ` +
        `first byte ${Math.round(b.wait / n)} ms, Go build ${(b.build / n).toFixed(1)} ms`;
    console.log(line);
    // Also to map-perf.log in the config directory (no devtools needed).
    void fetch("/res/perflog", { method: "POST", body: line }).catch(() => {});
  }

  /** Marks chunks (all when keys is null) changed: they load again, the old version shows meanwhile. */
  invalidate(keys: number[] | null): void {
    if (keys === null) {
      for (const e of this.chunks.values()) e.stale = true;
      // Answers in flight may hold the old data: drop them and ask again.
      this.inflight.clear();
      this.latest.clear();
      return;
    }
    const all = new Set<number>();
    for (const k of keys) {
      all.add(k);
      // The blocks holding the chunk, for every block size the renderer uses.
      const [cx, cy, z] = [Math.floor(k / 32768), Math.floor(k / 16) & 0x7ff, k & 15];
      for (const n of [2, 4, 8]) all.add(GEO_BLOCK + (cx - (cx % n)) * 32768 + (cy - (cy % n)) * 16 + z);
    }
    for (const k of all) {
      const e = this.chunks.get(k);
      if (e) e.stale = true;
      this.inflight.delete(k);
      this.latest.delete(k);
    }
  }

  /** Requests that ended in the last second: count and average times (ms). */
  stats(): { count: number; total: number; build: number } {
    const now = performance.now();
    this.times = this.times.filter((t) => now - t.at < 1000);
    const n = this.times.length;
    if (!n) return { count: 0, total: 0, build: 0 };
    const sum = (f: (t: (typeof this.times)[number]) => number) => this.times.reduce((a, t) => a + f(t), 0) / n;
    return { count: n, total: sum((t) => t.total), build: sum((t) => t.build) };
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
    if (this.chunks.size <= max) return;
    for (const k of this.chunks.keys()) {
      if (!keep(k)) this.chunks.delete(k);
      if (this.chunks.size <= max * 0.75) break;
    }
  }
}
