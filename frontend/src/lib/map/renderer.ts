// WebGL2 map renderer. Tiles are drawn as instanced quads (one per sprite
// piece or colored rectangle). The backend builds the quads of each 32x32
// chunk (geo.ts, app/map_geo.go) with sprite ids; they go to a GPU buffer
// as they come and are redrawn with a camera offset. The shader finds the
// atlas cell of a sprite in a cell index texture, so atlas changes never
// rebuild chunks; only animated pieces are patched per frame. Previews
// and overlays are built every frame in screen pixels. Sprites live in a
// texture array atlas.
import { colorize } from "../render/outfit";
import type { SpriteCache } from "../render/sprites";
import { AF, frameAt, type Appearances, type Look } from "./appearances";
import { Atlas } from "./atlas";
import { CHUNK, IF, TF, chunkKey, splitKey, type ChunkStore, type CreatureData, type TileData } from "./chunks";
import { GEO, GEO_BLOCK, GEO_COMPOSED, GEO_HIDDEN, QUAD_BYTES, geoBits, type GeoChunk, type GeoStore } from "./geo";
import type { ItemIndex } from "./items";

export interface ViewOptions {
  grid: boolean;
  lowerFloors: boolean;
  ghostHigher: boolean;
  shade: boolean;
  creatures: boolean;
  /** Names over creatures, houses, temples and waypoints. */
  labels: boolean;
  spawns: boolean;
  houses: boolean;
  special: boolean;
  zones: boolean;
  pathing: boolean;
  animation: boolean;
  onlyModified: boolean;
  highlightItems: boolean;
  wallHooks: boolean;
  markers: boolean;
  tooltips: boolean;
  /** x, y, z written on every tile. */
  coords: boolean;
  ghostItems: boolean;
  clientBox: boolean;
  lights: boolean;
}

export interface GhostTile {
  x: number;
  y: number;
  z: number;
  /** Server ids, bottom to top. */
  ids: number[];
  subs?: number[];
  creatures?: CreatureData[];
}

export interface RectMark {
  /** Tile rectangle on a floor (inclusive). */
  x0: number;
  y0: number;
  x1: number;
  y1: number;
  z: number;
  fill?: [number, number, number, number];
  stroke?: [number, number, number, number];
}

export interface Marker {
  x: number;
  y: number;
  z: number;
  label: string;
  color: [number, number, number];
}

export interface Label {
  x: number;
  y: number;
  text: string;
  color: string;
  /** Small font, centered on y (coordinate rulers). */
  small?: boolean;
  align?: "center" | "left";
  /** Left out where it would cover another label (item texts). */
  cull?: boolean;
}

/** Start of an item text short enough for a map label: its first line,
 * cut at a word. */
export function labelText(text: string, max = 18): string {
  const line = text.trim().split(/\r?\n/, 1)[0].trim();
  if (line.length <= max) return line;
  const cut = line.slice(0, max);
  const space = cut.lastIndexOf(" ");
  return (space > max / 2 ? cut.slice(0, space) : cut).replace(/[\s,.;:\-–]+$/, "") + "…";
}

export interface Scene {
  width: number;
  height: number;
  /** World pixels per tile (the sprite size). */
  S: number;
  /** Camera center in world pixels (ground floor space). */
  camX: number;
  camY: number;
  /** Device pixels per world pixel. */
  scale: number;
  floor: number;
  /** Tiles, for overlays and previews (only the floor in view is loaded). */
  chunks: ChunkStore;
  /** Chunk quads. */
  geo: GeoStore;
  items: ItemIndex;
  looks: Appearances;
  sprites: SpriteCache;
  opts: ViewOptions;
  time: number;
  /** House highlighted by the house brush (0: none). */
  house: number;
  ghosts: GhostTile[];
  ghostAlpha: number;
  rects: RectMark[];
  markers: Marker[];
  /** Selected things drawn again at this offset (dragging a selection). */
  moveOffset: { dx: number; dy: number; dz: number } | null;
  /** Tile under the mouse (labelled with its position when coords is on). */
  hover: { x: number; y: number; z: number } | null;
}

export interface FrameResult {
  /** Animated things are visible: draw again soon. */
  animated: boolean;
  /** Data is still loading: draw again when it arrives. */
  pending: boolean;
  labels: Label[];
  /** Zoomed out past drawing sprites: only overlays show. */
  far: boolean;
  /** Chunks are still being built: draw again on the next frame. */
  more: boolean;
}

// Quads are in "a_rect" units; a pixel is round((p + u_off) * u_scale + u_half).
// World chunks pass the camera there, screen-space quads pass (0, 1, 0).
// A negative layer hides the quad (an animation frame without a sprite).
const VERT = `#version 300 es
layout(location=0) in vec2 a_corner;
layout(location=1) in vec4 a_rect;
layout(location=2) in vec2 a_cell;
layout(location=3) in vec4 a_color;
uniform vec2 u_screen;
uniform vec2 u_off;
uniform float u_scale;
uniform vec2 u_half;
uniform float u_alpha;
uniform float u_perRow;
uniform float u_uvCell;
out vec3 v_uv;
out vec4 v_color;
void main() {
  if (a_cell.x < 0.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    v_uv = vec3(0.0);
    v_color = vec4(0.0);
    return;
  }
  vec2 w = a_rect.xy + a_corner * a_rect.zw;
  vec2 p = floor((w + u_off) * u_scale + u_half + 0.5);
  gl_Position = vec4(p.x / u_screen.x * 2.0 - 1.0, 1.0 - p.y / u_screen.y * 2.0, 0.0, 1.0);
  float layer = a_cell.x;
  float col = mod(a_cell.y, u_perRow);
  float row = floor(a_cell.y / u_perRow);
  v_uv = vec3((vec2(col, row) + a_corner) * u_uvCell, layer);
  v_color = vec4(a_color.rgb, a_color.a * u_alpha);
}`;

