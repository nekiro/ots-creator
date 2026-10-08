// Frame group geometry shared by the preview and the sprite slot editor.
// Mirrors internal/thing/thing.go.

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

export function totalSprites(g: Layout): number {
  return g.width * g.height * g.layers * g.patternX * g.patternY * g.patternZ * g.frames;
}

export function spriteIndex(g: Layout, w: number, h: number, p: TexturePos): number {
  const frame = p.frame % Math.max(g.frames, 1);
  return ((((((frame * g.patternZ + p.z) * g.patternY + p.y) * g.patternX + p.x) * g.layers + p.layer) * g.height + h) * g.width) + w;
}

/** Pixel offset of tile (w, h) inside a texture; tile (0,0) is bottom-right. */
export function tileOffset(g: Layout, w: number, h: number, size: number): [number, number] {
  return [(g.width - 1 - w) * size, (g.height - 1 - h) * size];
}

/** All sprite slot indices of one texture with their pixel offsets. */
export function textureSlots(g: Layout, p: TexturePos, size: number): { slot: number; x: number; y: number }[] {
  const out: { slot: number; x: number; y: number }[] = [];
  for (let h = 0; h < g.height; h++) {
    for (let w = 0; w < g.width; w++) {
      const [x, y] = tileOffset(g, w, h, size);
      out.push({ slot: spriteIndex(g, w, h, p), x, y });
    }
  }
  return out;
}

export function clampPos(g: Layout, p: TexturePos): TexturePos {
  const c = (v: number, n: number) => Math.min(Math.max(v, 0), Math.max(n - 1, 0));
  return { layer: c(p.layer, g.layers), x: c(p.x, g.patternX), y: c(p.y, g.patternY), z: c(p.z, g.patternZ), frame: c(p.frame, g.frames) };
}

/**
 * Resize a layout keeping sprites that still have valid coordinates.
 * Mirrors FrameGroup.Resize in Go.
 */
export function resizeSprites(old: Layout, sprites: number[], next: Layout): number[] {
  const out = new Array<number>(totalSprites(next)).fill(0);
  const m = (a: number, b: number) => Math.min(a, b);
  for (let f = 0; f < m(old.frames, next.frames); f++)
    for (let z = 0; z < m(old.patternZ, next.patternZ); z++)
      for (let y = 0; y < m(old.patternY, next.patternY); y++)
        for (let x = 0; x < m(old.patternX, next.patternX); x++)
          for (let l = 0; l < m(old.layers, next.layers); l++)
            for (let h = 0; h < m(old.height, next.height); h++)
              for (let w = 0; w < m(old.width, next.width); w++) {
                const p = { layer: l, x, y, z, frame: f };
                out[spriteIndex(next, w, h, p)] = sprites[spriteIndex(old, w, h, p)] ?? 0;
              }
  return out;
}

/** Sheet grid of a frame group, ObjectBuilder layout: columns are
 * pattern Z × pattern X × layers, rows are frames × pattern Y. */
export function sheetGrid(g: Layout): { columns: number; rows: number } {
  return { columns: g.patternZ * g.patternX * g.layers, rows: g.frames * g.patternY };
}

/** Texture shown at a sheet cell. */
export function sheetCellPos(g: Layout, column: number, row: number): TexturePos {
  return {
    layer: column % g.layers,
    x: Math.floor(column / g.layers) % g.patternX,
    z: Math.floor(column / (g.layers * g.patternX)),
    y: row % g.patternY,
    frame: Math.floor(row / g.patternY),
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
