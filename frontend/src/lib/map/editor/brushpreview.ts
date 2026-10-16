import type { BrushPreview } from "../data/api";

type Tile = { x: number; y: number; z?: number; items?: ({ id: number } | null)[] | null } | null;
export type Frame = { x: number; y: number; w: number; h: number };

/** The area of the ground floor of a group of tiles, at most max a side. */
export function frameOf(tiles: Tile[] | null | undefined, max = 12): Frame | null {
  const flat = (tiles ?? []).filter((tile) => !!tile && !tile.z) as NonNullable<Tile>[];
  if (!flat.length) return null;
  const x = Math.min(...flat.map((tile) => tile.x));
  const y = Math.min(...flat.map((tile) => tile.y));
  return { x, y, w: Math.min(max, Math.max(...flat.map((tile) => tile.x)) - x + 1), h: Math.min(max, Math.max(...flat.map((tile) => tile.y)) - y + 1) };
}

/** The ground floor of a group of tiles as a preview (same as the palette's), in frame (default: its own area). */
export function previewOf(tiles: Tile[] | null | undefined, frame: Frame | null = frameOf(tiles)): BrushPreview | null {
  if (!frame) return null;
  const flat = (tiles ?? []).filter((tile) => !!tile && !tile.z) as NonNullable<Tile>[];
  return {
    w: frame.w,
    h: frame.h,
    tiles: flat
      .map((tile) => ({ x: tile.x - frame.x, y: tile.y - frame.y, items: (tile.items ?? []).filter((item) => !!item).map((item) => item!.id) }))
      .filter((tile) => tile.x >= 0 && tile.y >= 0 && tile.x < frame.w && tile.y < frame.h),
  };
}
