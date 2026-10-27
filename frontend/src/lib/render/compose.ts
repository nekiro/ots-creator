// CPU compositing of one texture (all tiles, optional addons and outfit
// colors) into an RGBA buffer. Textures are small, so this is fast enough
// for 60 fps playback and keeps the logic testable.

import { spriteIndex, tileOffset, type Layout, type TexturePos } from "./layout";
import { colorize, type OutfitColors } from "./outfit";

export interface ComposeOptions {
  pos: TexturePos;
  addons?: number[];
  colors?: OutfitColors | null;
}

export type PixelSource = (spriteId: number) => Uint8ClampedArray | undefined;

export function blendOver(dst: Uint8ClampedArray, dstOffset: number, src: Uint8ClampedArray, srcOffset: number): void {
  const srcAlpha8 = src[srcOffset + 3];
  if (srcAlpha8 === 0) return;
  if (srcAlpha8 === 255) {
    dst[dstOffset] = src[srcOffset];
    dst[dstOffset + 1] = src[srcOffset + 1];
    dst[dstOffset + 2] = src[srcOffset + 2];
    dst[dstOffset + 3] = 255;
    return;
  }
  const srcAlpha = srcAlpha8 / 255;
  const dstAlpha = dst[dstOffset + 3] / 255;
  const outAlpha = srcAlpha + dstAlpha * (1 - srcAlpha);
  for (let channel = 0; channel < 3; channel++) {
    dst[dstOffset + channel] = (src[srcOffset + channel] * srcAlpha + dst[dstOffset + channel] * dstAlpha * (1 - srcAlpha)) / outAlpha;
  }
  dst[dstOffset + 3] = outAlpha * 255;
}

export function compose(layout: Layout, sprites: number[], size: number, opts: ComposeOptions, get: PixelSource): ImageData {
  const outWidth = layout.width * size;
  const out = new ImageData(outWidth, layout.height * size);
  const patternYs = [opts.pos.y, ...(opts.addons ?? []).filter((y) => y !== opts.pos.y && y < layout.patternY)];
  const useColors = !!opts.colors && layout.layers > 1 && opts.pos.layer === 0;
  const colored = new Uint8ClampedArray(size * size * 4);
  for (const patternY of patternYs) {
    for (let tileY = 0; tileY < layout.height; tileY++) {
      for (let tileX = 0; tileX < layout.width; tileX++) {
        const texturePos = { ...opts.pos, y: patternY };
        const base = get(sprites[spriteIndex(layout, tileX, tileY, texturePos)] ?? 0);
        if (!base) continue;
        let pixels = base;
        if (useColors) {
          const template = get(sprites[spriteIndex(layout, tileX, tileY, { ...texturePos, layer: 1 })] ?? 0);
          if (template) {
            colored.set(base);
            colorize(colored, template, opts.colors!);
            pixels = colored;
          }
        }
        const [offsetX, offsetY] = tileOffset(layout, tileX, tileY, size);
        for (let row = 0; row < size; row++) {
          let dstOffset = ((offsetY + row) * outWidth + offsetX) * 4;
          let srcOffset = row * size * 4;
          for (let col = 0; col < size; col++, dstOffset += 4, srcOffset += 4) blendOver(out.data, dstOffset, pixels, srcOffset);
        }
      }
    }
  }
  return out;
}

/** Stacks images bottom-right aligned (how the client anchors textures),
 * first image at the bottom. Returns the result and each image's offset. */
export function stackBottomRight(images: ImageData[]): { image: ImageData; offsets: [number, number][] } {
  const width = Math.max(0, ...images.map((image) => image.width));
  const height = Math.max(0, ...images.map((image) => image.height));
  const out = new ImageData(Math.max(1, width), Math.max(1, height));
  const offsets: [number, number][] = [];
  for (const image of images) {
    const offsetX = width - image.width;
    const offsetY = height - image.height;
    offsets.push([offsetX, offsetY]);
    for (let row = 0; row < image.height; row++) {
      let dstOffset = ((offsetY + row) * out.width + offsetX) * 4;
      let srcOffset = row * image.width * 4;
      for (let col = 0; col < image.width; col++, dstOffset += 4, srcOffset += 4) blendOver(out.data, dstOffset, image.data, srcOffset);
    }
  }
  return { image: out, offsets };
}
