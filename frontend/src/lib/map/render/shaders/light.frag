#version 300 es
// One texel per tile, filtered linearly: the light fades from tile to tile.
precision mediump float;
uniform sampler2D u_light;
in vec2 v_uv;
out vec4 outColor;
void main() {
  outColor = vec4(texture(u_light, v_uv).rgb, 1.0);
}
