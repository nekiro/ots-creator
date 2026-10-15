// Overlays drawn over the map every frame in screen pixels: grid, spawn
// radiuses, selection and area rectangles, markers, the client view box,
// and labels (creature names, item texts, positions, item tooltips).
import { IF, TF } from "./chunks";
import { labelText, wrapText } from "./itemText";
import type { QuadBatch } from "./quadBatch";
import type { FrameResult, Scene } from "./scene";
import { TILES_TILE_PX, type Camera } from "./view";

/** Smallest tile size in device pixels with grid lines, marker names and other labels. */
const GRID_TILE_PX = 8;
const MARKER_LABEL_TILE_PX = 12;
const LABEL_TILE_PX = 24;
/** Tiles around the view searched for spawns whose radius reaches into it. */
const SPAWN_SEARCH = 30;

const GRID_COLOR = [1, 1, 1, 0.12];
const SPAWN_SELECTED = [1, 1, 0.4, 0.9];
const SPAWN_NPC = [0.4, 0.7, 1, 0.7];
const SPAWN_MONSTER = [1, 0.45, 0.45, 0.7];
/** Alpha of the spawn center tile and of marker outlines. */
const SPAWN_CENTER_ALPHA = 0.25;
const MARKER_ALPHA = 0.9;
const CLIENT_BOX_COLOR = [1, 1, 1, 0.6];
const HOVER_TILE = [1, 1, 1, 0.12];
const POSITION_TAG = [0, 0, 0, 0.75];
const CREATURE_NAME_COLOR = "#00c000";
const ITEM_TEXT_COLOR = "#e0d29a";
const POSITION_TEXT_COLOR = "#e8e8e8";
const TOOLTIP_COLOR = "#ffd75e";

/** What every overlay draws with in a frame. */
interface Overlay {
  scene: Scene;
  result: FrameResult;
  camera: Camera;
  batch: QuadBatch;
  /** The floor in view. */
  z: number;
  tilePx: number;
  /** Outline width in device pixels. */
  lineWidth: number;
}

export function drawOverlays(scene: Scene, result: FrameResult, camera: Camera, batch: QuadBatch): void {
  const overlay: Overlay = { scene, result, camera, batch, z: scene.floor, tilePx: camera.tilePx, lineWidth: Math.max(1, Math.round(scene.scale)) };
  const { opts } = scene;
  const tilePx = camera.tilePx;
  const near = !result.far;
  if (opts.grid && tilePx >= GRID_TILE_PX) drawGrid(overlay);
  if (opts.spawns && tilePx >= TILES_TILE_PX) drawSpawns(overlay);
  drawRects(overlay);
  if (opts.markers) drawMarkers(overlay);
  if (opts.clientBox) drawClientBox(overlay);
  if (opts.creatures && opts.labels && tilePx >= LABEL_TILE_PX && near) labelCreatures(overlay);
  if (opts.labels && tilePx >= LABEL_TILE_PX && near) labelItemTexts(overlay);
  if (opts.coords && scene.hover && scene.hover.z === overlay.z) labelHoverPosition(overlay);
  if (opts.tooltips && tilePx >= LABEL_TILE_PX && near) labelItemTooltips(overlay);
}

/** Calls visit for the tiles of the floor in view, from `before` tiles up
 * and left of the view to `after` tiles down and right of it. */
function forEachTile(overlay: Overlay, before: number, after: number, visit: (x: number, y: number) => void): void {
  const { camera, z } = overlay;
  const halfWidth = camera.halfWidthTiles;
  const halfHeight = camera.halfHeightTiles;
  const centerX = camera.centerX(z);
  const centerY = camera.centerY(z);
  for (let y = Math.max(0, Math.floor(centerY - halfHeight - before)); y <= centerY + halfHeight + after; y++) {
    for (let x = Math.max(0, Math.floor(centerX - halfWidth - before)); x <= centerX + halfWidth + after; x++) visit(x, y);
  }
}

function isHovered(scene: Scene, x: number, y: number, z: number): boolean {
  return !!scene.hover && scene.hover.x === x && scene.hover.y === y && scene.hover.z === z;
}

