// Tile and world coordinates of the map view. The client draws floors
// above ground shifted up and left by their distance to the ground floor
// (underground: to the floor in view), so tile (x, y) of floor z is drawn
// at world tile (x - shift, y - shift).
import type { Position } from "../data/api";
import { GROUND_FLOOR } from "../constants";

/** Tiles floor z is shifted when the view is on `floor`. */
export function floorShift(z: number, floor: number): number {
  return floor <= GROUND_FLOOR ? GROUND_FLOOR - z : floor - z;
}

/** Shift of the floor in view itself: 0 underground. */
export function floorOffset(z: number): number {
  return floorShift(z, z);
}

/** Tile under a world position on the floor in view. */
export function worldToTile(worldX: number, worldY: number, floor: number, tileSize: number): { x: number; y: number } {
  const offset = floorOffset(floor);
  return { x: Math.floor(worldX / tileSize) + offset, y: Math.floor(worldY / tileSize) + offset };
}

/** World position of the center of a tile, seen from its own floor. */
export function tileToWorld(position: Position, tileSize: number): { x: number; y: number } {
  const offset = floorOffset(position.z);
  return { x: (position.x - offset + 0.5) * tileSize, y: (position.y - offset + 0.5) * tileSize };
}
