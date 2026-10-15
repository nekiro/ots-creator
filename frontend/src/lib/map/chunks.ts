// Map tiles fetched from /res/map/{id}/chunks in 32x32 chunks per floor.
// Only chunks a change touched are fetched again.
import { res } from "./api";

export const CHUNK = 32;

/** Tile flags of the chunk format (see app/map_resources.go). */
export const TF = {
  selected: 1,
  modified: 2,
  house: 4,
  pz: 8,
  noPvp: 16,
  noLogout: 32,
  pvpZone: 64,
  creatures: 128,
  spawn: 256,
  spawnNpc: 512,
  zones: 1024,
  optional: 2048,
  blocking: 4096,
  text: 8192,
  ground: 0x4000,
} as const;

/** Item flags of the chunk format. */
export const IF = { selected: 1, ids: 2, text: 4, teleport: 8, contents: 16, doorId: 32 } as const;

export interface CreatureData {
  npc: boolean;
  selected: boolean;
  dir: number;
  lookType: number;
  head: number;
  body: number;
  legs: number;
  feet: number;
  addons: number;
  mount: number;
  lookItem: number;
  name: string;
}

export interface TileData {
  flags: number;
  house: number;
  /** Items bottom to top; the ground first when flags has TF.ground. */
  ids: Uint16Array;
  subs: Uint16Array;
  iflags: Uint8Array;
  creatures: CreatureData[] | null;
  spawn: { radius: number; selected: boolean } | null;
  spawnNpc: { radius: number; selected: boolean } | null;
  zones: number[] | null;
  /** Text of the topmost item with one (its start). */
  text: string | null;
}

export interface ChunkData {
  key: number;
  tiles: (TileData | undefined)[];
  count: number;
}

export function chunkKey(cx: number, cy: number, z: number): number {
  return cx * 32768 + cy * 16 + z;
}

export function keyOf(x: number, y: number, z: number): number {
  return chunkKey(x >> 5, y >> 5, z);
}

export function splitKey(k: number): [number, number, number] {
  return [Math.floor(k / 32768), Math.floor(k / 16) & 0x7ff, k & 15];
}

const decoder = new TextDecoder();

const NO_IDS = new Uint16Array(0);
const NO_FLAGS = new Uint8Array(0);

/** Decodes a chunk response into chunks. */
export function decodeChunks(buf: ArrayBuffer): ChunkData[] {
  const v = new DataView(buf);
  const bytes = new Uint8Array(buf);
  let p = 0;
  const out: ChunkData[] = [];
  // The item arrays of all tiles are views into three shared buffers: an
  // array buffer per tile costs several times its few bytes. An item takes
  // at least 5 bytes of the response, which bounds the item count.
  const room = Math.floor(buf.byteLength / 5) + 1;
  const allIds = new Uint16Array(room);
  const allSubs = new Uint16Array(room);
  const allFlags = new Uint8Array(room);
  let q = 0;
  while (p + 6 <= buf.byteLength) {
    const key = v.getUint32(p, true);
    const count = v.getUint16(p + 4, true);
    p += 6;
    const tiles: (TileData | undefined)[] = new Array(CHUNK * CHUNK);
    for (let t = 0; t < count; t++) {
      const idx = v.getUint16(p, true);
      const flags = v.getUint16(p + 2, true);
      p += 4;
      let house = 0;
      if (flags & TF.house) {
        house = v.getUint32(p, true);
        p += 4;
      }
      const n = v.getUint8(p++);
      const ids = n ? allIds.subarray(q, q + n) : NO_IDS;
      const subs = n ? allSubs.subarray(q, q + n) : NO_IDS;
      const iflags = n ? allFlags.subarray(q, q + n) : NO_FLAGS;
      q += n;
      for (let i = 0; i < n; i++) {
        ids[i] = v.getUint16(p, true);
        subs[i] = v.getUint16(p + 2, true);
        iflags[i] = v.getUint8(p + 4);
        p += 5;
      }
      let creatures: CreatureData[] | null = null;
      if (flags & TF.creatures) {
        const cn = v.getUint8(p++);
        creatures = [];
        for (let i = 0; i < cn; i++) {
          const cf = v.getUint8(p);
          const c: CreatureData = {
            npc: (cf & 1) !== 0,
            selected: (cf & 2) !== 0,
            dir: (cf >> 2) & 3,
            lookType: v.getUint16(p + 1, true),
            head: v.getUint8(p + 3),
            body: v.getUint8(p + 4),
            legs: v.getUint8(p + 5),
            feet: v.getUint8(p + 6),
            addons: v.getUint8(p + 7),
            mount: v.getUint16(p + 8, true),
            lookItem: v.getUint16(p + 10, true),
            name: "",
          };
          const len = v.getUint8(p + 12);
          p += 13;
          c.name = decoder.decode(bytes.subarray(p, p + len));
          p += len;
          creatures.push(c);
        }
      }
      let spawn: TileData["spawn"] = null;
      let spawnNpc: TileData["spawnNpc"] = null;
      if (flags & TF.spawn) {
        spawn = { radius: v.getUint16(p, true), selected: v.getUint8(p + 2) === 1 };
        p += 3;
      }
      if (flags & TF.spawnNpc) {
        spawnNpc = { radius: v.getUint16(p, true), selected: v.getUint8(p + 2) === 1 };
        p += 3;
      }
      let zones: number[] | null = null;
      if (flags & TF.zones) {
        const zn = v.getUint8(p++);
        zones = [];
        for (let i = 0; i < zn; i++) {
          zones.push(v.getUint16(p, true));
          p += 2;
        }
      }
      let text: string | null = null;
      if (flags & TF.text) {
        const len = v.getUint8(p++);
        text = decoder.decode(bytes.subarray(p, p + len));
        p += len;
      }
      tiles[idx] = { flags, house, ids, subs, iflags, creatures, spawn, spawnNpc, zones, text };
    }
    out.push({ key, tiles, count });
  }
  return out;
}

