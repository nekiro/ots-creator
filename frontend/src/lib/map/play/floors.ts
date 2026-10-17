// The floors the client draws around the player (OTClient MapView
// calcFirstVisibleFloor / calcLastVisibleFloor): from a few floors down to
// the floor above which a roof or a floor covers the player.
import { GROUND_FLOOR, MAX_FLOOR } from "../constants";
import type { Position } from "./player";

/** First floor under ground (setup.otml underground-floor). */
const UNDERGROUND_FLOOR = 8;
/** Floors seen above and below the player under ground (aware-underground-floor-range). */
const UNDERGROUND_RANGE = 2;

/** Client flags of a map item the floor rules read. */
export interface FloorItemFlags {
  ground: boolean;
  /** Ground border, on bottom and on top items come before the common ones in the client's stack. */
  border: boolean;
  bottom: boolean;
  top: boolean;
  dontHide: boolean;
  blockProjectile: boolean;
}

/** The map as the floor rules see it: the items of a tile, bottom to top (undefined: no tile, null: not loaded yet). */
export interface FloorWorld {
  items(x: number, y: number, z: number): number[] | undefined | null;
  flags(id: number): FloorItemFlags;
}

/** Map::isLookPossible: a tile without items that block projectiles. */
function isLookPossible(items: number[] | undefined | null, world: FloorWorld): boolean {
  return !!items && !items.some((id) => world.flags(id).blockProjectile);
}

/** Tile::limitsFloorsView: a ground, or an on-bottom item (that blocks projectiles unless the view is free), hides the floors above. */
function limitsFloorsView(world: FloorWorld, items: number[], isFreeView: boolean): boolean {
  for (const id of items) {
    const flags = world.flags(id);
    // Common items come last in the client's stack: the loop stops at them.
    if (!flags.ground && !flags.border && !flags.top && !flags.bottom) continue;
    if (!flags.dontHide && (flags.ground || (isFreeView ? flags.bottom : flags.bottom && flags.blockProjectile))) return true;
  }
  return false;
}

/** MapView::calcFirstVisibleFloor (floors are always limited by what covers the player).
 * Null while a tile it reads is not loaded: an unloaded roof would show the floors above for a frame. */
export function firstVisibleFloor(world: FloorWorld, camera: Position): number | null {
  let firstFloor = 0;
  let unloaded = false;
  const items = (x: number, y: number, z: number) => {
    const tileItems = world.items(x, y, z);
    if (tileItems === null) unloaded = true;
    return tileItems;
  };
  if (camera.z > GROUND_FLOOR) firstFloor = Math.max(camera.z - UNDERGROUND_RANGE, UNDERGROUND_FLOOR);
  for (let offsetX = -1; offsetX <= 1 && firstFloor < camera.z; offsetX++) {
    for (let offsetY = -1; offsetY <= 1 && firstFloor < camera.z; offsetY++) {
      const x = camera.x + offsetX;
      const y = camera.y + offsetY;
      const lookPossible = isLookPossible(items(x, y, camera.z), world);
      if (!((offsetX === 0 && offsetY === 0) || (Math.abs(offsetX) !== Math.abs(offsetY) && lookPossible))) continue;
      // Straight up, and up the diagonal the client draws upper floors along.
      let upperZ = camera.z;
      let coveredX = x;
      let coveredY = y;
      let coveredZ = camera.z;
      while (coveredZ > 0 && upperZ > 0) {
        coveredX++;
        coveredY++;
        coveredZ--;
        upperZ--;
        if (upperZ < firstFloor) break;
        const upper = items(x, y, upperZ);
        if (upper && limitsFloorsView(world, upper, !lookPossible)) {
          firstFloor = upperZ + 1;
          break;
        }
        const covered = items(coveredX, coveredY, coveredZ);
        if (covered && limitsFloorsView(world, covered, lookPossible)) {
          firstFloor = coveredZ + 1;
          break;
        }
      }
    }
  }
  if (unloaded) return null;
  return Math.min(Math.max(firstFloor, 0), MAX_FLOOR);
}

/** MapView::calcLastVisibleFloor: the ground floor above ground, two floors down under it. */
export function lastVisibleFloor(cameraZ: number): number {
  return Math.min(Math.max(cameraZ > GROUND_FLOOR ? cameraZ + UNDERGROUND_RANGE : GROUND_FLOOR, 0), MAX_FLOOR);
}
