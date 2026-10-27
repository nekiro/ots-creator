// The quads of map chunks on the GPU: uploaded within a per-frame budget,
// drawn with the camera offset, freed when not drawn for a while. The chunk
// shader finds sprites through cell indexes, so atlas changes never upload
// chunks again; only animated pieces are patched.
import { frameAt, spriteIndex, type Look } from "../data/appearances";
import type { Atlas } from "./atlas";
import { CellIndex } from "./cellIndex";
import { CHUNK, splitKey } from "../data/chunks";
import { composedKey, composeOutfit, unpackColors } from "./composed";
import { GPU, LIMITS } from "../constants";
import { GEO_BLOCK, GEO_COMPOSED, GEO_HIDDEN, QUAD_BYTES, type GeoChunk } from "./geo";
import { compileProgram, createQuadVao } from "./gpu";
import type { FrameResult, Scene } from "./scene";
import CHUNK_VERT from "./shaders/chunk.vert?raw";
import SPRITE_FRAG from "./shaders/sprite.frag?raw";

interface ChunkBuffer {
  vao: WebGLVertexArrayObject;
  buffer: WebGLBuffer;
  source: GeoChunk;
  cells: Int32Array;
  /** The cells are valid for this atlas and epoch, and every sprite was in it. */
  atlas: Atlas | null;
  epoch: number;
  ready: boolean;
  lastDrawn: number;
  /** The quads, kept on the CPU to patch animated pieces. */
  quads: Uint8Array | null;
}

const NO_QUADS = new Uint8Array(0);

export class ChunkBuffers {
  private program: WebGLProgram;
  private uniforms: Record<string, WebGLUniformLocation | null> = {};
  private atlas: Atlas | null = null;
  private buffers = new Map<number, ChunkBuffer>();
  private spriteCells: CellIndex;
  private composedCells: CellIndex;
  private composedIds = new Map<string, number>();
  private composedKeys: string[] = [];
  private frameNumber = 0;
  /** The chunk program is bound with this alpha (-1: not bound); anything else drawn clears it. */
  private boundAlpha = -1;
  private boundTexture: WebGLTexture | null = null;
  private uploadedBytes = 0;
  private uploadDeadline = 0;

  constructor(
    private gl: WebGL2RenderingContext,
    private corners: WebGLBuffer,
    private missing: Set<number>,
  ) {
    this.program = compileProgram(gl, CHUNK_VERT, SPRITE_FRAG);
    for (const name of ["u_screen", "u_offset", "u_tile", "u_spriteSize", "u_scale", "u_half", "u_alpha", "u_perRow", "u_perLayer", "u_uvCell", "u_tex", "u_spriteCells", "u_composedCells"]) this.uniforms[name] = gl.getUniformLocation(this.program, name);
    this.spriteCells = new CellIndex(gl, 2, 2048, 64);
    this.composedCells = new CellIndex(gl, 3, 1024, 4);
  }

  useAtlas(atlas: Atlas): void {
    this.atlas = atlas;
    this.spriteCells.clear();
    this.composedCells.clear();
    atlas.onRelease = (key) => {
      if (typeof key === "number") this.spriteCells.set(key, GPU.missingCell);
      else {
        const index = this.composedIds.get(key);
        if (index !== undefined) this.composedCells.set(index, GPU.missingCell);
      }
    };
  }

  beginFrame(): void {
    this.frameNumber++;
    this.boundAlpha = -1;
    this.uploadedBytes = 0;
    this.uploadDeadline = performance.now() + LIMITS.uploadMs;
  }

  unbind(): void {
    this.boundAlpha = -1;
    this.boundTexture = this.atlas!.texture;
  }

  prefetch(chunk: GeoChunk): void {
    const buffer = this.upload(chunk, true);
    if (buffer) buffer.lastDrawn = this.frameNumber;
  }

  private free(buffer: ChunkBuffer): void {
    this.gl.deleteVertexArray(buffer.vao);
    this.gl.deleteBuffer(buffer.buffer);
  }

  dispose(): void {
    for (const buffer of this.buffers.values()) this.free(buffer);
    this.buffers.clear();
    this.spriteCells.dispose();
    this.composedCells.dispose();
  }

