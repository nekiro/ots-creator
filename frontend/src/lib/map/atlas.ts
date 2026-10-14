// Sprite atlas of the map renderer: one WebGL2 texture array of square
// cells. Plain sprites are keyed by their id, composed images (colored
// outfits) by a string. The texture grows by layers up to a limit, then
// the least recently used cells are reused.

const DIM = 1024;
const START_LAYERS = 4;
const MAX_LAYERS = 64;
/** Uploads per frame; more wait for the next frame so panning stays smooth. */
const MAX_UPLOADS = 1500;

export type AtlasKey = number | string;

export class Atlas {
  tex: WebGLTexture;
  readonly perRow: number;
  readonly perLayer: number;
  layers = START_LAYERS;
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
    this.perRow = Math.floor(DIM / cell);
    this.perLayer = this.perRow * this.perRow;
    this.tex = this.create(this.layers);
    this.used = new Uint32Array(this.layers * this.perLayer);
    const white = new Uint8Array(cell * cell * 4).fill(255);
    gl.texSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, 0, cell, cell, 1, gl.RGBA, gl.UNSIGNED_BYTE, white);
  }

  get uvCell(): number {
    return this.cell / DIM;
  }

  private create(layers: number): WebGLTexture {
    const gl = this.gl;
    const tex = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, tex);
    gl.texStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.RGBA8, DIM, DIM, layers);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    return tex;
  }

  /** Starts a frame: cells used from now on are protected from reuse. */
  begin(): void {
    this.frame++;
    this.uploads = 0;
    this.pending = false;
    this.gl.bindTexture(this.gl.TEXTURE_2D_ARRAY, this.tex);
  }

  /** Marks cells used this frame without looking them up (cached geometry). */
  touch(cells: Int32Array): void {
    const used = this.used;
    const f = this.frame;
    for (let i = 0; i < cells.length; i++) used[cells[i]] = f;
  }

  /** Marks one cell used this frame. */
  use(cell: number): void {
    this.used[cell] = this.frame;
  }

  /** Cell of a key, uploading its pixels when needed; -1 when the pixels
   * are not available yet or the upload budget is spent. */
  cellOf(key: AtlasKey, pixels: () => Uint8ClampedArray | undefined | null): number {
    const s = this.slots.get(key);
    if (s !== undefined) {
      this.used[s] = this.frame;
      return s;
    }
    if (this.uploads >= MAX_UPLOADS) {
      this.pending = true;
      return -1;
    }
    const px = pixels();
    if (!px) return -1;
    const slot = this.alloc();
    if (slot < 0) return -1;
    this.uploads++;
    const layer = Math.floor(slot / this.perLayer);
    const i = slot % this.perLayer;
    const gl = this.gl;
    gl.texSubImage3D(
      gl.TEXTURE_2D_ARRAY, 0, (i % this.perRow) * this.cell, Math.floor(i / this.perRow) * this.cell, layer,
      this.cell, this.cell, 1, gl.RGBA, gl.UNSIGNED_BYTE, px,
    );
    this.slots.set(key, slot);
    this.keys[slot] = key;
    this.used[slot] = this.frame;
    return slot;
  }

  private alloc(): number {
    const f = this.free.pop();
    if (f !== undefined) return f;
    if (this.next < this.layers * this.perLayer) return this.next++;
    if (this.layers < MAX_LAYERS) {
      this.grow(Math.min(this.layers * 2, MAX_LAYERS));
      return this.next++;
    }
    this.evict();
    return this.free.pop() ?? -1;
  }

  private grow(layers: number): void {
    const gl = this.gl;
    const old = this.tex;
    const tex = this.create(layers);
    const fb = gl.createFramebuffer();
    gl.bindFramebuffer(gl.READ_FRAMEBUFFER, fb);
    for (let l = 0; l < this.layers; l++) {
      gl.framebufferTextureLayer(gl.READ_FRAMEBUFFER, gl.COLOR_ATTACHMENT0, old, 0, l);
      gl.copyTexSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, l, 0, 0, DIM, DIM);
    }
    gl.bindFramebuffer(gl.READ_FRAMEBUFFER, null);
    gl.deleteFramebuffer(fb);
    gl.deleteTexture(old);
    this.tex = tex;
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, tex);
    const used = new Uint32Array(layers * this.perLayer);
    used.set(this.used);
    this.used = used;
    this.layers = layers;
  }

  /** Frees the oldest eighth of the cells that were not used this frame. */
  private evict(): void {
    const total = this.layers * this.perLayer;
    const ages: number[] = [];
    for (let s = 1; s < total; s++) if (this.used[s] < this.frame && this.keys[s] !== undefined) ages.push(s);
    ages.sort((a, b) => this.used[a] - this.used[b]);
    for (const s of ages.slice(0, Math.max(1, Math.floor(total / 8)))) this.release(s);
  }

  private release(slot: number): void {
    const k = this.keys[slot];
    if (k !== undefined) {
      this.slots.delete(k);
      this.onRelease(k);
    }
    this.keys[slot] = undefined;
    this.free.push(slot);
    this.epoch++;
  }

  /** Drops keys (all when null) so they upload again. Composed images that
   * depend on a sprite are dropped by prefix match on "id:". */
  invalidate(ids: number[] | null): void {
    if (ids === null) {
      for (const [k, s] of this.slots) {
        this.keys[s] = undefined;
        this.free.push(s);
        this.onRelease(k);
      }
      this.slots.clear();
      this.epoch++;
      return;
    }
    const set = new Set(ids);
    for (const [k, s] of [...this.slots]) {
      const hit = typeof k === "number" ? set.has(k) : k.split(":").some((part) => set.has(Number(part)));
      if (hit) this.release(s);
    }
  }

  dispose(): void {
    this.gl.deleteTexture(this.tex);
  }
}
