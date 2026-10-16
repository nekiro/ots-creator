#version 300 es
// Chunk quads (QUAD_BYTES each, see geo.ts): the sprite id is looked up in
// the cell index (composed outfit pieces in their own index). The depth is
// the drawing rank of the quad's tile: chunks are drawn whole, so a quad of
// a later chunk must not cover one of an earlier chunk's tile drawn after
// it (a sprite reaching up into the chunk above, see map_geo.go). Top items
// (tile y + TILE_TOP) rank half a tile later: the play view draws creatures
// between a tile's items and its top items (see PLAY.playerRank in constants.ts).
layout(location=0) in vec2 a_corner;
layout(location=1) in vec2 a_pos;
layout(location=2) in uint a_sprite;
layout(location=3) in vec4 a_color;
layout(location=4) in vec2 a_tile;
uniform vec2 u_screen;
uniform vec2 u_offset;
// The chunk's corner tile from the tile the ranks start at.
uniform vec2 u_tile;
uniform float u_spriteSize;
uniform float u_scale;
uniform vec2 u_half;
uniform float u_alpha;
uniform uint u_perRow;
uniform uint u_perLayer;
uniform float u_uvCell;
uniform highp usampler2D u_spriteCells;
uniform highp usampler2D u_composedCells;
out vec3 v_uv;
out vec4 v_color;
const uint MISSING = 0xFFFFFFFFu;
// Ranks from -DEPTH_RANGE to DEPTH_RANGE fit the depth range.
const float DEPTH_RANGE = 524288.0;
const float TILE_TOP = 32768.0;
uint lookup(highp usampler2D cells, uint id) {
  ivec2 size = textureSize(cells, 0);
  uint width = uint(size.x);
  if (id >= width * uint(size.y)) return MISSING;
  return texelFetch(cells, ivec2(int(id % width), int(id / width)), 0).r;
}
void main() {
  uint cell = MISSING;
  vec2 size = vec2(u_spriteSize);
  if (a_sprite != MISSING) {
    if ((a_sprite & 0x80000000u) != 0u) cell = lookup(u_composedCells, a_sprite & 0x7FFFFFFFu);
    else if ((a_sprite & 0x40000000u) != 0u) {
      // A colored rectangle: the white cell, its size in the value.
      cell = 0u;
      size = vec2(float((a_sprite >> 15) & 0x7FFFu), float(a_sprite & 0x7FFFu));
    } else cell = lookup(u_spriteCells, a_sprite);
  }
  if (cell == MISSING) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    v_uv = vec3(0.0);
    v_color = vec4(0.0);
    return;
  }
  vec2 world = a_pos + a_corner * size;
  vec2 pixel = floor((world + u_offset) * u_scale + u_half + 0.5);
  // Diagonals from the top-left, each from its bottom-left end, as
  // map_geo.go orders tiles; exact for tiles less than 8 apart, the only
  // ones whose sprites overlap.
  vec2 tile = a_tile;
  float top = 0.0;
  if (tile.y >= TILE_TOP) {
    tile.y -= TILE_TOP;
    top = 0.5;
  }
  vec2 ranked = tile + u_tile;
  float rank = 8.0 * (ranked.x + ranked.y) + ranked.x + top;
  gl_Position = vec4(pixel.x / u_screen.x * 2.0 - 1.0, 1.0 - pixel.y / u_screen.y * 2.0, clamp(rank / DEPTH_RANGE, -1.0, 1.0), 1.0);
  uint cellInLayer = cell % u_perLayer;
  v_uv = vec3((vec2(float(cellInLayer % u_perRow), float(cellInLayer / u_perRow)) + a_corner) * u_uvCell, float(cell / u_perLayer));
  v_color = vec4(a_color.rgb, a_color.a * u_alpha);
}
