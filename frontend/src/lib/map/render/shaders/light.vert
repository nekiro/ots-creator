#version 300 es
// The light map stretched over its tiles: a rectangle in screen pixels.
layout(location=0) in vec2 a_corner;
uniform vec2 u_screen;
uniform vec4 u_rect;
out vec2 v_uv;
void main() {
  vec2 pixel = u_rect.xy + a_corner * u_rect.zw;
  gl_Position = vec4(pixel.x / u_screen.x * 2.0 - 1.0, 1.0 - pixel.y / u_screen.y * 2.0, 0.0, 1.0);
  v_uv = a_corner;
}
