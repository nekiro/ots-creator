// Where the map is on screen in a frame: tile positions in device pixels,
// the tiles in view and the floors drawn.
import { GROUND_FLOOR, LIMITS, MAX_FLOOR } from "./constants";
import { floorShift } from "./coords";
import type { Scene } from "./scene";

/** A rectangle of tiles (inclusive). */
export interface TileRange {
  x0: number;
  y0: number;
  x1: number;
  y1: number;
}

export class Camera {
  /** Device pixels per tile. */
  readonly tilePx: number;
  /** Tiles of one floor in view, with a margin. */
  readonly visibleTiles: number;
  /** Half the view size in tiles. */
  readonly halfWidthTiles: number;
  readonly halfHeightTiles: number;
  /** Half the view size in device pixels. */
  private halfWidthPx: number;
  private halfHeightPx: number;

  constructor(private scene: Scene) {
    this.tilePx = scene.spriteSize * scene.scale;
    this.halfWidthPx = scene.width / 2;
    this.halfHeightPx = scene.height / 2;
    this.halfWidthTiles = scene.width / 2 / scene.scale / scene.spriteSize;
    this.halfHeightTiles = scene.height / 2 / scene.scale / scene.spriteSize;
    this.visibleTiles = ((scene.width / this.tilePx + 6) * (scene.height / this.tilePx + 6)) | 0;
  }

  /** Tiles floor z is drawn up and left of its position, as the client
   * stacks floors (relative to the ground floor, or underground to the
   * floor in view). */
  shift(z: number): number {
    return floorShift(z, this.scene.floor);
  }

  /** Screen x of the left edge of tile x on floor z. */
  tileX(x: number, z: number): number {
    const scene = this.scene;
    return Math.round(((x - this.shift(z)) * scene.spriteSize - scene.camX) * scene.scale + this.halfWidthPx);
  }

  /** Screen y of the top edge of tile y on floor z. */
  tileY(y: number, z: number): number {
    const scene = this.scene;
    return Math.round(((y - this.shift(z)) * scene.spriteSize - scene.camY) * scene.scale + this.halfHeightPx);
  }

  /** Tiles in view on floor z, widened by `before` tiles up and left and `after` tiles down and right. */
  tiles(z: number, before: number, after: number): TileRange {
    const scene = this.scene;
    const shift = this.shift(z);
    return {
      x0: Math.floor((scene.camX - this.halfWidthPx / scene.scale) / scene.spriteSize + shift) - before,
      y0: Math.floor((scene.camY - this.halfHeightPx / scene.scale) / scene.spriteSize + shift) - before,
      x1: Math.ceil((scene.camX + this.halfWidthPx / scene.scale) / scene.spriteSize + shift) + after,
      y1: Math.ceil((scene.camY + this.halfHeightPx / scene.scale) / scene.spriteSize + shift) + after,
    };
  }

  /** The view center on floor z in tiles (fractional). */
  centerX(z: number): number {
    return this.scene.camX / this.scene.spriteSize + this.shift(z);
  }

  centerY(z: number): number {
    return this.scene.camY / this.scene.spriteSize + this.shift(z);
  }
}

/** Floors to draw, bottom up: the floor in view and, with lowerFloors, the
 * floors under it down to the ground floor (underground: two floors down).
 * Too many tiles in all leaves out the lowest floors. */
export function floorsToDraw(scene: Scene, visibleTiles: number): number[] {
  const floor = scene.floor;
  let floors: number[] = [];
  if (floor <= GROUND_FLOOR) for (let z = scene.opts.lowerFloors ? GROUND_FLOOR : floor; z >= floor; z--) floors.push(z);
  else for (let z = scene.opts.lowerFloors ? Math.min(MAX_FLOOR, floor + 2) : floor; z >= floor; z--) floors.push(z);
  if (floors.length * visibleTiles > LIMITS.floorTiles) {
    const kept = Math.max(1, Math.floor(LIMITS.floorTiles / visibleTiles));
    // Keep the floor in view and the floors right under it.
    const nearestFirst = [floor, ...floors.filter((z) => z !== floor).sort((a, b) => a - b)];
    floors = nearestFirst.slice(0, kept).sort((a, b) => b - a);
  }
  return floors;
}