// Chunk quads (QUAD_BYTES each, see geo.ts): the sprite id is looked up in
// the cell index (composed outfit pieces in their own index).
const VERT_CHUNK = `#version 300 es
layout(location=0) in vec2 a_corner;
layout(location=1) in vec2 a_pos;
layout(location=2) in uint a_sprite;
layout(location=3) in vec4 a_color;
uniform vec2 u_screen;
uniform vec2 u_off;
uniform float u_S;
uniform float u_scale;
uniform vec2 u_half;
uniform float u_alpha;
uniform uint u_perRow;
uniform uint u_perLayer;
uniform float u_uvCell;
uniform highp usampler2D u_ind;
uniform highp usampler2D u_indC;
out vec3 v_uv;
out vec4 v_color;
const uint MISSING = 0xFFFFFFFFu;
uint lookup(highp usampler2D t, uint i) {
  ivec2 size = textureSize(t, 0);
  uint w = uint(size.x);
  if (i >= w * uint(size.y)) return MISSING;
  return texelFetch(t, ivec2(int(i % w), int(i / w)), 0).r;
}
void main() {
  uint cell = MISSING;
  vec2 size = vec2(u_S);
  if (a_sprite != MISSING) {
    if ((a_sprite & 0x80000000u) != 0u) cell = lookup(u_indC, a_sprite & 0x7FFFFFFFu);
    else if ((a_sprite & 0x40000000u) != 0u) {
      // A colored rectangle: the white cell, its size in the value.
      cell = 0u;
      size = vec2(float((a_sprite >> 15) & 0x7FFFu), float(a_sprite & 0x7FFFu));
    } else cell = lookup(u_ind, a_sprite);
  }
  if (cell == MISSING) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    v_uv = vec3(0.0);
    v_color = vec4(0.0);
    return;
  }
  vec2 w = a_pos + a_corner * size;
  vec2 p = floor((w + u_off) * u_scale + u_half + 0.5);
  gl_Position = vec4(p.x / u_screen.x * 2.0 - 1.0, 1.0 - p.y / u_screen.y * 2.0, 0.0, 1.0);
  uint c = cell % u_perLayer;
  v_uv = vec3((vec2(float(c % u_perRow), float(c / u_perRow)) + a_corner) * u_uvCell, float(cell / u_perLayer));
  v_color = vec4(a_color.rgb, a_color.a * u_alpha);
}`;

const FRAG = `#version 300 es
precision mediump float;
precision mediump sampler2DArray;
uniform sampler2DArray u_tex;
in vec3 v_uv;
in vec4 v_color;
out vec4 outColor;
void main() {
  vec4 t = texture(u_tex, v_uv);
  if (t.a == 0.0) discard;
  outColor = t * v_color;
}`;

const FLOATS = 10;
/** Bytes of chunk quads uploaded per frame; the rest wait for the next frames. */
const UPLOAD_BYTES = 32 << 20;
/** Time spent uploading chunk quads per frame (ms); the rest wait for the next frames. */
const UPLOAD_MS = 6;
/** Chunks a side of the blocks drawn far out. */
const BLOCK = 4;
/** Chunk buffers kept before the least recently drawn are freed. */
const MAX_GEOS = 12000;
/** Tiles drawn over all floors before the farthest lower floors are left out. */
const MAX_FLOOR_TILES = 3_000_000;
/** Tiles of one floor in view above which sprites are not drawn. */
const MAX_VISIBLE_TILES = 6_000_000;

type DrawItem = (id: number, sub: number, x: number, y: number, z: number, ax: number, ay: number, r: number, g: number, b: number, a: number, hook: number) => number;
type DrawCreature = (c: CreatureData, ax: number, ay: number, x: number, y: number, z: number, alpha: number) => void;

/** The quads of one chunk on the GPU. */
interface ChunkGeo {
  vao: WebGLVertexArrayObject;
  buf: WebGLBuffer;
  /** The geometry uploaded; a new one from the store uploads again. */
  src: GeoChunk;
  /** Atlas cells the quads use, kept alive in the atlas while drawn. */
  cells: Int32Array;
  /** The cells are valid for this atlas and epoch, and every sprite was in it. */
  atlas: Atlas | null;
  epoch: number;
  ready: boolean;
  used: number;
  /** The quads, kept on the CPU to patch animated pieces. */
  data: Uint8Array | null;
}

const MISSING = 0xffffffff;
const NO_QUADS = new Uint8Array(0);

/** Atlas cells of ids in a R32UI texture the chunk shader reads (MISSING: not in the atlas). */
class CellIndex {
  private data: Uint32Array;
  private tex: WebGLTexture;
  private rows: number;
  private lo = Infinity;
  private hi = -1;
  private grown = true;
  /** Nothing else binds its texture unit: once bound it stays. */
  private bound = false;

  constructor(
    private gl: WebGL2RenderingContext,
    readonly unit: number,
    private width: number,
    rows: number,
  ) {
    this.rows = rows;
    this.data = new Uint32Array(width * rows).fill(MISSING);
    this.tex = gl.createTexture()!;
  }

  get(id: number): number {
    return id < this.data.length ? this.data[id] : MISSING;
  }

  set(id: number, cell: number): void {
    if (id >= this.data.length) {
      let rows = this.rows;
      while (id >= this.width * rows) rows *= 2;
      const d = new Uint32Array(this.width * rows).fill(MISSING);
      d.set(this.data);
      this.data = d;
      this.rows = rows;
      this.grown = true;
    }
    if (this.data[id] === cell) return;
    this.data[id] = cell;
    const row = Math.floor(id / this.width);
    this.lo = Math.min(this.lo, row);
    this.hi = Math.max(this.hi, row);
  }

  clear(): void {
    this.data.fill(MISSING);
    this.grown = true;
  }

  /** Uploads changes and binds the texture to its unit (leaves unit 0 active). */
  bind(): void {
    if (this.bound && !this.grown && this.hi < 0) return;
    this.bound = true;
    const gl = this.gl;
    gl.activeTexture(gl.TEXTURE0 + this.unit);
    gl.bindTexture(gl.TEXTURE_2D, this.tex);
    if (this.grown) {
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.R32UI, this.width, this.rows, 0, gl.RED_INTEGER, gl.UNSIGNED_INT, this.data);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
      this.grown = false;
    } else if (this.hi >= 0) {
      const rows = this.data.subarray(this.lo * this.width, (this.hi + 1) * this.width);
      gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, this.lo, this.width, this.hi - this.lo + 1, gl.RED_INTEGER, gl.UNSIGNED_INT, rows);
    }
    this.lo = Infinity;
    this.hi = -1;
    gl.activeTexture(gl.TEXTURE0);
  }

  dispose(): void {
    this.gl.deleteTexture(this.tex);
  }
}

function compile(gl: WebGL2RenderingContext, vs: string, fs: string): WebGLProgram {
  const p = gl.createProgram()!;
  for (const [type, src] of [
    [gl.VERTEX_SHADER, vs],
    [gl.FRAGMENT_SHADER, fs],
  ] as const) {
    const sh = gl.createShader(type)!;
    gl.shaderSource(sh, src);
    gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(sh) ?? "shader error");
    gl.attachShader(p, sh);
  }
  gl.linkProgram(p);
  if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(p) ?? "link error");
  return p;
}

export class MapRenderer {
  readonly gl: WebGL2RenderingContext;
  private prog: WebGLProgram;
  private progChunk: WebGLProgram;
  private vao: WebGLVertexArrayObject;
  private inst: WebGLBuffer;
  private quad: WebGLBuffer;
  private data = new Float32Array(FLOATS * 4096);
  private n = 0;
  atlas: Atlas | null = null;
  private u: Record<string, WebGLUniformLocation | null> = {};
  private uc: Record<string, WebGLUniformLocation | null> = {};
  private missing = new Set<number>();
  private composed = new Map<string, Uint8ClampedArray | null>();
  private geos = new Map<number, ChunkGeo>();
  /** Atlas cells of sprite ids, and of composed outfit pieces by their index in composedIds. */
  private ind: CellIndex;
  private indC: CellIndex;
  private composedIds = new Map<string, number>();
  private composedKeys: string[] = [];
  private frameNo = 0;
  /** Chunk drawing state is bound (with this alpha); anything else drawn clears it. */
  private chunkAlpha = -1;
  private boundTex: WebGLTexture | null = null;
  private uploaded = 0;
  private uploadEnd = 0;