/** Device pixels per CSS pixel, for label sizes. */
function pixelRatio(): number {
  return Math.max(1, Math.round(globalThis.devicePixelRatio || 1));
}

function drawGrid({ scene, camera, batch, z }: Overlay): void {
  const range = camera.tiles(z, 0, 0);
  for (let x = range.x0; x <= range.x1; x++) batch.rect(camera.tileX(x, z), 0, 1, scene.height, GRID_COLOR);
  for (let y = range.y0; y <= range.y1; y++) batch.rect(0, camera.tileY(y, z), scene.width, 1, GRID_COLOR);
}

/** Spawn radius outlines of the current floor. */
function drawSpawns(overlay: Overlay): void {
  const { scene, camera, batch, z, tilePx, lineWidth } = overlay;
  forEachTile(overlay, SPAWN_SEARCH, SPAWN_SEARCH, (x, y) => {
    const tile = scene.chunks.peekTile(x, y, z);
    if (!tile || !(tile.flags & (TF.spawn | TF.spawnNpc))) return;
    for (const spawn of [tile.spawn, tile.spawnNpc]) {
      if (!spawn) continue;
      const radius = spawn.radius;
      const color = spawn.selected ? SPAWN_SELECTED : tile.spawnNpc === spawn ? SPAWN_NPC : SPAWN_MONSTER;
      const size = (2 * radius + 1) * tilePx;
      batch.stroke(camera.tileX(x - radius, z), camera.tileY(y - radius, z), size, size, color, lineWidth);
      batch.rect(camera.tileX(x, z), camera.tileY(y, z), tilePx, tilePx, [color[0], color[1], color[2], SPAWN_CENTER_ALPHA]);
    }
  });
}

/** Selection and area rectangles. */
function drawRects({ scene, camera, batch, z, tilePx, lineWidth }: Overlay): void {
  for (const mark of scene.rects) {
    if (mark.z !== z) continue;
    const x = camera.tileX(Math.min(mark.x0, mark.x1), z);
    const y = camera.tileY(Math.min(mark.y0, mark.y1), z);
    const width = (Math.abs(mark.x1 - mark.x0) + 1) * tilePx;
    const height = (Math.abs(mark.y1 - mark.y0) + 1) * tilePx;
    if (mark.fill) batch.rect(x, y, width, height, mark.fill);
    if (mark.stroke) batch.stroke(x, y, width, height, mark.stroke, lineWidth);
  }
}

/** Marked positions: an outline on the tile and its name above it. */
function drawMarkers({ scene, result, camera, batch, z, tilePx, lineWidth }: Overlay): void {
  for (const marker of scene.markers) {
    if (marker.z !== z) continue;
    const x = camera.tileX(marker.x, z);
    const y = camera.tileY(marker.y, z);
    const [r, g, b] = marker.color;
    batch.stroke(x + lineWidth, y + lineWidth, tilePx - 2 * lineWidth, tilePx - 2 * lineWidth, [r, g, b, MARKER_ALPHA], Math.max(2, lineWidth * 2));
    if (tilePx >= MARKER_LABEL_TILE_PX && scene.opts.labels) {
      // The whole name under the mouse, the short one elsewhere.
      const hovered = isHovered(scene, marker.x, marker.y, z);
      result.labels.push({
        x: x + tilePx / 2,
        y: y - 2,
        text: hovered ? (marker.full ?? marker.label) : marker.label,
        top: hovered,
        color: `rgb(${Math.round(r * 255)},${Math.round(g * 255)},${Math.round(b * 255)})`,
      });
    }
  }
}

/** The client shows 15x11 tiles around the player (plus one hidden row). */
function drawClientBox({ camera, batch, z, tilePx, lineWidth }: Overlay): void {
  const playerX = Math.floor(camera.centerX(z));
  const playerY = Math.floor(camera.centerY(z));
  batch.stroke(camera.tileX(playerX - 7, z), camera.tileY(playerY - 5, z), 15 * tilePx, 11 * tilePx, CLIENT_BOX_COLOR, Math.max(2, lineWidth));
}

