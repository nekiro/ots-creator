// Guards the map renderer against unintended changes: every WebGL call of a
// series of frames goes to a log compared with a stored one. The scenes run
// on fake stores and caches whose answers depend only on the ids, so the log
// is the same on every run. A change meant to alter what is drawn updates the
// stored log (vitest -u) and the diff shows what changed.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Appearances, Look } from "../data/appearances";
import type { ChunkStore, CreatureData, TileData } from "../data/chunks";
import { QUAD_BYTES, type GeoChunk, type GeoStore } from "./geo";
import type { ItemIndex } from "../data/items";
import type { SpriteCache } from "../../render/sprites";
import { MapRenderer } from "./renderer";
import { fakeGl } from "./testing/fakeGl";
import type { PlayScene, Scene, ViewOptions } from "./scene";

/** A small deterministic hash. */
function mix(...values: number[]): number {
  let h = 2166136261;
  for (const v of values) {
    h ^= v | 0;
    h = Math.imul(h, 16777619);
    h ^= h >>> 13;
  }
  return h >>> 0;
}

const SPRITE = 32;

/** complete: every sprite is loaded. */
function fakeSprites(complete = false): SpriteCache {
  const cache = new Map<number, { pixels: Uint8ClampedArray }>();
  return {
    size: SPRITE,
    get(id: number) {
      if (id % 11 === 0 && !complete) return undefined;
      let e = cache.get(id);
      if (!e) {
        e = { pixels: new Uint8ClampedArray(SPRITE * SPRITE * 4).fill(id & 255) };
        cache.set(id, e);
      }
      return e;
    },
    load: () => Promise.resolve(),
  } as unknown as SpriteCache;
}

function look(id: number, outfit: boolean): Look {
  const f = id % 5 === 0 ? 3 : 1;
  // Items ending in 4 have the 4x2 pattern of stackables.
  const px = outfit || id % 10 === 4 ? 4 : 1 + (id % 3);
  const py = outfit ? 2 + (id % 2) : id % 10 === 4 ? 2 : 1 + (id % 2);
  const pz = outfit ? 2 : 1;
  const l = outfit && id % 2 ? 2 : 1;
  const w = 1 + (id % 2);
  const h = 1 + ((id >> 1) % 2);
  const s: number[] = [];
  for (let i = 0; i < f * pz * py * px * l * h * w; i++) s.push(mix(id, i) % 7 === 0 ? 0 : 100 + (mix(id, i) % 400));
  const cum = f > 1 ? [200, 400, 600] : null;
  return { id, f, px, py, pz, l, w, h, s, ox: id % 4, oy: id % 3, el: id % 6 === 0 ? 8 : 0, fl: id % 9 === 0 ? 2 : 0, cum, total: cum ? 600 : 0 } as unknown as Look;
}

/** complete: every appearance is loaded. */
function fakeLooks(complete = false): Appearances {
  const cache = (outfit: boolean) => ({
    get(id: number) {
      if (id % 17 === 0 && !complete) return undefined;
      if (id % 19 === 0) return null;
      return look(id, outfit);
    },
  });
  return { items: cache(false), outfits: cache(true), rev: 0 } as unknown as Appearances;
}

function fakeItems(): ItemIndex {
  const n = 600;
  const flags = new Uint32Array(n);
  const client = new Uint16Array(n);
  const top = new Uint8Array(n);
  for (let id = 0; id < n; id++) {
    // Items ending in 4 are stackable (and only those).
    flags[id] = id % 10 === 4 ? 8 : (mix(id) % 16) & ~8;
    client[id] = id;
    top[id] = id % 4;
  }
  return {
    maxId: n - 1,
    flags,
    client,
    top,
    bits: { meta: 1, splash: 2, fluidContainer: 4, stackable: 8, ground: 16 },
    has: (id: number) => id > 0 && id < n && id % 13 !== 0,
    is: (id: number, flag: string) => flag === "ground" && id < 100,
  } as unknown as ItemIndex;
}

