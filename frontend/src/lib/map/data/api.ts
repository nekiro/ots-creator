// Typed access to the map editor service and its resources.
export * as MapService from "../../../../bindings/github.com/nekiro/ots-creator/internal/app/mapservice";
export type {
  Appearance,
  BrushPreview,
  DataInfo,
  FindResult as FindResponse,
  ItemTable,
  MapChange,
  MapDataConfig,
  MapInfo,
  MapStatus,
  MaterialsResult,
  NewMapOptions,
  OpenResult,
  Palette,
  PaletteBrush,
  PaletteTileset,
  RecentMap,
  TileCreature,
  TileInfo,
  TileItem,
} from "../../../../bindings/github.com/nekiro/ots-creator/internal/app/models";
export type {
  ActionInfo,
  Clip,
  ClipTile,
  CreaturePatch,
  DoodadBuffer,
  DoodadTile,
  DrawRequest,
  FindQuery,
  FindResult,
  HouseInfo,
  ImportOptions,
  ItemPatch,
  ItemRef,
  Properties,
  Rect,
  Settings as EditorSettings,
  Statistics,
  TilePatch,
} from "../../../../bindings/github.com/nekiro/ots-creator/internal/mapping/mapedit/models";
export type { Attr, Creature, House, Item, ItemExt, Position, Spawn, Town, Waypoint, Zone } from "../../../../bindings/github.com/nekiro/ots-creator/internal/mapping/otmap/models";
export { SpawnFormat } from "../../../../bindings/github.com/nekiro/ots-creator/internal/mapping/otmap/models";
export type {
  Border,
  BorderRule,
  Brush,
  Chance,
  CreatureType,
  ItemSpec,
  Outfit,
  Tileset,
  WallPart,
  Door,
  Alternative,
  Composite,
  CompositeTile,
  Single,
  DoodadOptions,
} from "../../../../bindings/github.com/nekiro/ots-creator/internal/mapping/materials/models";

import type { Position } from "../../../../bindings/github.com/nekiro/ots-creator/internal/mapping/otmap/models";

/** Tile map flags (OTBM). */
export const TILE_FLAGS = { pz: 1, noPvp: 4, noLogout: 8, pvpZone: 16, refresh: 32 } as const;

export const res = {
  chunks: (map: number, keys: number[], rev: number) => `/res/map/${map}/chunks?k=${keys.join(",")}&r=${rev}`,
  geo: (map: number, keys: number[], opts: number, house: number, block: number) =>
    `/res/map/${map}/geo?k=${keys.join(",")}&o=${opts}&h=${house}&b=${block}`,
  minimap: (map: number, x: number, y: number, z: number, w: number, h: number, rev: number) =>
    `/res/map/${map}/minimap?x=${x}&y=${y}&z=${z}&w=${w}&h=${h}&r=${rev}`,
};

export const pos = (x: number, y: number, z: number): Position => ({ x, y, z });

export function samePos(first: Position | null | undefined, second: Position | null | undefined): boolean {
  return !!first && !!second && first.x === second.x && first.y === second.y && first.z === second.z;
}

export function fmtPos(position: Position | null | undefined): string {
  return position ? `${position.x}, ${position.y}, ${position.z}` : "–";
}

/** Parses "x, y, z" (any separators); null when it is not a position. */
export function parsePos(text: string): Position | null {
  const match = text.match(/(\d+)\D+(\d+)\D+(\d+)/);
  if (!match) return null;
  const [x, y, z] = [Number(match[1]), Number(match[2]), Number(match[3])];
  if (x > 65535 || y > 65535 || z > 15) return null;
  return { x, y, z };
}