  constructor(readonly canvas: HTMLCanvasElement) {
    const gl = canvas.getContext("webgl2", { alpha: false, antialias: false, premultipliedAlpha: false, preserveDrawingBuffer: true });
    if (!gl) throw new Error("WebGL 2 is not available");
    this.gl = gl;
    this.prog = compile(gl, VERT, FRAG);
    this.progChunk = compile(gl, VERT_CHUNK, FRAG);
    for (const k of ["u_screen", "u_off", "u_S", "u_scale", "u_half", "u_alpha", "u_perRow", "u_perLayer", "u_uvCell", "u_tex", "u_ind", "u_indC"]) this.uc[k] = gl.getUniformLocation(this.progChunk, k);
    this.ind = new CellIndex(gl, 2, 2048, 64);
    this.indC = new CellIndex(gl, 3, 1024, 4);
    for (const k of ["u_screen", "u_off", "u_scale", "u_half", "u_alpha", "u_perRow", "u_uvCell", "u_tex"]) this.u[k] = gl.getUniformLocation(this.prog, k);
    const quad = gl.createBuffer()!;
    this.quad = quad;
    gl.bindBuffer(gl.ARRAY_BUFFER, quad);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
    this.inst = gl.createBuffer()!;
    this.vao = this.instanceVao(this.inst);
  }

  /** A vertex array reading quads from an instance buffer (compact: the chunk format). */
  private instanceVao(buf: WebGLBuffer, compact = false): WebGLVertexArrayObject {
    const gl = this.gl;
    const vao = gl.createVertexArray()!;
    gl.bindVertexArray(vao);
    gl.bindBuffer(gl.ARRAY_BUFFER, this.quad);
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.enableVertexAttribArray(1);
    gl.enableVertexAttribArray(2);
    gl.enableVertexAttribArray(3);
    if (compact) {
      gl.vertexAttribPointer(1, 2, gl.SHORT, false, QUAD_BYTES, 0);
      gl.vertexAttribIPointer(2, 1, gl.UNSIGNED_INT, QUAD_BYTES, 4);
      gl.vertexAttribPointer(3, 4, gl.UNSIGNED_BYTE, true, QUAD_BYTES, 8);
    } else {
      const stride = FLOATS * 4;
      gl.vertexAttribPointer(1, 4, gl.FLOAT, false, stride, 0);
      gl.vertexAttribPointer(2, 2, gl.FLOAT, false, stride, 16);
      gl.vertexAttribPointer(3, 4, gl.FLOAT, false, stride, 24);
    }
    gl.vertexAttribDivisor(1, 1);
    gl.vertexAttribDivisor(2, 1);
    gl.vertexAttribDivisor(3, 1);
    gl.bindVertexArray(null);
    return vao;
  }

  /** Sprites changed in the client: drop them from the atlas (the cell index follows). */
  invalidateSprites(ids: number[] | null): void {
    this.atlas?.invalidate(ids);
    this.composed.clear();
  }

  private freeGeo(g: ChunkGeo): void {
    this.gl.deleteVertexArray(g.vao);
    this.gl.deleteBuffer(g.buf);
  }

  dispose(): void {
    for (const g of this.geos.values()) this.freeGeo(g);
    this.geos.clear();
    this.atlas?.dispose();
    this.ind.dispose();
    this.indC.dispose();
    const ext = this.gl.getExtension("WEBGL_lose_context");
    ext?.loseContext();
  }

  private push(x: number, y: number, w: number, h: number, cell: number, r: number, g: number, b: number, a: number): void {
    if ((this.n + 1) * FLOATS > this.data.length) {
      const d = new Float32Array(this.data.length * 2);
      d.set(this.data);
      this.data = d;
    }
    const o = this.n * FLOATS;
    const atlas = this.atlas!;
    const d = this.data;
    d[o] = x;
    d[o + 1] = y;
    d[o + 2] = w;
    d[o + 3] = h;
    if (cell < 0) {
      d[o + 4] = -1;
      d[o + 5] = -1;
    } else {
      d[o + 4] = Math.floor(cell / atlas.perLayer);
      d[o + 5] = cell % atlas.perLayer;
    }
    d[o + 6] = r;
    d[o + 7] = g;
    d[o + 8] = b;
    d[o + 9] = a;
    this.n++;
  }

  /** A colored rectangle (screen pixels, or world pixels while building a chunk). */
  private rect(x: number, y: number, w: number, h: number, c: readonly number[]): void {
    this.push(x, y, w, h, 0, c[0], c[1], c[2], c[3]);
  }

