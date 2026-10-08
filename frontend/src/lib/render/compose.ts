// CPU compositing of one texture (all tiles, optional addons and outfit
// colors) into an RGBA buffer. Textures are small, so this is fast enough
// for 60 fps playback and keeps the logic testable.

import { spriteIndex, tileOffset, type Layout, type TexturePos } from "./layout";
import { colorize, type OutfitColors } from "./outfit";

export interface ComposeOptions {
  pos: TexturePos;
  /** Extra pattern Y values drawn on top (outfit addons). */
  addons?: number[];
  /** Colorize with layer 1 as template (outfits with 2+ layers). */
  colors?: OutfitColors | null;
}

export type PixelSource = (spriteId: number) => Uint8ClampedArray | undefined;

/** Alpha-blends src over dst (straight alpha). */
export function blendOver(dst: Uint8ClampedArray, dOff: number, src: Uint8ClampedArray, sOff: number): void {
  const sa = src[sOff + 3];
  if (sa === 0) return;
  if (sa === 255) {
    dst[dOff] = src[sOff];
    dst[dOff + 1] = src[sOff + 1];
    dst[dOff + 2] = src[sOff + 2];
    dst[dOff + 3] = 255;
    return;
  }
  const a = sa / 255;
  const da = dst[dOff + 3] / 255;
  const oa = a + da * (1 - a);
  for (let k = 0; k < 3; k++) {
    dst[dOff + k] = (src[sOff + k] * a + dst[dOff + k] * da * (1 - a)) / oa;
  }
  dst[dOff + 3] = oa * 255;
}

export function compose(g: Layout, sprites: number[], size: number, opts: ComposeOptions, get: PixelSource): ImageData {
  const w = g.width * size;
  const h = g.height * size;
  const out = new ImageData(w, h);
  const ys = [opts.pos.y, ...(opts.addons ?? []).filter((y) => y !== opts.pos.y && y < g.patternY)];
  const useColors = !!opts.colors && g.layers > 1 && opts.pos.layer === 0;
  const tmp = new Uint8ClampedArray(size * size * 4);
  for (const py of ys) {
    for (let th = 0; th < g.height; th++) {
      for (let tw = 0; tw < g.width; tw++) {
        const p = { ...opts.pos, y: py };
        const base = get(sprites[spriteIndex(g, tw, th, p)] ?? 0);
        if (!base) continue;
        let px = base;
        if (useColors) {
          const tpl = get(sprites[spriteIndex(g, tw, th, { ...p, layer: 1 })] ?? 0);
          if (tpl) {
            tmp.set(base);
            colorize(tmp, tpl, opts.colors!);
            px = tmp;
          }
        }
        const [ox, oy] = tileOffset(g, tw, th, size);
        for (let row = 0; row < size; row++) {
          let d = ((oy + row) * w + ox) * 4;
          let s = row * size * 4;
          for (let col = 0; col < size; col++, d += 4, s += 4) blendOver(out.data, d, px, s);
        }
      }
    }
  }
  return out;
}

/** Stacks images bottom-right aligned (how the client anchors textures),
 * first image at the bottom. Returns the result and each image's offset. */
export function stackBottomRight(images: ImageData[]): { image: ImageData; offsets: [number, number][] } {
  const w = Math.max(0, ...images.map((i) => i.width));
  const h = Math.max(0, ...images.map((i) => i.height));
  const out = new ImageData(Math.max(1, w), Math.max(1, h));
  const offsets: [number, number][] = [];
  for (const img of images) {
    const ox = w - img.width;
    const oy = h - img.height;
    offsets.push([ox, oy]);
    for (let row = 0; row < img.height; row++) {
      let d = ((oy + row) * out.width + ox) * 4;
      let s = row * img.width * 4;
      for (let col = 0; col < img.width; col++, d += 4, s += 4) blendOver(out.data, d, img.data, s);
    }
  }
  return { image: out, offsets };
}
