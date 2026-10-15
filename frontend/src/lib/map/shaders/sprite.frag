#version 300 es
// Atlas texel tinted by the quad color; transparent texels are discarded.
precision mediump float;
precision mediump sampler2DArray;
uniform sampler2DArray u_tex;
in vec3 v_uv;
in vec4 v_color;
out vec4 outColor;
void main() {
  vec4 t = texture(u_tex, v_uv);
  if (t.a == 0.0) discard;
  outColor = t * v_color;
}