const BATCH = 96;

/**
 * Chunks of one map. get() returns what is loaded and queues the rest;
 * onload is called when requested chunks arrive.
 */
export class ChunkStore {
  private chunks = new Map<number, ChunkData | null>();
  private queued = new Set<number>();
  private inflight = new Set<number>();
  private gen = 0;
  private timer = 0;
  /** Request number of the newest request of each chunk: older answers are stale. */
  private latest = new Map<number, number>();
  private seq = 0;
  rev = 0;

  constructor(
    readonly mapId: number,
    private onload: () => void,
  ) {}

  /** The chunk, null when it is known to be empty, undefined when not loaded. */
  get(key: number): ChunkData | null | undefined {
    const c = this.chunks.get(key);
    if (c === undefined && !this.inflight.has(key)) this.queue(key);
    return c;
  }

  /** The loaded chunk without requesting it. */
  peek(key: number): ChunkData | null | undefined {
    return this.chunks.get(key);
  }

  tile(x: number, y: number, z: number): TileData | undefined {
    if (x < 0 || y < 0 || z < 0 || z > 15) return undefined;
    const c = this.get(keyOf(x, y, z));
    return c ? c.tiles[((y & 31) << 5) | (x & 31)] : undefined;
  }

  /** The tile when its chunk is loaded (no request). */
  peekTile(x: number, y: number, z: number): TileData | undefined {
    if (x < 0 || y < 0 || z < 0 || z > 15) return undefined;
    const c = this.chunks.get(keyOf(x, y, z));
    return c ? c.tiles[((y & 31) << 5) | (x & 31)] : undefined;
  }

  private queue(key: number): void {
    this.queued.add(key);
    if (!this.timer) this.timer = requestAnimationFrame(() => this.flush());
  }

  private flush(): void {
    this.timer = 0;
    const keys = [...this.queued];
    this.queued.clear();
    for (let i = 0; i < keys.length; i += BATCH) {
      const batch = keys.slice(i, i + BATCH);
      for (const k of batch) this.inflight.add(k);
      void this.fetch(batch, this.gen);
    }
  }

  private async fetch(keys: number[], gen: number): Promise<void> {
    const seq = ++this.seq;
    for (const k of keys) this.latest.set(k, seq);
    try {
      const r = await fetch(res.chunks(this.mapId, keys, this.rev) + `&q=${seq}`);
      if (!r.ok) throw new Error(await r.text());
      const data = decodeChunks(await r.arrayBuffer());
      if (gen !== this.gen) return;
      for (const c of data) {
        if (this.latest.get(c.key) === seq) this.chunks.set(c.key, c.count ? c : null);
      }
      this.onload();
    } catch {
      // The map was closed or the request failed: try again on demand.
    } finally {
      if (gen === this.gen) {
        for (const k of keys) {
          if (this.latest.get(k) === seq) {
            this.inflight.delete(k);
            this.latest.delete(k);
          }
        }
      }
    }
  }

  /** Drops chunks (all when keys is null) so they load again. */
  invalidate(keys: number[] | null, rev: number): void {
    this.rev = rev;
    if (keys === null) {
      this.gen++;
      this.chunks.clear();
      this.inflight.clear();
      this.queued.clear();
      return;
    }
    // Refetch changed chunks right away, keeping the old data until the
    // new one arrives so the map does not flicker.
    const fresh = keys.filter((k) => this.chunks.has(k) || this.inflight.has(k));
    if (fresh.length) {
      for (let i = 0; i < fresh.length; i += BATCH) {
        const batch = fresh.slice(i, i + BATCH);
        for (const k of batch) this.inflight.add(k);
        void this.fetch(batch, this.gen);
      }
    }
  }

  /** Forgets chunks far from a view to bound memory. */
  trim(keep: (key: number) => boolean, max = 3000): void {
    if (this.chunks.size <= max) return;
    for (const k of this.chunks.keys()) {
      if (!keep(k)) this.chunks.delete(k);
      if (this.chunks.size <= max * 0.75) break;
    }
  }
}