function geoChunk(key: number, sig: string): GeoChunk {
  const n = 20 + (key % 30);
  const quads = new Uint8Array(n * QUAD_BYTES);
  const v = new DataView(quads.buffer);
  const sprites = new Set<number>();
  for (let i = 0; i < n; i++) {
    const sid = 100 + (mix(key, i) % 400);
    v.setInt16(i * QUAD_BYTES, (i % 32) * SPRITE, true);
    v.setInt16(i * QUAD_BYTES + 2, ((i >> 5) % 32) * SPRITE, true);
    v.setUint32(i * QUAD_BYTES + 4, sid, true);
    v.setUint32(i * QUAD_BYTES + 8, 0xffffffff, true);
    sprites.add(sid);
  }
  const anim = key % 3 === 0 ? new Uint32Array([1, 5, 0, 2, 10, 1, 3, 17, 0]) : new Uint32Array(0);
  const comp = key % 4 === 0 ? new Uint32Array([4, 120, 121, 0x01020304, 5, 130, 0, 0x05060708]) : new Uint32Array(0);
  return { key, quads, n, anim, comp, sprites: Uint32Array.from(sprites), lights: new DataView(new ArrayBuffer(0)), lightCount: 0, shades: null, sig };
}

/** complete: every chunk is built (none still loading). */
function fakeGeo(log: string[], complete = false): GeoStore {
  const chunks = new Map<string, GeoChunk | null | undefined>();
  const get = (key: number, sig: string) => {
    const k = `${key}|${sig}`;
    if (!chunks.has(k)) {
      const h = mix(key);
      chunks.set(k, h % 7 === 0 && !complete ? undefined : h % 11 === 0 ? null : geoChunk(key, sig));
    }
    return chunks.get(k);
  };
  let lastSig = "";
  return {
    get(key: number, sig: string) {
      // The order of requests decides what loads first.
      log.push(`geo.get(${key}, ${sig})`);
      lastSig = sig;
      return get(key, sig);
    },
    peek: (key: number) => {
      log.push(`geo.peek(${key})`);
      return get(key, lastSig);
    },
    idle: () => true,
    drop: (key: number) => log.push(`geo.drop(${key})`),
  } as unknown as GeoStore;
}

function creature(seed: number): CreatureData {
  return {
    npc: seed % 2 === 0,
    selected: seed % 3 === 0,
    dir: seed % 4,
    lookType: seed % 5 === 0 ? 0 : 128 + (seed % 20),
    head: seed % 133,
    body: (seed * 3) % 133,
    legs: (seed * 5) % 133,
    feet: (seed * 7) % 133,
    addons: seed % 4,
    mount: seed % 6 === 0 ? 300 : 0,
    lookItem: seed % 7 === 0 ? 200 : 0,
    name: `creature ${seed}`,
  };
}

function fakeChunks(log: string[]): ChunkStore {
  const tiles = new Map<string, TileData | undefined>();
  return {
    get: (key: number) => {
      log.push(`chunks.get(${key})`);
      return undefined;
    },
    peekTile(x: number, y: number, z: number) {
      const k = `${x},${y},${z}`;
      if (!tiles.has(k)) {
        const h = mix(x, y, z);
        if (h % 5 !== 0) tiles.set(k, undefined);
        else {
          const count = 1 + (h % 4);
          const ids = Uint16Array.from({ length: count }, (_, i) => 1 + (mix(h, i) % 520));
          tiles.set(k, {
            flags: (h % 3 === 0 ? 1 : 0) | (h % 7 === 0 ? 256 : 0) | (h % 9 === 0 ? 512 : 0),
            house: 0,
            ids,
            subs: Uint16Array.from({ length: count }, (_, i) => mix(h, i, 1) % 60),
            iflags: Uint8Array.from({ length: count }, (_, i) => mix(h, i, 2) % 64),
            creatures: h % 4 === 0 ? [creature(h % 1000)] : null,
            spawn: h % 7 === 0 ? { radius: 1 + (h % 3), selected: h % 2 === 0 } : null,
            spawnNpc: h % 9 === 0 ? { radius: 2, selected: false } : null,
            zones: null,
            text: h % 6 === 0 ? `A sign at ${x}, ${y} with a rather long text that wraps over a few lines` : null,
          });
        }
      }
      return tiles.get(k);
    },
  } as unknown as ChunkStore;
}

