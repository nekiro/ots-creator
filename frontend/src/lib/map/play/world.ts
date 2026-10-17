// The loaded map as the play rules see it: what can be walked on and what
// limits the floors in view.
import { AF, type Appearances } from "../data/appearances";
import { TileFlags, type ChunkStore } from "../data/chunks";
import { TOP_ORDER, type ItemIndex } from "../data/items";
import type { FloorItemFlags, FloorWorld } from "./floors";
import type { WalkWorld } from "./player";

/** Walking: a tile with a ground and nothing that blocks (client Tile::isWalkable). Creatures of
 * the map stand where they spawn, not where they are in game: they do not block. */
export function walkWorld(chunks: ChunkStore, items: ItemIndex): WalkWorld {
  return {
    isWalkable(x, y, z) {
      const tile = chunks.tile(x, y, z);
      return !!tile && (tile.flags & TileFlags.ground) !== 0 && (tile.flags & TileFlags.blocking) === 0;
    },
    groundSpeed(x, y, z) {
      const tile = chunks.peekTile(x, y, z);
      if (!tile || !(tile.flags & TileFlags.ground)) return null;
      return items.speed[tile.ids[0]] ?? 0;
    },
  };
}

/** The floor rules: items of tiles (requested when not loaded) and their client flags. */
export function floorWorld(chunks: ChunkStore, items: ItemIndex, looks: Appearances): FloorWorld {
  return {
    items(x, y, z) {
      const tile = chunks.tile(x, y, z);
      return tile ? Array.from(tile.ids) : undefined;
    },
    flags(id): FloorItemFlags {
      const look = looks.items.get(items.client[id]);
      const top = items.top[id];
      return {
        ground: items.is(id, "ground"),
        border: top === TOP_ORDER.border,
        bottom: top === TOP_ORDER.bottom,
        top: top === TOP_ORDER.top,
        dontHide: !!look && (look.flags & AF.dontHide) !== 0,
        blockProjectile: items.is(id, "blockMissile"),
      };
    },
  };
}
