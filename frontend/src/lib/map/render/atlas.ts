// Sprite atlas of the map renderer: one WebGL2 texture array of square
// cells. Plain sprites are keyed by their id, composed images (colored
// outfits) by a string. The texture grows by layers up to a limit, then
// the least recently used cells are reused.

import { ATLAS } from "../constants";

export type AtlasKey = number | string;

export class Atlas {
  texture: WebGLTexture;
  readonly perRow: number;
  readonly perLayer: number;
  layers: number = ATLAS.startLayers;
  private slots = new Map<AtlasKey, number>();
  private keys: (AtlasKey | undefined)[] = [];
  private used = new Uint32Array(0);
  private free: number[] = [];
  private next = 1; // slot 0 is white
  private frame = 1;
  private uploads = 0;
  /** Set when an upload was deferred: the frame should be drawn again. */
  pending = false;
  /** Bumped whenever a cell is freed: cells remembered from before may hold other pixels now. */
  epoch = 0;
  /** Called with the key of every freed cell. */
  onRelease: (key: AtlasKey) => void = () => {};

  constructor(
    private gl: WebGL2RenderingContext,
    readonly cell: number,
  ) {
    this.perRow = Math.floor(ATLAS.size / cell);
    this.perLayer = this.perRow * this.perRow;
    this.texture = this.create(this.layers);
    this.used = new Uint32Array(this.layers * this.perLayer);
    const white = new Uint8Array(cell * cell * 4).fill(255);
    gl.texSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, 0, cell, cell, 1, gl.RGBA, gl.UNSIGNED_BYTE, white);
  }

  get uvCell(): number {
    return this.cell / ATLAS.size;
  }

  private create(layers: number): WebGLTexture {
    const gl = this.gl;
    const texture = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
    gl.texStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.RGBA8, ATLAS.size, ATLAS.size, layers);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    return texture;
  }

  /** Starts a frame: cells used from now on are protected from reuse. */
  begin(): void {
    this.frame++;
    this.uploads = 0;
    this.pending = false;
    this.gl.bindTexture(this.gl.TEXTURE_2D_ARRAY, this.texture);
  }

  /** Marks cells used this frame without looking them up (cached geometry). */
  touch(cells: Int32Array): void {
    const used = this.used;
    const frame = this.frame;
    for (let i = 0; i < cells.length; i++) used[cells[i]] = frame;
  }

  /** Marks one cell used this frame. */
  use(cell: number): void {
    this.used[cell] = this.frame;
  }

  /** The cell of key when it is already in the atlas (marked used), else -1.
   * Hot loops check this first so a cache hit allocates no callback. */
  cached(key: AtlasKey): number {
    const slot = this.slots.get(key);
    if (slot === undefined) return -1;
    this.used[slot] = this.frame;
    return slot;
  }

  /** Cell of a key, uploading its pixels when needed; -1 when the pixels
   * are not available yet or the upload budget is spent. */
  cellOf(key: AtlasKey, pixels: () => Uint8ClampedArray | undefined | null): number {
    const cached = this.cached(key);
    if (cached >= 0) return cached;
    if (this.uploads >= ATLAS.uploadsPerFrame) {
      this.pending = true;
      return -1;
    }
    const data = pixels();
    if (!data) return -1;
    const slot = this.alloc();
    if (slot < 0) return -1;
    this.uploads++;
    const layer = Math.floor(slot / this.perLayer);
    const inLayer = slot % this.perLayer;
    const gl = this.gl;
    gl.texSubImage3D(
      gl.TEXTURE_2D_ARRAY, 0, (inLayer % this.perRow) * this.cell, Math.floor(inLayer / this.perRow) * this.cell, layer,
      this.cell, this.cell, 1, gl.RGBA, gl.UNSIGNED_BYTE, data,
    );
    this.slots.set(key, slot);
    this.keys[slot] = key;
    this.used[slot] = this.frame;
    return slot;
  }

  private alloc(): number {
    const freed = this.free.pop();
    if (freed !== undefined) return freed;
    if (this.next < this.layers * this.perLayer) return this.next++;
    if (this.layers < ATLAS.maxLayers) {
      this.grow(Math.min(this.layers * 2, ATLAS.maxLayers));
      return this.next++;
    }
    this.evict();
    return this.free.pop() ?? -1;
  }

  private grow(layers: number): void {
    const gl = this.gl;
    const old = this.texture;
    const texture = this.create(layers);
    const framebuffer = gl.createFramebuffer();
    gl.bindFramebuffer(gl.READ_FRAMEBUFFER, framebuffer);
    for (let layer = 0; layer < this.layers; layer++) {
      gl.framebufferTextureLayer(gl.READ_FRAMEBUFFER, gl.COLOR_ATTACHMENT0, old, 0, layer);
      gl.copyTexSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, layer, 0, 0, ATLAS.size, ATLAS.size);
    }
    gl.bindFramebuffer(gl.READ_FRAMEBUFFER, null);
    gl.deleteFramebuffer(framebuffer);
    gl.deleteTexture(old);
    this.texture = texture;
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, texture);
    const used = new Uint32Array(layers * this.perLayer);
    used.set(this.used);
    this.used = used;
    this.layers = layers;
  }

  /** Frees the oldest eighth of the cells that were not used this frame. */
  private evict(): void {
    const total = this.layers * this.perLayer;
    const ages: number[] = [];
    for (let slot = 1; slot < total; slot++) if (this.used[slot] < this.frame && this.keys[slot] !== undefined) ages.push(slot);
    ages.sort((first, second) => this.used[first] - this.used[second]);
    for (const slot of ages.slice(0, Math.max(1, Math.floor(total / 8)))) this.release(slot);
  }

  private release(slot: number): void {
    const key = this.keys[slot];
    if (key !== undefined) {
      this.slots.delete(key);
      this.onRelease(key);
    }
    this.keys[slot] = undefined;
    this.free.push(slot);
    this.epoch++;
  }

  /** Drops keys (all when null) so they upload again. Composed images that
   * depend on a sprite are dropped by prefix match on "id:". */
  invalidate(ids: number[] | null): void {
    if (ids === null) {
      for (const [key, slot] of this.slots) {
        this.keys[slot] = undefined;
        this.free.push(slot);
        this.onRelease(key);
      }
      this.slots.clear();
      this.epoch++;
      return;
    }
    const set = new Set(ids);
    for (const [key, slot] of [...this.slots]) {
      const hit = typeof key === "number" ? set.has(key) : key.split(":").some((part) => set.has(Number(part)));
      if (hit) this.release(slot);
    }
  }

  dispose(): void {
    this.gl.deleteTexture(this.texture);
  }
}