const ALL: ViewOptions = {
  grid: true,
  lowerFloors: true,
  ghostHigher: true,
  shade: true,
  creatures: true,
  labels: true,
  spawns: true,
  houses: true,
  special: true,
  zones: true,
  pathing: true,
  animation: true,
  onlyModified: false,
  highlightItems: true,
  wallHooks: true,
  markers: true,
  tooltips: true,
  coords: true,
  ghostItems: true,
  clientBox: true,
  lights: false,
  ambientLight: 50,
};

/** What the play view draws with: no editor overlays. */
const PLAYING: ViewOptions = { ...ALL, grid: false, ghostHigher: false, labels: false, spawns: false, houses: false, special: false, zones: false, pathing: false, highlightItems: false, wallHooks: false, markers: false, tooltips: false, coords: false, ghostItems: false, clientBox: false, lights: true };

function playScene(change: Partial<PlayScene>): PlayScene {
  return { player: creature(4), x: 1000, y: 1000, z: 7, offsetX: 0, offsetY: 0, footPhase: 0, firstFloor: 5, lastFloor: 7, worldLight: 250, ...change };
}

describe("MapRenderer", () => {
  let now = 0;
  beforeEach(() => {
    now = 0;
    // Each call moves the clock 1 ms: uploads past the frame budget wait.
    vi.spyOn(performance, "now").mockImplementation(() => (now += 1));
    vi.stubGlobal("devicePixelRatio", 1);
  });
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("makes the same WebGL calls as before", async () => {
    const log: string[] = [];
    const fake = fakeGl((call) => log.push(fake.format(call)));
    const canvas = { getContext: () => fake.gl } as unknown as HTMLCanvasElement;
    const renderer = new MapRenderer(canvas);
    // Setup in any order: only what is created counts.
    const setup = [...log].sort();
    log.length = 0;
    const base: Scene = {
      width: 800,
      height: 600,
      spriteSize: SPRITE,
      camX: 1000 * SPRITE,
      camY: 1000 * SPRITE,
      scale: 1,
      floor: 7,
      chunks: fakeChunks(log),
      geo: fakeGeo(log),
      items: fakeItems(),
      looks: fakeLooks(),
      sprites: fakeSprites(),
      opts: ALL,
      time: 0,
      house: 3,
      ghosts: [
        { x: 1001, y: 1001, z: 7, ids: [5, 50, 150, 260], subs: [1, 2, 30, 4] },
        { x: 1003, y: 1002, z: 7, ids: [40], creatures: [creature(3), creature(10), creature(2)] },
        { x: 1003, y: 1002, z: 6, ids: [41] },
        { x: 999, y: 1003, z: 7, ids: [14, 24, 44, 54, 64, 74, 84], subs: [0, 3, 7, 24, 25, 49, 60] },
      ],
      ghostAlpha: 0.5,
      rects: [
        { x0: 998, y0: 998, x1: 1002, y1: 1001, z: 7, fill: [1, 0, 0, 0.2], stroke: [1, 1, 0, 1] },
        { x0: 990, y0: 990, x1: 991, y1: 991, z: 6, fill: [0, 1, 0, 0.2] },
      ],
      markers: [
        { x: 1000, y: 999, z: 7, label: "Temple", full: "Temple of Thais", color: [0.2, 0.8, 0.4] },
        { x: 1004, y: 999, z: 7, label: "Depot", color: [0.9, 0.1, 0.1] },
      ],
      moveOffset: null,
      hover: { x: 1000, y: 999, z: 7 },
    };
    // A tile with a text in view, for the hover over a sign.
    let sign = { x: 0, y: 0, z: 7 };
    for (let i = 0; i < 400 && !sign.x; i++) {
      const x = 995 + (i % 20);
      const y = 995 + Math.floor(i / 20);
      if (base.chunks.peekTile(x, y, 7)?.text) sign = { x, y, z: 7 };
    }
    expect(sign.x).toBeGreaterThan(0);
    // Everything loaded but the chunks (or with them): only chunk loading keeps the frame pending.
    const loaded = { geo: fakeGeo(log, true), sprites: fakeSprites(true), looks: fakeLooks(true) };
    const loading = { sprites: loaded.sprites, looks: loaded.looks };
    const frames: [string, Partial<Scene>][] = [
      ["near, all options", {}],
      ["near, next frame", { time: 250 }],
      ["dragging a selection", { time: 500, moveOffset: { dx: 2, dy: -1, dz: 0 }, hover: { x: 1003, y: 1003, z: 7 } }],
      ["hover on a sign", { hover: sign }],
      ["all built, uploads over budget", { ...loaded, camX: 1500 * SPRITE, scale: 0.25 }],
      ["all built, uploads go on", { ...loaded, camX: 1500 * SPRITE, scale: 0.25 }],
      ["all built, uploads go on 2", { ...loaded, camX: 1500 * SPRITE, scale: 0.25 }],
      ["all built, uploads go on 3", { ...loaded, camX: 1500 * SPRITE, scale: 0.25 }],
      ["all built, uploads go on 4", { ...loaded, camX: 1500 * SPRITE, scale: 0.25 }],
      ["all built, uploads go on 5", { ...loaded, camX: 1500 * SPRITE, scale: 0.25 }],
      ["chunks loading", { ...loading }],
      ["chunks loading 2", { ...loading }],
      ["chunks loading 3", { ...loading }],
      ["chunks loading, zoomed out", { ...loading, scale: 0.1 }],
      ["chunks loading, zoomed out 2", { ...loading, scale: 0.1 }],
      ["underground", { floor: 9, camX: 1200 * SPRITE }],
      ["upper floor", { floor: 4, opts: { ...ALL, lowerFloors: false, shade: false } }],
      ["few options", { opts: { ...ALL, grid: false, labels: false, ghostHigher: false, animation: false, markers: false, coords: false, tooltips: false, clientBox: false, spawns: false } }],
      ["zoomed out, blocks", { scale: 0.1 }],
      ["zoomed out, blocks again", { scale: 0.1, time: 700 }],
      ["far, overlays only", { scale: 0.02 }],
      ["hi-dpi", { scale: 2.5, width: 1600, height: 1000 }],
      ["back near", { time: 900 }],
      ["new sprite size", { spriteSize: 64, sprites: fakeSprites() }],
      ["lights", { ...loaded, opts: { ...ALL, lights: true, ambientLight: 20 } }],
      ["play view", { ...loaded, opts: PLAYING, ghosts: [], rects: [], markers: [], hover: null, play: playScene({}) }],
      ["play view, walking at night", { ...loaded, opts: PLAYING, ghosts: [], rects: [], markers: [], hover: null, play: playScene({ offsetX: -12, footPhase: 3, worldLight: 40 }) }],
      ["play view, under ground", { ...loaded, floor: 9, opts: PLAYING, ghosts: [], rects: [], markers: [], hover: null, play: playScene({ z: 9, firstFloor: 8, lastFloor: 11 }) }],
    ];
    const out: string[] = ["== setup", ...setup];
    for (const [name, change] of frames) {
      if (name === "back near") renderer.invalidateSprites([120, 130, 200]);
      log.length = 0;
      const result = renderer.render({ ...base, ...change });
      out.push(`== ${name}`, JSON.stringify(result), ...log);
    }
    log.length = 0;
    renderer.dispose();
    out.push("== dispose", ...log);
    await expect(out.join("\n") + "\n").toMatchFileSnapshot("./__snapshots__/renderer.gl.txt");
  });
});
