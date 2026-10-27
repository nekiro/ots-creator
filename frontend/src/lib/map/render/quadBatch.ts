import type { Atlas } from "./atlas";
import { GPU } from "../constants";
import { compileProgram, createQuadVao } from "./gpu";
import type { Scene } from "./scene";
import QUAD_VERT from "./shaders/quad.vert?raw";
import SPRITE_FRAG from "./shaders/sprite.frag?raw";

export type Color = readonly [number, number, number, number] | readonly number[];

export class QuadBatch {
  private program: WebGLProgram;
  private vao: WebGLVertexArrayObject;
  private instances: WebGLBuffer;
  private quads = new Float32Array(GPU.floatsPerQuad * 4096);
  private quadCount = 0;
  private uniforms: Record<string, WebGLUniformLocation | null> = {};
  atlas: Atlas | null = null;

  constructor(
    private gl: WebGL2RenderingContext,
    corners: WebGLBuffer,
  ) {
    this.program = compileProgram(gl, QUAD_VERT, SPRITE_FRAG);
    for (const name of ["u_screen", "u_offset", "u_scale", "u_half", "u_alpha", "u_perRow", "u_uvCell", "u_tex", "u_depth"]) this.uniforms[name] = gl.getUniformLocation(this.program, name);
    this.instances = gl.createBuffer()!;
    this.vao = createQuadVao(gl, corners, this.instances);
  }

  clear(): void {
    this.quadCount = 0;
  }

  /** A sprite quad (cell: atlas cell, -1 hides it) tinted with red, green, blue, alpha. */
  push(x: number, y: number, width: number, height: number, cell: number, red: number, green: number, blue: number, alpha: number): void {
    if ((this.quadCount + 1) * GPU.floatsPerQuad > this.quads.length) {
      const grown = new Float32Array(this.quads.length * 2);
      grown.set(this.quads);
      this.quads = grown;
    }
    const offset = this.quadCount * GPU.floatsPerQuad;
    const atlas = this.atlas!;
    const quads = this.quads;
    quads[offset] = x;
    quads[offset + 1] = y;
    quads[offset + 2] = width;
    quads[offset + 3] = height;
    if (cell < 0) {
      quads[offset + 4] = -1;
      quads[offset + 5] = -1;
    } else {
      quads[offset + 4] = Math.floor(cell / atlas.perLayer);
      quads[offset + 5] = cell % atlas.perLayer;
    }
    quads[offset + 6] = red;
    quads[offset + 7] = green;
    quads[offset + 8] = blue;
    quads[offset + 9] = alpha;
    this.quadCount++;
  }

  rect(x: number, y: number, width: number, height: number, color: Color): void {
    this.push(x, y, width, height, 0, color[0], color[1], color[2], color[3]);
  }

  stroke(x: number, y: number, width: number, height: number, color: Color, lineWidth: number): void {
    this.rect(x, y, width, lineWidth, color);
    this.rect(x, y + height - lineWidth, width, lineWidth, color);
    this.rect(x, y, lineWidth, height, color);
    this.rect(x + width - lineWidth, y, lineWidth, height, color);
  }

  /** Draws the quads pushed so far, at a depth (see quad.vert). False when
   * there were none: the program bound before is still bound. */
  flush(scene: Scene, depth = 0): boolean {
    const gl = this.gl;
    if (!this.quadCount) return false;
    this.useSprites(scene);
    gl.uniform2f(this.uniforms.u_offset, 0, 0);
    gl.uniform1f(this.uniforms.u_scale, 1);
    gl.uniform2f(this.uniforms.u_half, 0, 0);
    gl.uniform1f(this.uniforms.u_alpha, 1);
    gl.uniform1f(this.uniforms.u_depth, depth);
    gl.bindVertexArray(this.vao);
    gl.bindBuffer(gl.ARRAY_BUFFER, this.instances);
    gl.bufferData(gl.ARRAY_BUFFER, this.quads.subarray(0, this.quadCount * GPU.floatsPerQuad), gl.STREAM_DRAW);
    gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, this.quadCount);
    gl.bindVertexArray(null);
    this.quadCount = 0;
    return true;
  }

  /** Binds the sprite program and atlas (the atlas texture can change while building). */
  private useSprites(scene: Scene): void {
    const gl = this.gl;
    const atlas = this.atlas!;
    gl.useProgram(this.program);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, atlas.texture);
    gl.uniform1i(this.uniforms.u_tex, 0);
    gl.uniform2f(this.uniforms.u_screen, scene.width, scene.height);
    gl.uniform1f(this.uniforms.u_perRow, atlas.perRow);
    gl.uniform1f(this.uniforms.u_uvCell, atlas.uvCell);
  }
}
