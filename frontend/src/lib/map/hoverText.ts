// The status line text of the tile under the mouse.
import { fmtPos, type HouseInfo, type Position } from "./api";
import { TileFlags, type TileData } from "./chunks";

/** "x, y, z · top item (id) · creatures · house · PZ". */
export function describeTile(position: Position, tile: TileData | undefined, itemName: (id: number) => string, houses: HouseInfo[] | null | undefined): string {
  let text = fmtPos(position);
  if (!tile) return text;
  const top = tile.ids.length ? tile.ids[tile.ids.length - 1] : 0;
  if (top) text += ` · ${itemName(top)} (${top})`;
  if (tile.creatures?.length) text += ` · ${tile.creatures.map((creature) => creature.name).join(", ")}`;
  if (tile.house) {
    const house = houses?.find((candidate) => candidate.id === tile.house);
    text += ` · house ${house?.name ?? tile.house}`;
  }
  if (tile.flags & TileFlags.pz) text += " · PZ";
  return text;
}
