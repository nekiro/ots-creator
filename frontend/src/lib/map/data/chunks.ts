// Map tiles fetched from /res/map/{id}/chunks in 32x32 chunks per floor.
// Only chunks a change touched are fetched again.
import { res } from "./api";

export const CHUNK = 32;

/** Tile flags of the chunk format (see app/map_http.go). */
export const TileFlags = {
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
export const ItemFlags = { selected: 1, ids: 2, text: 4, teleport: 8, contents: 16, doorId: 32 } as const;

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
  /** Items bottom to top; the ground first when flags has TileFlags.ground. */
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

export function splitKey(key: number): [number, number, number] {
  return [Math.floor(key / 32768), Math.floor(key / 16) & 0x7ff, key & 15];
}

/** Index of tile (x, y) in its chunk's tiles. */
export function tileIndex(x: number, y: number): number {
  return ((y & 31) << 5) | (x & 31);
}

const decoder = new TextDecoder();

const NO_IDS = new Uint16Array(0);
const NO_FLAGS = new Uint8Array(0);

/** Decodes a chunk response into chunks. */
export function decodeChunks(buf: ArrayBuffer): ChunkData[] {
  const view = new DataView(buf);
  const bytes = new Uint8Array(buf);
  let offset = 0;
  const out: ChunkData[] = [];
  // The item arrays of all tiles are views into three shared buffers: an
  // array buffer per tile costs several times its few bytes. An item takes
  // at least 5 bytes of the response, which bounds the item count.
  const room = Math.floor(buf.byteLength / 5) + 1;
  const allIds = new Uint16Array(room);
  const allSubs = new Uint16Array(room);
  const allFlags = new Uint8Array(room);
  let itemCursor = 0;
  while (offset + 6 <= buf.byteLength) {
    const key = view.getUint32(offset, true);
    const count = view.getUint16(offset + 4, true);
    offset += 6;
    const tiles: (TileData | undefined)[] = new Array(CHUNK * CHUNK);
    for (let decoded = 0; decoded < count; decoded++) {
      const index = view.getUint16(offset, true);
      const flags = view.getUint16(offset + 2, true);
      offset += 4;
      let house = 0;
      if (flags & TileFlags.house) {
        house = view.getUint32(offset, true);
        offset += 4;
      }
      const itemCount = view.getUint8(offset++);
      const ids = itemCount ? allIds.subarray(itemCursor, itemCursor + itemCount) : NO_IDS;
      const subs = itemCount ? allSubs.subarray(itemCursor, itemCursor + itemCount) : NO_IDS;
      const iflags = itemCount ? allFlags.subarray(itemCursor, itemCursor + itemCount) : NO_FLAGS;
      itemCursor += itemCount;
      for (let i = 0; i < itemCount; i++) {
        ids[i] = view.getUint16(offset, true);
        subs[i] = view.getUint16(offset + 2, true);
        iflags[i] = view.getUint8(offset + 4);
        offset += 5;
      }
      let creatures: CreatureData[] | null = null;
      if (flags & TileFlags.creatures) {
        const creatureCount = view.getUint8(offset++);
        creatures = [];
        for (let i = 0; i < creatureCount; i++) {
          const creatureFlags = view.getUint8(offset);
          const creature: CreatureData = {
            npc: (creatureFlags & 1) !== 0,
            selected: (creatureFlags & 2) !== 0,
            dir: (creatureFlags >> 2) & 3,
            lookType: view.getUint16(offset + 1, true),
            head: view.getUint8(offset + 3),
            body: view.getUint8(offset + 4),
            legs: view.getUint8(offset + 5),
            feet: view.getUint8(offset + 6),
            addons: view.getUint8(offset + 7),
            mount: view.getUint16(offset + 8, true),
            lookItem: view.getUint16(offset + 10, true),
            name: "",
          };
          const len = view.getUint8(offset + 12);
          offset += 13;
          creature.name = decoder.decode(bytes.subarray(offset, offset + len));
          offset += len;
          creatures.push(creature);
        }
      }
      let spawn: TileData["spawn"] = null;
      let spawnNpc: TileData["spawnNpc"] = null;
      if (flags & TileFlags.spawn) {
        spawn = { radius: view.getUint16(offset, true), selected: view.getUint8(offset + 2) === 1 };
        offset += 3;
      }
      if (flags & TileFlags.spawnNpc) {
        spawnNpc = { radius: view.getUint16(offset, true), selected: view.getUint8(offset + 2) === 1 };
        offset += 3;
      }
      let zones: number[] | null = null;
      if (flags & TileFlags.zones) {
        const zoneCount = view.getUint8(offset++);
        zones = [];
        for (let i = 0; i < zoneCount; i++) {
          zones.push(view.getUint16(offset, true));
          offset += 2;
        }
      }
      let text: string | null = null;
      if (flags & TileFlags.text) {
        const len = view.getUint16(offset, true);
        offset += 2;
        text = decoder.decode(bytes.subarray(offset, offset + len));
        offset += len;
      }
      tiles[index] = { flags, house, ids, subs, iflags, creatures, spawn, spawnNpc, zones, text };
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
    const chunk = this.chunks.get(key);
    if (chunk === undefined && !this.inflight.has(key)) this.queue(key);
    return chunk;
  }

  /** The loaded chunk without requesting it. */
  peek(key: number): ChunkData | null | undefined {
    return this.chunks.get(key);
  }

  tile(x: number, y: number, z: number): TileData | undefined {
    if (x < 0 || y < 0 || z < 0 || z > 15) return undefined;
    const chunk = this.get(keyOf(x, y, z));
    return chunk ? chunk.tiles[tileIndex(x, y)] : undefined;
  }

  /** The tile when its chunk is loaded (no request). */
  peekTile(x: number, y: number, z: number): TileData | undefined {
    if (x < 0 || y < 0 || z < 0 || z > 15) return undefined;
    const chunk = this.chunks.get(keyOf(x, y, z));
    return chunk ? chunk.tiles[tileIndex(x, y)] : undefined;
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
      for (const key of batch) this.inflight.add(key);
      void this.fetch(batch, this.gen);
    }
  }

  private async fetch(keys: number[], gen: number): Promise<void> {
    const seq = ++this.seq;
    for (const key of keys) this.latest.set(key, seq);
    try {
      const response = await fetch(res.chunks(this.mapId, keys, this.rev) + `&q=${seq}`);
      if (!response.ok) throw new Error(await response.text());
      const data = decodeChunks(await response.arrayBuffer());
      if (gen !== this.gen) return;
      for (const chunk of data) {
        if (this.latest.get(chunk.key) === seq) this.chunks.set(chunk.key, chunk.count ? chunk : null);
      }
      this.onload();
    } catch {
      // The map was closed or the request failed: try again on demand.
    } finally {
      if (gen === this.gen) {
        for (const key of keys) {
          if (this.latest.get(key) === seq) {
            this.inflight.delete(key);
            this.latest.delete(key);
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
    const fresh = keys.filter((key) => this.chunks.has(key) || this.inflight.has(key));
    if (fresh.length) {
      for (let i = 0; i < fresh.length; i += BATCH) {
        const batch = fresh.slice(i, i + BATCH);
        for (const key of batch) this.inflight.add(key);
        void this.fetch(batch, this.gen);
      }
    }
  }

  /** Forgets chunks far from a view to bound memory. */
  trim(keep: (key: number) => boolean, max = 3000): void {
    if (this.chunks.size <= max) return;
    for (const key of this.chunks.keys()) {
      if (!keep(key)) this.chunks.delete(key);
      if (this.chunks.size <= max * 0.75) break;
    }
  }
}