  render(sc: Scene): FrameResult {
    const gl = this.gl;
    const result: FrameResult = { animated: false, pending: false, labels: [], far: false, more: false };
    if (!this.atlas || this.atlas.cell !== sc.S) {
      this.atlas?.dispose();
      this.atlas = new Atlas(gl, sc.S);
      this.ind.clear();
      this.indC.clear();
      this.atlas.onRelease = (key) => {
        if (typeof key === "number") this.ind.set(key, MISSING);
        else {
          const i = this.composedIds.get(key);
          if (i !== undefined) this.indC.set(i, MISSING);
        }
      };
    }
    const atlas = this.atlas;
    atlas.begin();
    this.frameNo++;
    this.chunkAlpha = -1;
    this.uploaded = 0;
    this.uploadEnd = performance.now() + UPLOAD_MS;
    this.n = 0;
    this.missing.clear();
    gl.viewport(0, 0, sc.width, sc.height);
    gl.clearColor(0, 0, 0, 1);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

    const { S, scale, floor } = sc;
    const T = S * scale; // device pixels per tile
    const halfW = sc.width / 2;
    const halfH = sc.height / 2;
    const adj = (z: number) => (floor <= 7 ? 7 - z : floor - z);
    const toX = (wx: number) => Math.round((wx - sc.camX) * scale + halfW);
    const toY = (wy: number) => Math.round((wy - sc.camY) * scale + halfH);
    const tileX = (x: number, z: number) => toX((x - adj(z)) * S);
    const tileY = (y: number, z: number) => toY((y - adj(z)) * S);

    let zs: number[] = [];
    if (floor <= 7) for (let z = sc.opts.lowerFloors ? 7 : floor; z >= floor; z--) zs.push(z);
    else for (let z = sc.opts.lowerFloors ? Math.min(15, floor + 2) : floor; z >= floor; z--) zs.push(z);
    const visibleTiles = ((sc.width / T + 6) * (sc.height / T + 6)) | 0;
    if (zs.length * visibleTiles > MAX_FLOOR_TILES) zs = [floor, ...zs.filter((z) => z !== floor)].slice(0, Math.max(1, Math.floor(MAX_FLOOR_TILES / visibleTiles))).sort((a, b) => b - a);
    const far = T < MapRenderer.NEAR_T || visibleTiles > MAX_VISIBLE_TILES;
    result.far = far;

    // What chunk quads are built for besides the chunk itself.
    // Far out, movable items and creatures are too small to see and
    // animation (all the water) would patch thousands of chunks per frame.
    // Far out, chunks also come in blocks of BLOCK x BLOCK: fewer draws.
    const farDetail = T < MapRenderer.DETAIL_T;
    const block = farDetail ? BLOCK : 1;
    let bits = geoBits(sc.opts);
    if (farDetail) bits = (bits & ~(GEO.animation | GEO.creatures | GEO.ghostItems)) | GEO.far;
    const sigBase = `${bits}|${sc.opts.houses ? sc.house : 0}|`;
    const sig = `${sigBase}${block}`;

    if (far) {
      // Overlays only.
      this.overlays(sc, result, tileX, tileY, T);
      this.flushScreen(sc);
      return result;
    }

    const { drawItem, drawCreature } = this.drawers(sc, scale, T, result);
    // Chunks of the current floor still loading or not uploaded yet.
    let notReady = 0;
    // Blocks loaded before, drawn under chunks still loading (right after zooming in).
    const under = new Map<number, GeoChunk[]>();
    const chunksOf = (z: number, request = true): GeoChunk[] => {
      const a = adj(z);
      const tx0 = Math.floor((sc.camX - halfW / scale) / S + a) - 1;
      const ty0 = Math.floor((sc.camY - halfH / scale) / S + a) - 1;
      // Sprites reach up and left of their tile: take the chunks right and below too.
      const tx1 = Math.ceil((sc.camX + halfW / scale) / S + a) + 3;
      const ty1 = Math.ceil((sc.camY + halfH / scale) / S + a) + 3;
      const out: GeoChunk[] = [];
      if (tx1 < 0 || ty1 < 0) return out;
      if (!request) {
        // Floors next to the view, loaded while idle so changing floors finds them.
        for (let cy = Math.max(0, ty0) >> 5; cy <= ty1 >> 5; cy += block) {
          for (let cx = Math.max(0, tx0) >> 5; cx <= tx1 >> 5; cx += block) {
            const k = chunkKey(cx - (cx % block), cy - (cy % block), z) + (block > 1 ? GEO_BLOCK : 0);
            const c = sc.geo.get(k, sig);
            if (c) out.push(c);
          }
        }
        return out;
      }
      const align = (c: number) => c - (c % block);
      // Row by row: a sprite only overlaps tiles up and left of it, which
      // this order has already drawn, as a tile by tile walk would.
      for (let cy = align(Math.max(0, ty0) >> 5); cy <= ty1 >> 5; cy += block) {
        for (let cx = align(Math.max(0, tx0) >> 5); cx <= tx1 >> 5; cx += block) {
          const k = chunkKey(cx, cy, z);
          if (block === 1) {
            // Tiles of the floor in view feed the overlays and previews.
            if (z === floor && (T >= MapRenderer.TILES_T || sc.moveOffset)) sc.chunks.get(k);
            const c = sc.geo.get(k, sig);
            if (c) out.push(c);
            else if (c === undefined) {
              const bk = chunkKey(cx - (cx % BLOCK), cy - (cy % BLOCK), z) + GEO_BLOCK;
              const b = sc.geo.peek(bk);
              if (b) {
                let list = under.get(z);
                if (!list) under.set(z, (list = []));
                if (!list.includes(b)) list.push(b);
              }
              if (z === floor) notReady++;
            }
            continue;
          }
          const c = sc.geo.get(k + GEO_BLOCK, sig);
          if (c) out.push(c);
          else if (c === undefined) {
            // Until the block arrives, its chunks loaded before show.
            for (let y = cy; y < cy + block; y++) {
              for (let x = cx; x < cx + block; x++) {
                const p = sc.geo.peek(chunkKey(x, y, z));
                if (p) out.push(p);
              }
            }
            if (z === floor) notReady++;
          }
        }
      }
      return out;
    };
    // The floor in view requests first.
    const lists = new Map<number, GeoChunk[]>();
    lists.set(floor, chunksOf(floor));
    for (const z of zs) if (z !== floor) lists.set(z, chunksOf(z));
    const ghostZ = sc.opts.ghostHigher && floor > 0 && floor !== 8 ? floor - 1 : -1;
    if (ghostZ >= 0) lists.set(ghostZ, chunksOf(ghostZ));
    // The current floor uploads first.
    for (const c of lists.get(floor) ?? []) if (!this.geoOf(c)) notReady++;
    // What is in view has upload time first; the floors above and below take what is left.
    for (const z of zs) for (const c of lists.get(z)!) this.geoOf(c);
    // Only while nothing else loads: those requests would hold up what comes into view.
    if (!notReady && sc.geo.idle()) {
      for (const z of [floor - 1, floor + 1]) {
        if (z < 0 || z > 15 || lists.has(z)) continue;
        for (const c of chunksOf(z, false)) {
          const g = this.geoOf(c, true);
          if (g) g.used = this.frameNo;
        }
      }
    }

    // Draw floors from the bottom up.
    for (const z of zs) {
      const isCur = z === floor;
      if (isCur && zs.length > 1 && sc.opts.shade) {
        this.rect(0, 0, sc.width, sc.height, [0, 0, 0, 0.45]);
        this.flushScreen(sc);
      }
      for (const c of under.get(z) ?? []) this.drawGeo(sc, c, 1, adj(z), result);
      for (const c of lists.get(z)!) this.drawGeo(sc, c, 1, adj(z), result);
      if (isCur && sc.moveOffset) {
        const a = adj(z);
        const tx0 = Math.floor((sc.camX - halfW / scale) / S + a) - 1;
        const ty0 = Math.floor((sc.camY - halfH / scale) / S + a) - 1;
        const tx1 = Math.ceil((sc.camX + halfW / scale) / S + a) + 3;
        const ty1 = Math.ceil((sc.camY + halfH / scale) / S + a) + 3;
        this.drawMoved(sc, z, tx0, ty0, tx1, ty1, tileX, tileY, T, drawItem, drawCreature);
        this.flushScreen(sc);
      }
    }
    // Ghost of the floor above.
    if (ghostZ >= 0) for (const c of lists.get(ghostZ)!) this.drawGeo(sc, c, 0.3, adj(ghostZ), result);
    // Brush preview and paste ghost.
    for (const g of sc.ghosts) {
      if (g.z !== floor) continue;
      const ax = tileX(g.x, g.z);
      const ay = tileY(g.y, g.z);
      let elev = 0;
      // Client stacking: ground, borders, bottom, common, top items.
      const rank = (id: number) => (sc.items.is(id, "ground") ? 0 : ([3, 1, 2, 4][sc.items.top[id]] ?? 3));
      const order = g.ids.map((_, k) => k).sort((a, b) => rank(g.ids[a]) - rank(g.ids[b]) || a - b);
      for (const i of order) {
        elev += drawItem(g.ids[i], g.subs?.[i] ?? 1, g.x, g.y, g.z, ax - elev * scale, ay - elev * scale, 1, 1, 1, sc.ghostAlpha, 0);
      }
      for (const c of g.creatures ?? []) drawCreature(c, ax - elev * scale, ay - elev * scale, g.x, g.y, g.z, sc.ghostAlpha);
    }
    this.overlays(sc, result, tileX, tileY, T);
    if (this.missing.size) {
      result.pending = true;
      void sc.sprites.load([...this.missing]);
    }
    if (atlas.pending) result.pending = true;
    if (notReady) result.pending = true;
    this.flushScreen(sc);
    this.sweep(sc);
    return result;
  }

