#version 300 es
// Chunk quads (QUAD_BYTES each, see geo.ts): the sprite id is looked up in
// the cell index (composed outfit pieces in their own index). The depth is
// the drawing rank of the quad's tile: chunks are drawn whole, so a quad of
// a later chunk must not cover one of an earlier chunk's tile drawn after
// it (a sprite reaching up into the chunk above, see map_geo.go).
layout(location=0) in vec2 a_corner;
layout(location=1) in vec2 a_pos;
layout(location=2) in uint a_sprite;
layout(location=3) in vec4 a_color;
layout(location=4) in vec2 a_tile;
uniform vec2 u_screen;
uniform vec2 u_off;
// The chunk's corner tile from the tile the ranks start at.
uniform vec2 u_tile;
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
// Ranks from -DEPTH_RANGE to DEPTH_RANGE fit the depth range.
const float DEPTH_RANGE = 524288.0;
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
  // Diagonals from the top-left, each from its bottom-left end, as
  // map_geo.go orders tiles; exact for tiles less than 8 apart, the only
  // ones whose sprites overlap.
  vec2 t = a_tile + u_tile;
  float rank = 8.0 * (t.x + t.y) + t.x;
  gl_Position = vec4(p.x / u_screen.x * 2.0 - 1.0, 1.0 - p.y / u_screen.y * 2.0, clamp(rank / DEPTH_RANGE, -1.0, 1.0), 1.0);
  uint c = cell % u_perLayer;
  v_uv = vec3((vec2(float(c % u_perRow), float(c / u_perRow)) + a_corner) * u_uvCell, float(cell / u_perLayer));
  v_color = vec4(a_color.rgb, a_color.a * u_alpha);
}
