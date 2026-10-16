#version 300 es
// Quads built on the CPU (previews, overlays): a rectangle, an atlas cell and a color each.
// Quads are in "a_rect" units; a pixel is round((p + u_off) * u_scale + u_half).
// World chunks pass the camera there, screen-space quads pass (0, 1, 0).
// A negative layer hides the quad (an animation frame without a sprite).
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
// Depth of the quads (0: under everything; the play view's player sits among the chunk quads).
uniform float u_depth;
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
  gl_Position = vec4(p.x / u_screen.x * 2.0 - 1.0, 1.0 - p.y / u_screen.y * 2.0, u_depth, 1.0);
  float layer = a_cell.x;
  float col = mod(a_cell.y, u_perRow);
  float row = floor(a_cell.y / u_perRow);
  v_uv = vec3((vec2(col, row) + a_corner) * u_uvCell, layer);
  v_color = vec4(a_color.rgb, a_color.a * u_alpha);
}
