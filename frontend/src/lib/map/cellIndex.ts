// Sprite id to atlas cell lookup the chunk shader reads.
import { GPU } from "./constants";

/** Atlas cells of ids in a R32UI texture the chunk shader reads (GPU.missingCell: not in the atlas). */
export class CellIndex {
  private cells: Uint32Array;
  private texture: WebGLTexture;
  private rows: number;
  /** Rows changed since the last upload. */
  private dirtyFrom = Infinity;
  private dirtyTo = -1;
  /** The texture must be allocated again (new size or cleared). */
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
    this.cells = new Uint32Array(width * rows).fill(GPU.missingCell);
    this.texture = gl.createTexture()!;
  }

  get(id: number): number {
    return id < this.cells.length ? this.cells[id] : GPU.missingCell;
  }

  set(id: number, cell: number): void {
    if (id >= this.cells.length) {
      let rows = this.rows;
      while (id >= this.width * rows) rows *= 2;
      const grown = new Uint32Array(this.width * rows).fill(GPU.missingCell);
      grown.set(this.cells);
      this.cells = grown;
      this.rows = rows;
      this.grown = true;
    }
    if (this.cells[id] === cell) return;
    this.cells[id] = cell;
    const row = Math.floor(id / this.width);
    this.dirtyFrom = Math.min(this.dirtyFrom, row);
    this.dirtyTo = Math.max(this.dirtyTo, row);
  }

  clear(): void {
    this.cells.fill(GPU.missingCell);
    this.grown = true;
  }

  /** Uploads changes and binds the texture to its unit (leaves unit 0 active). */
  bind(): void {
    if (this.bound && !this.grown && this.dirtyTo < 0) return;
    this.bound = true;
    const gl = this.gl;
    gl.activeTexture(gl.TEXTURE0 + this.unit);
    gl.bindTexture(gl.TEXTURE_2D, this.texture);
    if (this.grown) {
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.R32UI, this.width, this.rows, 0, gl.RED_INTEGER, gl.UNSIGNED_INT, this.cells);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
      this.grown = false;
    } else if (this.dirtyTo >= 0) {
      const rows = this.cells.subarray(this.dirtyFrom * this.width, (this.dirtyTo + 1) * this.width);
      gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, this.dirtyFrom, this.width, this.dirtyTo - this.dirtyFrom + 1, gl.RED_INTEGER, gl.UNSIGNED_INT, rows);
    }
    this.dirtyFrom = Infinity;
    this.dirtyTo = -1;
    gl.activeTexture(gl.TEXTURE0);
  }

  dispose(): void {
    this.gl.deleteTexture(this.texture);
  }
}
