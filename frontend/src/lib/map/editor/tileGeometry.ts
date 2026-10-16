// Tile shapes of brushes and strokes, and the tile under a screen point.
import { pos, type Position } from "../data/api";
import { worldToPosition } from "../view/coords";

/** Tiles a brush of `size` (tiles from the center) covers around center; none below 0. */
export function brushArea(center: Position, size: number, shape: string): Position[] {
  const out: Position[] = [];
  for (let dy = -size; dy <= size; dy++) {
    for (let dx = -size; dx <= size; dx++) {
      if (shape === "circle" && Math.sqrt(dx * dx + dy * dy) >= size + 0.005) continue;
      if (center.x + dx < 0 || center.y + dy < 0) continue;
      out.push(pos(center.x + dx, center.y + dy, center.z));
    }
  }
  return out;
}

/** The outline of a square brush (a wall brush draws only its border). */
export function squareOutline(center: Position, size: number): Position[] {
  return brushArea(center, size, "square").filter((position) => Math.abs(position.x - center.x) === size || Math.abs(position.y - center.y) === size);
}

/** Tiles on the line from one tile to another, without the first (strokes never skip tiles). */
export function bresenhamLine(from: Position, to: Position): Position[] {
  const out: Position[] = [];
  let x = from.x;
  let y = from.y;
  const dx = Math.abs(to.x - from.x);
  const dy = -Math.abs(to.y - from.y);
  const stepX = from.x < to.x ? 1 : -1;
  const stepY = from.y < to.y ? 1 : -1;
  let error = dx + dy;
  for (;;) {
    if (x !== from.x || y !== from.y) out.push(pos(x, y, from.z));
    if (x === to.x && y === to.y) break;
    const doubled = 2 * error;
    if (doubled >= dy) {
      error += dy;
      x += stepX;
    }
    if (doubled <= dx) {
      error += dx;
      y += stepY;
    }
  }
  return out;
}

export interface ViewTransform {
  /** Canvas size in device pixels. */
  width: number;
  height: number;
  /** Camera center in world pixels. */
  camX: number;
  camY: number;
  /** Device pixels per world pixel. */
  scale: number;
  floor: number;
  tileSize: number;
}

/** Tile of the floor in view under a canvas point in device pixels (never below 0). */
export function screenToTile(x: number, y: number, view: ViewTransform): Position {
  const worldX = (x - view.width / 2) / view.scale + view.camX;
  const worldY = (y - view.height / 2) / view.scale + view.camY;
  return worldToPosition(worldX, worldY, view.floor, view.tileSize);
}