  /** The chunk's GPU buffer, uploaded when the store has a new version and
   * the frame has upload budget left (spareTimeOnly: never past the budget,
   * not even for the first upload of the frame). Null when not uploaded yet. */
  upload(chunk: GeoChunk, spareTimeOnly = false): ChunkBuffer | null {
    let buffer = this.buffers.get(chunk.key) ?? null;
    if (buffer && buffer.source === chunk) return buffer;
    // Something always uploads, unless only spare time is asked for.
    if ((this.uploadedBytes > 0 || spareTimeOnly) && (this.uploadedBytes + chunk.quads.byteLength > LIMITS.uploadBytes || performance.now() > this.uploadDeadline)) return buffer;
    this.uploadedBytes += chunk.quads.byteLength;
    const gl = this.gl;
    if (!buffer) {
      const glBuffer = gl.createBuffer()!;
      buffer = { vao: createQuadVao(gl, this.corners, glBuffer, true), buffer: glBuffer, source: chunk, cells: new Int32Array(0), atlas: null, epoch: 0, ready: false, lastDrawn: 0, quads: null };
      this.buffers.set(chunk.key, buffer);
    }
    // Composed outfit pieces point at their entry in the composed index.
    const quads = chunk.quads;
    if (chunk.composed.length) {
      const view = new DataView(quads.buffer, quads.byteOffset, quads.byteLength);
      const composed = chunk.composed;
      for (let i = 0; i < composed.length; i += 4) {
        const key = composedKey(composed[i + 1], composed[i + 2], composed[i + 3]);
        let index = this.composedIds.get(key);
        if (index === undefined) {
          index = this.composedKeys.length;
          this.composedIds.set(key, index);
          this.composedKeys.push(key);
        }
        view.setUint32(composed[i] * QUAD_BYTES + 4, (GEO_COMPOSED | index) >>> 0, true);
      }
    }
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer.buffer);
    gl.bufferData(gl.ARRAY_BUFFER, quads, chunk.animated.length ? gl.DYNAMIC_DRAW : gl.STATIC_DRAW);
    buffer.source = chunk;
    buffer.atlas = null;
    buffer.quads = chunk.animated.length ? quads : null;
    // Only animated chunks patch their quads: the others need no CPU copy.
    if (!chunk.animated.length) chunk.quads = NO_QUADS;
    return buffer;
  }

  private ensureSprites(scene: Scene, buffer: ChunkBuffer, result: FrameResult): void {
    const atlas = this.atlas!;
    if (buffer.atlas === atlas && buffer.epoch === atlas.epoch && buffer.ready) {
      atlas.touch(buffer.cells);
      return;
    }
    const cells: number[] = [];
    let ready = true;
    for (const spriteId of buffer.source.sprites) {
      const cell = this.cellOfSprite(scene, spriteId);
      if (cell < 0) ready = false;
      else cells.push(cell);
    }
    const composed = buffer.source.composed;
    for (let i = 0; i < composed.length; i += 4) {
      const key = composedKey(composed[i + 1], composed[i + 2], composed[i + 3]);
      const index = this.composedIds.get(key)!;
      let cell = this.composedCells.get(index);
      if (cell === GPU.missingCell) {
        const spriteId = composed[i + 1];
        const templateId = composed[i + 2];
        const colors = unpackColors(composed[i + 3]);
        cell = atlas.cellOf(key, () => composeOutfit(scene.sprites, spriteId, templateId, colors));
        if (cell < 0) {
          ready = false;
          if (!scene.sprites.get(spriteId)) this.missing.add(spriteId);
          if (templateId && !scene.sprites.get(templateId)) this.missing.add(templateId);
          continue;
        }
        this.composedCells.set(index, cell);
      }
      cells.push(cell);
    }
    buffer.cells = Int32Array.from(cells);
    atlas.touch(buffer.cells);
    buffer.atlas = atlas;
    buffer.epoch = atlas.epoch;
    buffer.ready = ready;
    if (!ready) result.pending = true;
  }

  private cellOfSprite(scene: Scene, spriteId: number): number {
    const indexed = this.spriteCells.get(spriteId);
    if (indexed !== GPU.missingCell) return indexed;
    const cell = this.atlas!.cellOf(spriteId, () => scene.sprites.get(spriteId)?.pixels);
    if (cell < 0) {
      if (!scene.sprites.get(spriteId)) this.missing.add(spriteId);
      return -1;
    }
    this.spriteCells.set(spriteId, cell);
    return cell;
  }

  /** Draws a chunk of a floor shifted floorShift tiles up and left (see Camera.shift). */
  draw(scene: Scene, chunk: GeoChunk, alpha: number, floorShift: number, result: FrameResult): void {
    const buffer = this.upload(chunk);
    if (!buffer) {
      result.more = true;
      return;
    }
    if (buffer.source !== chunk) result.more = true;
    buffer.lastDrawn = this.frameNumber;
    this.ensureSprites(scene, buffer, result);
    if (buffer.quads && scene.opts.animation) {
      result.animated = true;
      this.animate(scene, buffer, result);
    }
    const gl = this.gl;
    const [chunkX, chunkY] = splitKey(buffer.source.key % GEO_BLOCK);
    this.bindProgram(scene, alpha);
    gl.uniform2f(this.uniforms.u_offset, (chunkX * CHUNK - floorShift) * scene.spriteSize - scene.camX, (chunkY * CHUNK - floorShift) * scene.spriteSize - scene.camY);
    // Tile ranks from the camera keep the depth values small.
    gl.uniform2f(this.uniforms.u_tile, chunkX * CHUNK - Math.round(scene.camX / scene.spriteSize), chunkY * CHUNK - Math.round(scene.camY / scene.spriteSize));
    gl.bindVertexArray(buffer.vao);
    gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, buffer.source.quadCount);
    gl.bindVertexArray(null);
  }

  private animate(scene: Scene, buffer: ChunkBuffer, result: FrameResult): void {
    const quads = buffer.quads!;
    const view = new DataView(quads.buffer, quads.byteOffset, quads.byteLength);
    // Three values per animated piece: quad index, client item id, frame 0 sprite index.
    const animated = buffer.source.animated;
    let changed = false;
    let lastItemId = -1;
    let look: Look | null | undefined = null;
    let frameOffset = 0;
    for (let i = 0; i < animated.length; i += 3) {
      if (animated[i + 1] !== lastItemId) {
        lastItemId = animated[i + 1];
        look = scene.looks.items.get(lastItemId);
        if (look === undefined) result.pending = true;
        if (look) frameOffset = spriteIndex(look, frameAt(look, scene.time), 0, 0, 0, 0);
      }
      if (!look) continue;
      const spriteId = look.sprites[animated[i + 2] + frameOffset] ?? 0;
      let value = GEO_HIDDEN;
      if (spriteId) {
        const cell = this.cellOfSprite(scene, spriteId);
        if (cell < 0) continue;
        this.atlas!.use(cell);
        value = spriteId;
      }
      const offset = animated[i] * QUAD_BYTES + 4;
      if (view.getUint32(offset, true) !== value) {
        view.setUint32(offset, value, true);
        changed = true;
      }
    }
    if (changed) {
      const gl = this.gl;
      gl.bindBuffer(gl.ARRAY_BUFFER, buffer.buffer);
      gl.bufferSubData(gl.ARRAY_BUFFER, 0, quads);
    }
  }

  /** Frees chunk buffers not drawn for a while once there are many (the store fetches them again). */
  sweep(scene: Scene): void {
    if (this.buffers.size <= LIMITS.chunkBuffers) return;
    const oldest = this.frameNumber - LIMITS.keepFrames;
    for (const [key, buffer] of this.buffers) {
      if (buffer.lastDrawn < oldest) {
        this.free(buffer);
        this.buffers.delete(key);
        scene.geo.drop(key);
      }
    }
  }

  private bindProgram(scene: Scene, alpha: number): void {
    const gl = this.gl;
    // Uploads to the atlas or the cell index may have happened since.
    if (this.boundAlpha < 0 || this.boundTexture !== this.atlas!.texture) {
      const atlas = this.atlas!;
      const uniforms = this.uniforms;
      gl.useProgram(this.program);
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D_ARRAY, atlas.texture);
      this.boundTexture = atlas.texture;
      gl.uniform1i(uniforms.u_tex, 0);
      gl.uniform1i(uniforms.u_spriteCells, this.spriteCells.unit);
      gl.uniform1i(uniforms.u_composedCells, this.composedCells.unit);
      gl.uniform2f(uniforms.u_screen, scene.width, scene.height);
      gl.uniform1ui(uniforms.u_perRow, atlas.perRow);
      gl.uniform1ui(uniforms.u_perLayer, atlas.perLayer);
      gl.uniform1f(uniforms.u_uvCell, atlas.uvCell);
      gl.uniform1f(uniforms.u_scale, scene.scale);
      gl.uniform1f(uniforms.u_spriteSize, scene.spriteSize);
      gl.uniform2f(uniforms.u_half, scene.width / 2, scene.height / 2);
      this.boundAlpha = -1;
    }
    this.spriteCells.bind();
    this.composedCells.bind();
    if (this.boundAlpha !== alpha) gl.uniform1f(this.uniforms.u_alpha, alpha);
    this.boundAlpha = alpha;
  }
}