  /** Smallest tile size in device pixels that still draws sprites. */
  static readonly NEAR_T = 1;
  /** Smallest tile size in device pixels that draws every item, creatures and animation. */
  static readonly DETAIL_T = 4;
  /** Smallest tile size in device pixels that loads tile data for overlays. */
  static readonly TILES_T = 8;

  /** The chunk's GPU buffer, uploaded when the store has a new version and the frame has upload budget left. */
  private geoOf(chunk: GeoChunk, spare = false): ChunkGeo | null {
    let g = this.geos.get(chunk.key) ?? null;
    if (g && g.src === chunk) return g;
    // Something always uploads, unless only spare time is asked for.
    if ((this.uploaded > 0 || spare) && (this.uploaded + chunk.quads.byteLength > UPLOAD_BYTES || performance.now() > this.uploadEnd)) return g;
    this.uploaded += chunk.quads.byteLength;
    const gl = this.gl;
    if (!g) {
      const buf = gl.createBuffer()!;
      g = { vao: this.instanceVao(buf, true), buf, src: chunk, cells: new Int32Array(0), atlas: null, epoch: 0, ready: false, used: 0, data: null };
      this.geos.set(chunk.key, g);
    }
    // Composed outfit pieces point at their entry in the composed index.
    const q = chunk.quads;
    if (chunk.comp.length) {
      const v = new DataView(q.buffer, q.byteOffset, q.byteLength);
      const c = chunk.comp;
      for (let i = 0; i < c.length; i += 4) {
        const key = composedKey(c[i + 1], c[i + 2], c[i + 3]);
        let id = this.composedIds.get(key);
        if (id === undefined) {
          id = this.composedKeys.length;
          this.composedIds.set(key, id);
          this.composedKeys.push(key);
        }
        v.setUint32(c[i] * QUAD_BYTES + 4, (GEO_COMPOSED | id) >>> 0, true);
      }
    }
    gl.bindBuffer(gl.ARRAY_BUFFER, g.buf);
    gl.bufferData(gl.ARRAY_BUFFER, q, chunk.anim.length ? gl.DYNAMIC_DRAW : gl.STATIC_DRAW);
    g.src = chunk;
    g.atlas = null;
    g.data = chunk.anim.length ? q : null;
    // Only animated chunks patch their quads: the others need no CPU copy.
    if (!chunk.anim.length) chunk.quads = NO_QUADS;
    return g;
  }

  /** Puts every sprite of a chunk in the atlas and the cell index. */
  private ensure(sc: Scene, g: ChunkGeo, result: FrameResult): void {
    const atlas = this.atlas!;
    if (g.atlas === atlas && g.epoch === atlas.epoch && g.ready) {
      atlas.touch(g.cells);
      return;
    }
    const cells: number[] = [];
    let ready = true;
    for (const sid of g.src.sprites) {
      const c = this.cellOfSprite(sc, sid);
      if (c < 0) ready = false;
      else cells.push(c);
    }
    const comp = g.src.comp;
    for (let i = 0; i < comp.length; i += 4) {
      const key = composedKey(comp[i + 1], comp[i + 2], comp[i + 3]);
      const id = this.composedIds.get(key)!;
      let c = this.indC.get(id);
      if (c === MISSING) {
        const sid = comp[i + 1];
        const tpl = comp[i + 2];
        const k = comp[i + 3];
        const colors = { head: k & 255, body: (k >> 8) & 255, legs: (k >> 16) & 255, feet: k >>> 24 };
        c = atlas.cellOf(key, () => this.compose(sc.sprites, sid, tpl, colors));
        if (c < 0) {
          ready = false;
          if (!sc.sprites.get(sid)) this.missing.add(sid);
          if (tpl && !sc.sprites.get(tpl)) this.missing.add(tpl);
          continue;
        }
        this.indC.set(id, c);
      }
      cells.push(c);
    }
    g.cells = Int32Array.from(cells);
    atlas.touch(g.cells);
    g.atlas = atlas;
    g.epoch = atlas.epoch;
    g.ready = ready;
    if (!ready) result.pending = true;
  }

  /** The atlas cell of a sprite, uploading it when needed (-1: not loaded yet). */
  private cellOfSprite(sc: Scene, sid: number): number {
    const c = this.ind.get(sid);
    if (c !== MISSING) return c;
    const cell = this.atlas!.cellOf(sid, () => sc.sprites.get(sid)?.pixels);
    if (cell < 0) {
      if (!sc.sprites.get(sid)) this.missing.add(sid);
      return -1;
    }
    this.ind.set(sid, cell);
    return cell;
  }

