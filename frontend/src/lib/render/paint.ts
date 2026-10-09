// Pixel painting on one texture: strokes on an ImageData and splitting the
// result back into sprite slots.

import { textureSlots, type Layout, type TexturePos } from "./layout";

export type RGBA = [number, number, number, number];

/** Sets one pixel; outside the image does nothing. */
export function setPixel(img: ImageData, x: number, y: number, c: RGBA): void {
  if (x < 0 || y < 0 || x >= img.width || y >= img.height) return;
  img.data.set(c, (y * img.width + x) * 4);
}

export function getPixel(img: ImageData, x: number, y: number): RGBA {
  const o = (y * img.width + x) * 4;
  return [img.data[o], img.data[o + 1], img.data[o + 2], img.data[o + 3]];
}

/** Paints every pixel on the line from (x0, y0) to (x1, y1) (Bresenham). */
export function drawLine(img: ImageData, x0: number, y0: number, x1: number, y1: number, c: RGBA): void {
  const dx = Math.abs(x1 - x0);
  const dy = -Math.abs(y1 - y0);
  const sx = x0 < x1 ? 1 : -1;
  const sy = y0 < y1 ? 1 : -1;
  let err = dx + dy;
  for (;;) {
    setPixel(img, x0, y0, c);
    if (x0 === x1 && y0 === y1) return;
    const e2 = 2 * err;
    if (e2 >= dy) {
      err += dy;
      x0 += sx;
    }
    if (e2 <= dx) {
      err += dx;
      y0 += sy;
    }
  }
}

/** Copies the size x size tile at (x, y) out of an image. */
function tile(img: ImageData, x: number, y: number, size: number): Uint8ClampedArray {
  const out = new Uint8ClampedArray(size * size * 4);
  for (let row = 0; row < size; row++) {
    const o = ((y + row) * img.width + x) * 4;
    out.set(img.data.subarray(o, o + size * 4), row * size * 4);
  }
  return out;
}

function equal(a: Uint8ClampedArray, b: Uint8ClampedArray): boolean {
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}

/**
 * Slots of the texture at pos whose pixels differ between before and after
 * (both the raw texture, one layer, no addons or colors), with the new
 * pixels of each.
 */
export function changedSlots(before: ImageData, after: ImageData, g: Layout, pos: TexturePos, size: number): { slots: number[]; pixels: Uint8ClampedArray[] } {
  const slots: number[] = [];
  const pixels: Uint8ClampedArray[] = [];
  for (const s of textureSlots(g, pos, size)) {
    const next = tile(after, s.x, s.y, size);
    if (equal(tile(before, s.x, s.y, size), next)) continue;
    slots.push(s.slot);
    pixels.push(next);
  }
  return { slots, pixels };
}

export function hexToRGBA(hex: string): RGBA {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255, 255];
}

export function rgbaToHex(c: RGBA): string {
  return "#" + ((c[0] << 16) | (c[1] << 8) | c[2]).toString(16).padStart(6, "0");
}
