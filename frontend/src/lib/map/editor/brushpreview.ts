import type { BrushPreview } from "../data/api";

type Tile = { x: number; y: number; z?: number; items?: ({ id: number } | null)[] | null } | null;
export type Frame = { x: number; y: number; width: number; height: number };

export function frameOf(tiles: Tile[] | null | undefined, max = 12): Frame | null {
  const flat = (tiles ?? []).filter((tile) => !!tile && !tile.z) as NonNullable<Tile>[];
  if (!flat.length) return null;
  const x = Math.min(...flat.map((tile) => tile.x));
  const y = Math.min(...flat.map((tile) => tile.y));
  return { x, y, width: Math.min(max, Math.max(...flat.map((tile) => tile.x)) - x + 1), height: Math.min(max, Math.max(...flat.map((tile) => tile.y)) - y + 1) };
}

export function previewOf(tiles: Tile[] | null | undefined, frame: Frame | null = frameOf(tiles)): BrushPreview | null {
  if (!frame) return null;
  const flat = (tiles ?? []).filter((tile) => !!tile && !tile.z) as NonNullable<Tile>[];
  return {
    w: frame.width,
    h: frame.height,
    tiles: flat
      .map((tile) => ({ x: tile.x - frame.x, y: tile.y - frame.y, items: (tile.items ?? []).filter((item) => !!item).map((item) => item!.id) }))
      .filter((tile) => tile.x >= 0 && tile.y >= 0 && tile.x < frame.width && tile.y < frame.height),
  };
}