  private drawGeo(sc: Scene, chunk: GeoChunk, alpha: number, a: number, result: FrameResult): void {
    const g = this.geoOf(chunk);
    if (!g) {
      result.more = true;
      return;
    }
    if (g.src !== chunk) result.more = true;
    g.used = this.frameNo;
    this.ensure(sc, g, result);
    if (g.data && sc.opts.animation) {
      result.animated = true;
      this.animate(sc, g, result);
    }
    const gl = this.gl;
    const [cx, cy] = splitKey(g.src.key % GEO_BLOCK);
    this.bindChunks(sc, alpha);
    gl.uniform2f(this.uc.u_off, (cx * CHUNK - a) * sc.S - sc.camX, (cy * CHUNK - a) * sc.S - sc.camY);
    gl.bindVertexArray(g.vao);
    gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, g.src.n);
    gl.bindVertexArray(null);
  }

  /** Points animated pieces of a chunk at the sprites of their current frame. */
  private animate(sc: Scene, g: ChunkGeo, result: FrameResult): void {
    const d = g.data!;
    const v = new DataView(d.buffer, d.byteOffset, d.byteLength);
    const anim = g.src.anim;
    let dirty = false;
    let lastId = -1;
    let look: Look | null | undefined = null;
    let frameOff = 0;
    for (let k = 0; k < anim.length; k += 3) {
      if (anim[k + 1] !== lastId) {
        lastId = anim[k + 1];
        look = sc.looks.items.get(lastId);
        if (look === undefined) result.pending = true;
        if (look) frameOff = frameAt(look, sc.time) * look.pz * look.py * look.px * look.l * look.h * look.w;
      }
      if (!look) continue;
      const sid = look.s[anim[k + 2] + frameOff] ?? 0;
      let val = GEO_HIDDEN;
      if (sid) {
        const c = this.cellOfSprite(sc, sid);
        if (c < 0) continue;
        this.atlas!.use(c);
        val = sid;
      }
      const o = anim[k] * QUAD_BYTES + 4;
      if (v.getUint32(o, true) !== val) {
        v.setUint32(o, val, true);
        dirty = true;
      }
    }
    if (dirty) {
      const gl = this.gl;
      gl.bindBuffer(gl.ARRAY_BUFFER, g.buf);
      gl.bufferSubData(gl.ARRAY_BUFFER, 0, d);
    }
  }

  /** Frees chunk buffers not drawn for a while once there are many (the store fetches them again). */
  private sweep(sc: Scene): void {
    if (this.geos.size <= MAX_GEOS) return;
    const old = this.frameNo - 60;
    for (const [k, g] of this.geos) {
      if (g.used < old) {
        this.freeGeo(g);
        this.geos.delete(k);
        sc.geo.drop(k);
      }
    }
  }

  /** Item and creature drawing at a scale: screen pixels (scale, T) or world pixels (1, S). */
  private drawers(sc: Scene, scale: number, T: number, st: { pending: boolean; animated: boolean }): { drawItem: DrawItem; drawCreature: DrawCreature } {
    const items = sc.items;
    const bits = items.bits;
    const metaBit = bits.meta ?? 0;
    const fluidBits = (bits.splash ?? 0) | (bits.fluidContainer ?? 0);
    const stackBit = bits.stackable ?? 0;
    const time = sc.opts.animation ? sc.time : 0;

    const drawItem: DrawItem = (id, sub, x, y, z, ax, ay, cr, cg, cb, ca, hook) => {
      if (!items.has(id)) {
        this.rect(ax, ay, T, T, [0.8, 0, 0, 0.6 * ca]);
        return 0;
      }
      const fl = items.flags[id];
      if (fl & metaBit) return 0;
      const look = sc.looks.items.get(items.client[id]);
      if (look === undefined) {
        st.pending = true;
        return 0;
      }
      if (look === null) {
        this.rect(ax, ay, T, T, [0.8, 0, 0, 0.5 * ca]);
        return 0;
      }
      let px = 0;
      let py = 0;
      if (fl & fluidBits) {
        const c = sub % 8;
        px = (c % 4) % look.px;
        py = Math.floor(c / 4) % look.py;
      } else if (look.fl & AF.hangable && hook) {
        px = Math.min(hook, look.px - 1);
      } else if (fl & stackBit && look.px === 4 && look.py === 2) {
        const n = sub;
        if (n <= 0) [px, py] = [0, 0];
        else if (n < 5) [px, py] = [n - 1, 0];
        else if (n < 10) [px, py] = [0, 1];
        else if (n < 25) [px, py] = [1, 1];
        else if (n < 50) [px, py] = [2, 1];
        else [px, py] = [3, 1];
      } else {
        px = x % look.px;
        py = y % look.py;
      }
      const pz = z % look.pz;
      let frame = 0;
      if (look.f > 1) {
        st.animated = sc.opts.animation || st.animated;
        frame = frameAt(look, time);
      }
      this.drawLook(sc, look, px, py, pz, frame, ax - look.ox * scale, ay - look.oy * scale, T, cr, cg, cb, ca, null);
      return look.el;
    };

    const drawCreature: DrawCreature = (c, ax, ay, x, y, z, alpha) => {
      const dark = c.selected ? 0.5 : 1;
      if (c.lookItem) {
        drawItem(c.lookItem, 0, x, y, z, ax, ay, dark, dark, dark, alpha, 0);
        return;
      }
      if (!c.lookType) {
        // Unknown creature: a marker so it is not invisible.
        this.rect(ax + T * 0.25, ay + T * 0.25, T * 0.5, T * 0.5, c.npc ? [0.3, 0.6, 1, 0.8 * alpha] : [1, 0.5, 0.2, 0.8 * alpha]);
        return;
      }
      const look = sc.looks.outfits.get(c.lookType);
      if (look === undefined) {
        st.pending = true;
        return;
      }
      if (!look) return;
      const dir = Math.min(c.dir, look.px - 1);
      let mounted = 0;
      if (c.mount && look.pz > 1) {
        const mount = sc.looks.outfits.get(c.mount);
        if (mount === undefined) st.pending = true;
        else if (mount) {
          this.drawLook(sc, mount, Math.min(dir, mount.px - 1), 0, 0, 0, ax - mount.ox * scale, ay - mount.oy * scale, T, dark, dark, dark, alpha, null);
          mounted = 1;
        }
      }
      const colors = look.l > 1 ? c : null;
      this.drawLook(sc, look, dir, 0, mounted, 0, ax - look.ox * scale, ay - look.oy * scale, T, dark, dark, dark, alpha, colors);
      for (const [bit, yy] of [
        [1, 1],
        [2, 2],
      ]) {
        if (c.addons & bit && look.py > yy) {
          this.drawLook(sc, look, dir, yy, mounted, 0, ax - look.ox * scale, ay - look.oy * scale, T, dark, dark, dark, alpha, colors);
        }
      }
    };
    return { drawItem, drawCreature };
  }

  /** Draws the selected things of the current floor at the drag offset. */
  private drawMoved(
    sc: Scene,
    z: number,
    tx0: number,
    ty0: number,
    tx1: number,
    ty1: number,
    tileX: (x: number, z: number) => number,
    tileY: (y: number, z: number) => number,
    T: number,
    drawItem: DrawItem,
    drawCreature: DrawCreature,
  ): void {
    const off = sc.moveOffset!;
    for (let ty = Math.max(0, ty0 - off.dy); ty <= ty1 - off.dy; ty++) {
      for (let tx = Math.max(0, tx0 - off.dx); tx <= tx1 - off.dx; tx++) {
        const t = sc.chunks.peekTile(tx, ty, z);
        if (!t || !(t.flags & TF.selected)) continue;
        const nx = tx + off.dx;
        const ny = ty + off.dy;
        const ax = tileX(nx, z);
        const ay = tileY(ny, z);
        let elev = 0;
        for (let k = 0; k < t.ids.length; k++) {
          if (!(t.iflags[k] & IF.selected)) continue;
          elev += drawItem(t.ids[k], t.subs[k], nx, ny, z, ax - elev * sc.scale, ay - elev * sc.scale, 1, 1, 1, 0.6, 0);
        }
        for (const c of t.creatures ?? []) if (c.selected) drawCreature(c, ax, ay, nx, ny, z, 0.6);
        this.rect(ax, ay, T, T, [0.4, 0.7, 1, 0.15]);
      }
    }
  }

  private drawLook(
    sc: Scene,
    look: Look,
    px: number,
    py: number,
    pz: number,
    frame: number,
    ax: number,
    ay: number,
    T: number,
    r: number,
    g: number,
    b: number,
    a: number,
    colors: CreatureData | null = null,
  ): void {
    const atlas = this.atlas!;
    const sprites = sc.sprites;
    const layers = colors ? 1 : look.l;
    const fr = frame % Math.max(look.f, 1);
    for (let l = 0; l < layers; l++) {
      for (let cy = 0; cy < look.h; cy++) {
        for (let cx = 0; cx < look.w; cx++) {
          const idx = (((((fr * look.pz + pz) * look.py + py) * look.px + px) * look.l + l) * look.h + cy) * look.w + cx;
          const sid = look.s[idx];
          if (!sid) continue;
          let cell: number;
          if (colors) {
            const tidx = idx + look.h * look.w; // layer 1 of the same texture
            const tpl = look.s[tidx] ?? 0;
            const key = composedKey(sid, tpl, colors.head | (colors.body << 8) | (colors.legs << 16) | (colors.feet << 24));
            cell = atlas.cellOf(key, () => this.compose(sc.sprites, sid, tpl, colors));
            if (cell < 0) {
              if (!sprites.get(sid)) this.missing.add(sid);
              if (tpl && !sprites.get(tpl)) this.missing.add(tpl);
              continue;
            }
          } else {
            cell = atlas.cellOf(sid, () => sprites.get(sid)?.pixels);
            if (cell < 0) {
              if (!sprites.get(sid)) this.missing.add(sid);
              continue;
            }
          }
          this.push(ax - cx * T, ay - cy * T, T, T, cell, r, g, b, a);
        }
      }
    }
  }

  private compose(sprites: SpriteCache, sid: number, tpl: number, c: Pick<CreatureData, "head" | "body" | "legs" | "feet">): Uint8ClampedArray | undefined {
    const base = sprites.get(sid)?.pixels;
    if (!base) return undefined;
    if (!tpl) return base;
    const t = sprites.get(tpl)?.pixels;
    if (!t) return undefined;
    const out = new Uint8ClampedArray(base);
    colorize(out, t, { head: c.head, body: c.body, legs: c.legs, feet: c.feet });
    return out;
  }

  private overlays(sc: Scene, result: FrameResult, tileX: (x: number, z: number) => number, tileY: (y: number, z: number) => number, T: number): void {
    const z = sc.floor;
    const line = Math.max(1, Math.round(sc.scale));
    const stroke = (x: number, y: number, w: number, h: number, c: readonly number[], lw = line) => {
      this.rect(x, y, w, lw, c);
      this.rect(x, y + h - lw, w, lw, c);
      this.rect(x, y, lw, h, c);
      this.rect(x + w - lw, y, lw, h, c);
    };
    if (sc.opts.grid && T >= 8) {
      const halfW = sc.width / 2;
      const halfH = sc.height / 2;
      const a = sc.floor <= 7 ? 7 - z : 0;
      const tx0 = Math.floor((sc.camX - halfW / sc.scale) / sc.S + a);
      const ty0 = Math.floor((sc.camY - halfH / sc.scale) / sc.S + a);
      const tx1 = Math.ceil((sc.camX + halfW / sc.scale) / sc.S + a);
      const ty1 = Math.ceil((sc.camY + halfH / sc.scale) / sc.S + a);
      const c = [1, 1, 1, 0.12];
      for (let x = tx0; x <= tx1; x++) this.rect(tileX(x, z), 0, 1, sc.height, c);
      for (let y = ty0; y <= ty1; y++) this.rect(0, tileY(y, z), sc.width, 1, c);
    }
    // Spawn radius outlines of the current floor.
    if (sc.opts.spawns && T >= MapRenderer.TILES_T) {
      const halfW = sc.width / 2 / sc.scale / sc.S;
      const halfH = sc.height / 2 / sc.scale / sc.S;
      const a = sc.floor <= 7 ? 7 - z : 0;
      const cx = sc.camX / sc.S + a;
      const cy = sc.camY / sc.S + a;
      const R = 30;
      for (let ty = Math.max(0, Math.floor(cy - halfH - R)); ty <= cy + halfH + R; ty++) {
        for (let tx = Math.max(0, Math.floor(cx - halfW - R)); tx <= cx + halfW + R; tx++) {
          const t = sc.chunks.peekTile(tx, ty, z);
          if (!t || !(t.flags & (TF.spawn | TF.spawnNpc))) continue;
          for (const s of [t.spawn, t.spawnNpc]) {
            if (!s) continue;
            const r = s.radius;
            const c = s.selected ? [1, 1, 0.4, 0.9] : t.spawnNpc === s ? [0.4, 0.7, 1, 0.7] : [1, 0.45, 0.45, 0.7];
            stroke(tileX(tx - r, z), tileY(ty - r, z), (2 * r + 1) * T, (2 * r + 1) * T, c);
            this.rect(tileX(tx, z), tileY(ty, z), T, T, [c[0], c[1], c[2], 0.25]);
          }
        }
      }
    }
    for (const m of sc.rects) {
      if (m.z !== z) continue;
      const x = tileX(Math.min(m.x0, m.x1), z);
      const y = tileY(Math.min(m.y0, m.y1), z);
      const w = (Math.abs(m.x1 - m.x0) + 1) * T;
      const h = (Math.abs(m.y1 - m.y0) + 1) * T;
      if (m.fill) this.rect(x, y, w, h, m.fill);
      if (m.stroke) stroke(x, y, w, h, m.stroke);
    }
    if (sc.opts.markers) {
      for (const mk of sc.markers) {
        if (mk.z !== z) continue;
        const x = tileX(mk.x, z);
        const y = tileY(mk.y, z);
        const c = [mk.color[0], mk.color[1], mk.color[2], 0.9];
        stroke(x + line, y + line, T - 2 * line, T - 2 * line, c, Math.max(2, line * 2));
        if (T >= 12 && sc.opts.labels) {
          result.labels.push({
            x: x + T / 2,
            y: y - 2,
            text: mk.label,
            color: `rgb(${Math.round(mk.color[0] * 255)},${Math.round(mk.color[1] * 255)},${Math.round(mk.color[2] * 255)})`,
          });
        }
      }
    }
    if (sc.opts.clientBox) {
      // The client shows 15x11 tiles around the player (plus one hidden row).
      const a = sc.floor <= 7 ? 7 - z : 0;
      const cx = Math.floor(sc.camX / sc.S + a);
      const cy = Math.floor(sc.camY / sc.S + a);
      stroke(tileX(cx - 7, z), tileY(cy - 5, z), 15 * T, 11 * T, [1, 1, 1, 0.6], Math.max(2, line));
    }
    if (sc.opts.creatures && sc.opts.labels && T >= 24 && !result.far) {
      // Names over creatures as the client shows them: green (full health),
      // centered over the sprite, which is drawn 8 px up and left of its tile.
      const halfW = sc.width / 2 / sc.scale / sc.S;
      const halfH = sc.height / 2 / sc.scale / sc.S;
      const a = sc.floor <= 7 ? 7 - z : 0;
      const cx = sc.camX / sc.S + a;
      const cy = sc.camY / sc.S + a;
      const d = T / 4;
      for (let ty = Math.max(0, Math.floor(cy - halfH)); ty <= cy + halfH + 1; ty++) {
        for (let tx = Math.max(0, Math.floor(cx - halfW)); tx <= cx + halfW + 1; tx++) {
          const t = sc.chunks.peekTile(tx, ty, z);
          if (!t?.creatures) continue;
          for (const c of t.creatures) result.labels.push({ x: tileX(tx, z) + T / 2 - d, y: tileY(ty, z) - d - 2, text: c.name, color: "#00c000" });
        }
      }
    }
    if (sc.opts.labels && T >= 24 && !result.far) {
      // Text of signs, books and other readable items: its first line over
      // the tile.
      const halfW = sc.width / 2 / sc.scale / sc.S;
      const halfH = sc.height / 2 / sc.scale / sc.S;
      const a = sc.floor <= 7 ? 7 - z : 0;
      const cx = sc.camX / sc.S + a;
      const cy = sc.camY / sc.S + a;
      for (let ty = Math.max(0, Math.floor(cy - halfH)); ty <= cy + halfH + 1; ty++) {
        for (let tx = Math.max(0, Math.floor(cx - halfW)); tx <= cx + halfW + 1; tx++) {
          const text = sc.chunks.peekTile(tx, ty, z)?.text;
          if (!text) continue;
          const line = labelText(text);
          if (line) result.labels.push({ x: tileX(tx, z) + T / 2, y: tileY(ty, z) + T / 2, text: line, color: "#e0d29a", small: true, cull: true });
        }
      }
    }
    if (sc.opts.coords && sc.hover && sc.hover.z === z) {
      // The position of the tile under the mouse, in a dark tag above it.
      const h = sc.hover;
      const s = Math.max(1, Math.round(globalThis.devicePixelRatio || 1));
      const text = `${h.x}, ${h.y}, ${h.z}`;
      const w = (text.length * 5 + 8) * s;
      const ht = 13 * s;
      const x = tileX(h.x, z);
      const y = tileY(h.y, z);
      const tx = Math.min(Math.max(0, x + T / 2 - w / 2), sc.width - w);
      const ty = y - ht - 2 * s >= 0 ? y - ht - 2 * s : y + T + 2 * s;
      this.rect(x, y, T, T, [1, 1, 1, 0.12]);
      this.rect(tx, ty, w, ht, [0, 0, 0, 0.75]);
      result.labels.push({ x: tx + w / 2, y: ty + ht / 2, text, color: "#e8e8e8", small: true });
    }
    if (sc.opts.tooltips && T >= 24 && !result.far) {
      const halfW = sc.width / 2 / sc.scale / sc.S;
      const halfH = sc.height / 2 / sc.scale / sc.S;
      const a = sc.floor <= 7 ? 7 - z : 0;
      const cx = sc.camX / sc.S + a;
      const cy = sc.camY / sc.S + a;
      for (let ty = Math.max(0, Math.floor(cy - halfH)); ty <= cy + halfH; ty++) {
        for (let tx = Math.max(0, Math.floor(cx - halfW)); tx <= cx + halfW; tx++) {
          const t = sc.chunks.peekTile(tx, ty, z);
          if (!t) continue;
          for (let k = 0; k < t.ids.length; k++) {
            const f = t.iflags[k];
            if (!(f & (IF.ids | IF.text | IF.teleport | IF.doorId))) continue;
            const tags: string[] = [];
            if (f & IF.ids) tags.push("id");
            if (f & IF.text) tags.push("text");
            if (f & IF.teleport) tags.push("tp");
            if (f & IF.doorId) tags.push("door");
            result.labels.push({ x: tileX(tx, z) + T / 2, y: tileY(ty, z) + T / 2, text: `${t.ids[k]} ${tags.join(" ")}`, color: "#ffd75e" });
            break;
          }
        }
      }
    }
  }

  /** Binds the chunk program, unless it still is. */
  private bindChunks(sc: Scene, alpha: number): void {
    const gl = this.gl;
    // Uploads to the atlas or the cell index may have happened since.
    if (this.chunkAlpha < 0 || this.boundTex !== this.atlas!.tex) {
      const atlas = this.atlas!;
      gl.useProgram(this.progChunk);
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D_ARRAY, atlas.tex);
      this.boundTex = atlas.tex;
      gl.uniform1i(this.uc.u_tex, 0);
      gl.uniform1i(this.uc.u_ind, this.ind.unit);
      gl.uniform1i(this.uc.u_indC, this.indC.unit);
      gl.uniform2f(this.uc.u_screen, sc.width, sc.height);
      gl.uniform1ui(this.uc.u_perRow, atlas.perRow);
      gl.uniform1ui(this.uc.u_perLayer, atlas.perLayer);
      gl.uniform1f(this.uc.u_uvCell, atlas.uvCell);
      gl.uniform1f(this.uc.u_scale, sc.scale);
      gl.uniform1f(this.uc.u_S, sc.S);
      gl.uniform2f(this.uc.u_half, sc.width / 2, sc.height / 2);
      this.chunkAlpha = -1;
    }
    this.ind.bind();
    this.indC.bind();
    if (this.chunkAlpha !== alpha) gl.uniform1f(this.uc.u_alpha, alpha);
    this.chunkAlpha = alpha;
  }

  /** Binds the sprite program and atlas (the atlas texture can change while building). */
  private useSprites(sc: Scene): void {
    const gl = this.gl;
    this.chunkAlpha = -1;
    this.boundTex = this.atlas!.tex;
    gl.useProgram(this.prog);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, this.atlas!.tex);
    gl.uniform1i(this.u.u_tex, 0);
    gl.uniform2f(this.u.u_screen, sc.width, sc.height);
    gl.uniform1f(this.u.u_perRow, this.atlas!.perRow);
    gl.uniform1f(this.u.u_uvCell, this.atlas!.uvCell);
  }

  /** Draws the screen-space quads pushed so far. */
  private flushScreen(sc: Scene): void {
    const gl = this.gl;
    if (!this.n) return;
    this.useSprites(sc);
    gl.uniform2f(this.u.u_off, 0, 0);
    gl.uniform1f(this.u.u_scale, 1);
    gl.uniform2f(this.u.u_half, 0, 0);
    gl.uniform1f(this.u.u_alpha, 1);
    gl.bindVertexArray(this.vao);
    gl.bindBuffer(gl.ARRAY_BUFFER, this.inst);
    gl.bufferData(gl.ARRAY_BUFFER, this.data.subarray(0, this.n * FLOATS), gl.STREAM_DRAW);
    gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, this.n);
    gl.bindVertexArray(null);
    this.n = 0;
  }
}

/** Light-weight per-tile draw description shared by palettes and ghosts. */
export type { TileData };

/** Atlas key of a colored outfit piece (colors: head, body, legs, feet bytes). */
function composedKey(sid: number, tpl: number, colors: number): string {
  return `o:${sid}:${tpl}:${colors & 255}.${(colors >> 8) & 255}.${(colors >> 16) & 255}.${colors >>> 24}`;
}