/** Names over creatures as the client shows them: green (full health),
 * centered over the sprite, which is drawn 8 px up and left of its tile. */
function labelCreatures(overlay: Overlay): void {
  const { scene, result, camera, z, tilePx } = overlay;
  const spriteOffset = tilePx / 4;
  forEachTile(overlay, 0, 1, (x, y) => {
    const tile = scene.chunks.peekTile(x, y, z);
    if (!tile?.creatures) return;
    for (const creature of tile.creatures) {
      result.labels.push({ x: camera.tileX(x, z) + tilePx / 2 - spriteOffset, y: camera.tileY(y, z) - spriteOffset - 2, text: creature.name, color: CREATURE_NAME_COLOR });
    }
  });
}

/** Text of signs, books and other readable items: its first line over the
 * tile, the whole text under the mouse. */
function labelItemTexts(overlay: Overlay): void {
  const { scene, result, camera, z, tilePx } = overlay;
  forEachTile(overlay, 0, 1, (x, y) => {
    const text = scene.chunks.peekTile(x, y, z)?.text;
    if (!text) return;
    const centerX = camera.tileX(x, z) + tilePx / 2;
    const centerY = camera.tileY(y, z) + tilePx / 2;
    if (isHovered(scene, x, y, z)) {
      // Line by line, over the other labels.
      const lines = wrapText(text);
      const lineHeight = 11 * pixelRatio();
      lines.forEach((line, i) => result.labels.push({ x: centerX, y: centerY + (i - (lines.length - 1) / 2) * lineHeight, text: line, color: ITEM_TEXT_COLOR, small: true, top: true }));
      return;
    }
    const firstLine = labelText(text);
    if (firstLine) result.labels.push({ x: centerX, y: centerY, text: firstLine, color: ITEM_TEXT_COLOR, small: true, cull: true });
  });
}

/** The position of the tile under the mouse, in a dark tag above it (below it at the top edge). */
function labelHoverPosition({ scene, result, camera, batch, z, tilePx }: Overlay): void {
  const hover = scene.hover!;
  const ratio = pixelRatio();
  const text = `${hover.x}, ${hover.y}, ${hover.z}`;
  const tagWidth = (text.length * 5 + 8) * ratio;
  const tagHeight = 13 * ratio;
  const x = camera.tileX(hover.x, z);
  const y = camera.tileY(hover.y, z);
  const tagX = Math.min(Math.max(0, x + tilePx / 2 - tagWidth / 2), scene.width - tagWidth);
  const tagY = y - tagHeight - 2 * ratio >= 0 ? y - tagHeight - 2 * ratio : y + tilePx + 2 * ratio;
  batch.rect(x, y, tilePx, tilePx, HOVER_TILE);
  batch.rect(tagX, tagY, tagWidth, tagHeight, POSITION_TAG);
  result.labels.push({ x: tagX + tagWidth / 2, y: tagY + tagHeight / 2, text, color: POSITION_TEXT_COLOR, small: true });
}

/** The id of the first item on a tile with an action id, unique id, text,
 * teleport destination or door id, and what it has. */
function labelItemTooltips(overlay: Overlay): void {
  const { scene, result, camera, z, tilePx } = overlay;
  forEachTile(overlay, 0, 0, (x, y) => {
    const tile = scene.chunks.peekTile(x, y, z);
    if (!tile) return;
    for (let i = 0; i < tile.ids.length; i++) {
      const itemFlags = tile.iflags[i];
      if (!(itemFlags & (IF.ids | IF.text | IF.teleport | IF.doorId))) continue;
      const tags: string[] = [];
      if (itemFlags & IF.ids) tags.push("id");
      if (itemFlags & IF.text) tags.push("text");
      if (itemFlags & IF.teleport) tags.push("tp");
      if (itemFlags & IF.doorId) tags.push("door");
      result.labels.push({ x: camera.tileX(x, z) + tilePx / 2, y: camera.tileY(y, z) + tilePx / 2, text: `${tile.ids[i]} ${tags.join(" ")}`, color: TOOLTIP_COLOR });
      break;
    }
  });
}
