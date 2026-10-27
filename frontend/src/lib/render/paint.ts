import { textureSlots, type Layout, type TexturePos } from "./layout";

export type RGBA = [number, number, number, number];

export function setPixel(img: ImageData, x: number, y: number, color: RGBA): void {
  if (x < 0 || y < 0 || x >= img.width || y >= img.height) return;
  img.data.set(color, (y * img.width + x) * 4);
}

export function getPixel(img: ImageData, x: number, y: number): RGBA {
  const offset = (y * img.width + x) * 4;
  return [img.data[offset], img.data[offset + 1], img.data[offset + 2], img.data[offset + 3]];
}

export function drawLine(img: ImageData, x0: number, y0: number, x1: number, y1: number, color: RGBA): void {
  const dx = Math.abs(x1 - x0);
  const dy = -Math.abs(y1 - y0);
  const stepX = x0 < x1 ? 1 : -1;
  const stepY = y0 < y1 ? 1 : -1;
  let err = dx + dy;
  for (;;) {
    setPixel(img, x0, y0, color);
    if (x0 === x1 && y0 === y1) return;
    const doubled = 2 * err;
    if (doubled >= dy) {
      err += dy;
      x0 += stepX;
    }
    if (doubled <= dx) {
      err += dx;
      y0 += stepY;
    }
  }
}

function tile(img: ImageData, x: number, y: number, size: number): Uint8ClampedArray {
  const out = new Uint8ClampedArray(size * size * 4);
  for (let row = 0; row < size; row++) {
    const offset = ((y + row) * img.width + x) * 4;
    out.set(img.data.subarray(offset, offset + size * 4), row * size * 4);
  }
  return out;
}

function equal(first: Uint8ClampedArray, second: Uint8ClampedArray): boolean {
  for (let i = 0; i < first.length; i++) if (first[i] !== second[i]) return false;
  return true;
}

/**
 * Slots of the texture at pos whose pixels differ between before and after
 * (both the raw texture, one layer, no addons or colors), with the new
 * pixels of each.
 */
export function changedSlots(before: ImageData, after: ImageData, layout: Layout, pos: TexturePos, size: number): { slots: number[]; pixels: Uint8ClampedArray[] } {
  const slots: number[] = [];
  const pixels: Uint8ClampedArray[] = [];
  for (const slot of textureSlots(layout, pos, size)) {
    const next = tile(after, slot.x, slot.y, size);
    if (equal(tile(before, slot.x, slot.y, size), next)) continue;
    slots.push(slot.slot);
    pixels.push(next);
  }
  return { slots, pixels };
}

export function hexToRGBA(hex: string): RGBA {
  const value = parseInt(hex.slice(1), 16);
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255, 255];
}

export function rgbaToHex(color: RGBA): string {
  return "#" + ((color[0] << 16) | (color[1] << 8) | color[2]).toString(16).padStart(6, "0");
}
