// Map tiles fetched from /res/map/{id}/chunks in 32x32 chunks per floor.
// Only chunks a change touched are fetched again.
import { res } from "./api";
import { MAX_FLOOR } from "../constants";

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
  itemFlags: Uint8Array;
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

export function chunkKey(chunkX: number, chunkY: number, z: number): number {
  return chunkX * 32768 + chunkY * 16 + z;
}

/** The chunk a tile is in (one coordinate). */
export function chunkOfTile(tile: number): number {
  return tile >> 5;
}

export function keyOf(x: number, y: number, z: number): number {
  return chunkKey(chunkOfTile(x), chunkOfTile(y), z);
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
export function decodeChunks(buffer: ArrayBuffer): ChunkData[] {
  const view = new DataView(buffer);
  const bytes = new Uint8Array(buffer);
  let offset = 0;
  const out: ChunkData[] = [];
  // The item arrays of all tiles are views into three shared buffers: an
  // array buffer per tile costs several times its few bytes. An item takes
  // at least 5 bytes of the response, which bounds the item count.
  const room = Math.floor(buffer.byteLength / 5) + 1;
  const allIds = new Uint16Array(room);
  const allSubs = new Uint16Array(room);
  const allFlags = new Uint8Array(room);
  let itemCursor = 0;
  while (offset + 6 <= buffer.byteLength) {
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
      const itemFlags = itemCount ? allFlags.subarray(itemCursor, itemCursor + itemCount) : NO_FLAGS;
      itemCursor += itemCount;
      for (let i = 0; i < itemCount; i++) {
        ids[i] = view.getUint16(offset, true);
        subs[i] = view.getUint16(offset + 2, true);
        itemFlags[i] = view.getUint8(offset + 4);
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
          const nameLength = view.getUint8(offset + 12);
          offset += 13;
          creature.name = decoder.decode(bytes.subarray(offset, offset + nameLength));
          offset += nameLength;
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
        const textLength = view.getUint16(offset, true);
        offset += 2;
        text = decoder.decode(bytes.subarray(offset, offset + textLength));
        offset += textLength;
      }
      tiles[index] = { flags, house, ids, subs, itemFlags, creatures, spawn, spawnNpc, zones, text };
    }
    out.push({ key, tiles, count });
  }
  return out;
}

/** Chunks per request. */
const BATCH = 96;

/** Forgets entries of a chunk store far from a view to bound memory: once above max, down to three quarters of it. */
export function trimStore(store: Map<number, unknown>, keep: (key: number) => boolean, max: number): void {
  if (store.size <= max) return;
  for (const key of store.keys()) {
    if (!keep(key)) store.delete(key);
    if (store.size <= max * 0.75) break;
  }
}

/** Calls request with the keys split into batches of at most size keys. */
export function inBatches(keys: number[], size: number, request: (batch: number[]) => void): void {
  for (let i = 0; i < keys.length; i += size) request(keys.slice(i, i + size));
}

/**
 * Chunks of one map. get() returns what is loaded and queues the rest;
 * onload is called when requested chunks arrive.
 */
export class ChunkStore {
  private chunks = new Map<number, ChunkData | null>();
  private queued = new Set<number>();
  private inflight = new Set<number>();
  /** Bumped when every chunk is dropped: answers to older requests are stale. */
  private generation = 0;
  private timer = 0;
  /** Request number of the newest request of each chunk: older answers are stale. */
  private latest = new Map<number, number>();
  private requestNumber = 0;
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

  /** The tile, its chunk requested when not loaded. */
  tile(x: number, y: number, z: number): TileData | undefined {
    return this.tileIn(x, y, z, true);
  }

  /** The tile when its chunk is loaded (no request). */
  peekTile(x: number, y: number, z: number): TileData | undefined {
    return this.tileIn(x, y, z, false);
  }

  /** request: fetch the chunk when not loaded. */
  private tileIn(x: number, y: number, z: number, request: boolean): TileData | undefined {
    if (x < 0 || y < 0 || z < 0 || z > MAX_FLOOR) return undefined;
    const key = keyOf(x, y, z);
    const chunk = request ? this.get(key) : this.chunks.get(key);
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
    this.request(keys);
  }

  /** Fetches chunks in batches, marking them in flight. */
  private request(keys: number[]): void {
    inBatches(keys, BATCH, (batch) => {
      for (const key of batch) this.inflight.add(key);
      void this.fetch(batch, this.generation);
    });
  }

  private async fetch(keys: number[], generation: number): Promise<void> {
    const requestNumber = ++this.requestNumber;
    for (const key of keys) this.latest.set(key, requestNumber);
    try {
      const response = await fetch(res.chunks(this.mapId, keys, this.rev) + `&q=${requestNumber}`);
      if (!response.ok) throw new Error(await response.text());
      const data = decodeChunks(await response.arrayBuffer());
      if (generation !== this.generation) return;
      for (const chunk of data) {
        if (this.latest.get(chunk.key) === requestNumber) this.chunks.set(chunk.key, chunk.count ? chunk : null);
      }
      this.onload();
    } catch {
      // The map was closed or the request failed: try again on demand.
    } finally {
      if (generation === this.generation) {
        for (const key of keys) {
          if (this.latest.get(key) === requestNumber) {
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
      this.generation++;
      this.chunks.clear();
      this.inflight.clear();
      this.queued.clear();
      return;
    }
    // Refetch changed chunks right away, keeping the old data until the
    // new one arrives so the map does not flicker.
    const fresh = keys.filter((key) => this.chunks.has(key) || this.inflight.has(key));
    this.request(fresh);
  }

  /** Forgets chunks far from a view to bound memory. */
  trim(keep: (key: number) => boolean, max = 3000): void {
    trimStore(this.chunks, keep, max);
  }
}
