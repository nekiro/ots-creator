// Frame group geometry shared by the preview and the sprite slot editor.
// Mirrors internal/core/thing/thing.go.
import { clamp } from "../util/math";

export interface Layout {
  width: number;
  height: number;
  layers: number;
  patternX: number;
  patternY: number;
  patternZ: number;
  frames: number;
}

export interface TexturePos {
  layer: number;
  x: number;
  y: number;
  z: number;
  frame: number;
}

export function totalSprites(layout: Layout): number {
  return layout.width * layout.height * layout.layers * layout.patternX * layout.patternY * layout.patternZ * layout.frames;
}

export function spriteIndex(layout: Layout, tileX: number, tileY: number, pos: TexturePos): number {
  const frame = pos.frame % Math.max(layout.frames, 1);
  return ((((((frame * layout.patternZ + pos.z) * layout.patternY + pos.y) * layout.patternX + pos.x) * layout.layers + pos.layer) * layout.height + tileY) * layout.width) + tileX;
}

/** Pixel offset of tile (tileX, tileY) inside a texture; tile (0,0) is bottom-right. */
export function tileOffset(layout: Layout, tileX: number, tileY: number, size: number): [number, number] {
  return [(layout.width - 1 - tileX) * size, (layout.height - 1 - tileY) * size];
}

export function textureSlots(layout: Layout, pos: TexturePos, size: number): { slot: number; x: number; y: number }[] {
  const out: { slot: number; x: number; y: number }[] = [];
  for (let tileY = 0; tileY < layout.height; tileY++) {
    for (let tileX = 0; tileX < layout.width; tileX++) {
      const [x, y] = tileOffset(layout, tileX, tileY, size);
      out.push({ slot: spriteIndex(layout, tileX, tileY, pos), x, y });
    }
  }
  return out;
}

export function clampPos(layout: Layout, pos: TexturePos): TexturePos {
  const inRange = (value: number, count: number) => clamp(value, 0, Math.max(count - 1, 0));
  return {
    layer: inRange(pos.layer, layout.layers),
    x: inRange(pos.x, layout.patternX),
    y: inRange(pos.y, layout.patternY),
    z: inRange(pos.z, layout.patternZ),
    frame: inRange(pos.frame, layout.frames),
  };
}

/**
 * Resize a layout keeping sprites that still have valid coordinates.
 * Mirrors FrameGroup.Resize in Go.
 */
export function resizeSprites(old: Layout, sprites: number[], next: Layout): number[] {
  const out = new Array<number>(totalSprites(next)).fill(0);
  const kept = (key: keyof Layout) => Math.min(old[key], next[key]);
  for (let frame = 0; frame < kept("frames"); frame++)
    for (let z = 0; z < kept("patternZ"); z++)
      for (let y = 0; y < kept("patternY"); y++)
        for (let x = 0; x < kept("patternX"); x++)
          for (let layer = 0; layer < kept("layers"); layer++)
            for (let tileY = 0; tileY < kept("height"); tileY++)
              for (let tileX = 0; tileX < kept("width"); tileX++) {
                const pos = { layer, x, y, z, frame };
                out[spriteIndex(next, tileX, tileY, pos)] = sprites[spriteIndex(old, tileX, tileY, pos)] ?? 0;
              }
  return out;
}

/** Sheet grid of a frame group, ObjectBuilder layout: columns are
 * pattern Z × pattern X × layers, rows are frames × pattern Y. */
export function sheetGrid(layout: Layout): { columns: number; rows: number } {
  return { columns: layout.patternZ * layout.patternX * layout.layers, rows: layout.frames * layout.patternY };
}

export function sheetCellPos(layout: Layout, column: number, row: number): TexturePos {
  return {
    layer: column % layout.layers,
    x: Math.floor(column / layout.layers) % layout.patternX,
    z: Math.floor(column / (layout.layers * layout.patternX)),
    y: row % layout.patternY,
    frame: Math.floor(row / layout.patternY),
  };
}

/**
 * Writes ids into consecutive sprite slots from `start` (slot order is the
 * order clients store sprites, so a range of sprite ids lands where it
 * belongs). Ids past the last slot are ignored. Returns the new array.
 */
export function fillSlots(sprites: number[], start: number, ids: number[]): number[] {
  const out = [...sprites];
  for (let i = 0; i < ids.length && start + i < out.length; i++) out[start + i] = ids[i];
  return out;
}
