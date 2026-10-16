// WebGL programs and vertex layouts of the map renderer.
import { GPU } from "./constants";
import { QUAD_BYTES } from "./geo";

export function compileProgram(gl: WebGL2RenderingContext, vertexSource: string, fragmentSource: string): WebGLProgram {
  const program = gl.createProgram()!;
  for (const [type, source] of [
    [gl.VERTEX_SHADER, vertexSource],
    [gl.FRAGMENT_SHADER, fragmentSource],
  ] as const) {
    const shader = gl.createShader(type)!;
    gl.shaderSource(shader, source);
    gl.compileShader(shader);
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(shader) ?? "shader error");
    gl.attachShader(program, shader);
  }
  gl.linkProgram(program);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(program) ?? "link error");
  return program;
}

/** A vertex array reading quads from an instance buffer (compact: the chunk format), corners from the unit square buffer. */
export function createQuadVao(gl: WebGL2RenderingContext, corners: WebGLBuffer, instances: WebGLBuffer, compact = false): WebGLVertexArrayObject {
  const vao = gl.createVertexArray()!;
  gl.bindVertexArray(vao);
  gl.bindBuffer(gl.ARRAY_BUFFER, corners);
  gl.enableVertexAttribArray(0);
  gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
  gl.bindBuffer(gl.ARRAY_BUFFER, instances);
  gl.enableVertexAttribArray(1);
  gl.enableVertexAttribArray(2);
  gl.enableVertexAttribArray(3);
  if (compact) {
    gl.vertexAttribPointer(1, 2, gl.SHORT, false, QUAD_BYTES, 0);
    gl.vertexAttribIPointer(2, 1, gl.UNSIGNED_INT, QUAD_BYTES, 4);
    gl.vertexAttribPointer(3, 4, gl.UNSIGNED_BYTE, true, QUAD_BYTES, 8);
  } else {
    const stride = GPU.floatsPerQuad * 4;
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
