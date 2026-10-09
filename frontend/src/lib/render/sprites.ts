// Client-side sprite cache. Sprites are fetched in batches as raw RGBA from
// /res/sprites. A change only drops the sprites it touched.

import { res } from "../api";

export interface SpriteEntry {
  pixels: Uint8ClampedArray;
}

const BATCH = 512;

export class SpriteCache {
  /** url builds the batch request: res.sprites for the open client. */
  constructor(private url: (ids: number[], rev: number) => string = res.sprites) {}
  size = 32;
  // Bumped on every invalidation; batches fetched under an older generation
  // are discarded because they may hold stale pixels.
  private gen = 0;
  private entries = new Map<number, SpriteEntry>();
  private inflight = new Map<number, Promise<void>>();

  /** Drops the given sprites, or everything when ids is null. */
  invalidate(ids: number[] | null): void {
    this.gen++;
    this.inflight.clear();
    if (ids === null) this.entries.clear();
    else for (const id of ids) this.entries.delete(id);
  }

  get(id: number): SpriteEntry | undefined {
    return this.entries.get(id);
  }

  /** Loads missing sprites; id 0 is always empty and never fetched. */
  async load(ids: Iterable<number>): Promise<void> {
    const missing: number[] = [];
    const waits: Promise<void>[] = [];
    for (const id of new Set(ids)) {
      if (id === 0 || this.entries.has(id)) continue;
      const p = this.inflight.get(id);
      if (p) waits.push(p);
      else missing.push(id);
    }
    for (let i = 0; i < missing.length; i += BATCH) {
      const chunk = missing.slice(i, i + BATCH);
      const p = this.fetchChunk(chunk, this.gen);
      for (const id of chunk) this.inflight.set(id, p);
      waits.push(p);
    }
    await Promise.all(waits);
  }

  private async fetchChunk(ids: number[], gen: number): Promise<void> {
    try {
      const resp = await fetch(this.url(ids, gen));
      if (!resp.ok) throw new Error(await resp.text());
      const size = Number(resp.headers.get("X-Sprite-Size")) || 32;
      this.size = size;
      const buf = new Uint8ClampedArray(await resp.arrayBuffer());
      if (gen !== this.gen) return;
      const bytes = size * size * 4;
      // Views into one buffer: no per-sprite copies.
      ids.forEach((id, i) => this.entries.set(id, { pixels: buf.subarray(i * bytes, (i + 1) * bytes) }));
    } finally {
      if (gen === this.gen) for (const id of ids) this.inflight.delete(id);
    }
  }
}

export const spriteCache = new SpriteCache();
/** Sprites of the second client of the compare window. */
export const otherSpriteCache = new SpriteCache(res.otherSprites);
