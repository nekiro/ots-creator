// Overlays drawn over the map every frame in screen pixels: grid, spawn
// radiuses, selection and area rectangles, markers, the client view box,
// and labels (creature names, item texts, positions, item tooltips).
import { ItemFlags, TileFlags } from "../data/chunks";
import { ALPHA, COLORS, LABEL_COLORS, LIMITS, ZOOM } from "../constants";
import { labelText, wrapText } from "../editor/itemText";
import { wholePixelRatio } from "./labels";
import type { QuadBatch } from "./quadBatch";
import type { FrameResult, Scene } from "./scene";
import { type Camera } from "../view/view";

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
  if (opts.grid && tilePx >= ZOOM.grid) drawGrid(overlay);
  if (opts.spawns && tilePx >= ZOOM.tileData) drawSpawns(overlay);
  drawRects(overlay);
  if (opts.markers) drawMarkers(overlay);
  if (opts.clientBox) drawClientBox(overlay);
  if (opts.creatures && opts.labels && tilePx >= ZOOM.labels && near) labelCreatures(overlay);
  if (opts.labels && tilePx >= ZOOM.labels && near) labelItemTexts(overlay);
  if (opts.coords && scene.hover && scene.hover.z === overlay.z) labelHoverPosition(overlay);
  if (opts.tooltips && tilePx >= ZOOM.labels && near) labelItemTooltips(overlay);
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

function drawGrid({ scene, camera, batch, z }: Overlay): void {
  const range = camera.tiles(z, 0, 0);
  for (let x = range.x0; x <= range.x1; x++) batch.rect(camera.tileX(x, z), 0, 1, scene.height, COLORS.grid);
  for (let y = range.y0; y <= range.y1; y++) batch.rect(0, camera.tileY(y, z), scene.width, 1, COLORS.grid);
}

/** Spawn radius outlines of the current floor. */
function drawSpawns(overlay: Overlay): void {
  const { scene, camera, batch, z, tilePx, lineWidth } = overlay;
  forEachTile(overlay, LIMITS.spawnSearch, LIMITS.spawnSearch, (x, y) => {
    const tile = scene.chunks.peekTile(x, y, z);
    if (!tile || !(tile.flags & (TileFlags.spawn | TileFlags.spawnNpc))) return;
    for (const spawn of [tile.spawn, tile.spawnNpc]) {
      if (!spawn) continue;
      const radius = spawn.radius;
      const color = spawn.selected ? COLORS.spawn.selected : tile.spawnNpc === spawn ? COLORS.spawn.npc : COLORS.spawn.monster;
      const size = (2 * radius + 1) * tilePx;
      batch.stroke(camera.tileX(x - radius, z), camera.tileY(y - radius, z), size, size, color, lineWidth);
      batch.rect(camera.tileX(x, z), camera.tileY(y, z), tilePx, tilePx, [color[0], color[1], color[2], ALPHA.spawnCenter]);
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
    const [red, green, blue] = marker.color;
    batch.stroke(x + lineWidth, y + lineWidth, tilePx - 2 * lineWidth, tilePx - 2 * lineWidth, [red, green, blue, ALPHA.marker], Math.max(2, lineWidth * 2));
    if (tilePx >= ZOOM.markerNames && scene.opts.labels) {
      // The whole name under the mouse, the short one elsewhere.
      const hovered = isHovered(scene, marker.x, marker.y, z);
      result.labels.push({
        x: x + tilePx / 2,
        y: y - 2,
        text: hovered ? (marker.full ?? marker.label) : marker.label,
        top: hovered,
        color: `rgb(${Math.round(red * 255)},${Math.round(green * 255)},${Math.round(blue * 255)})`,
      });
    }
  }
}

/** The client shows 15x11 tiles around the player (plus one hidden row). */
function drawClientBox({ camera, batch, z, tilePx, lineWidth }: Overlay): void {
  const playerX = Math.floor(camera.centerX(z));
  const playerY = Math.floor(camera.centerY(z));
  batch.stroke(camera.tileX(playerX - 7, z), camera.tileY(playerY - 5, z), 15 * tilePx, 11 * tilePx, COLORS.clientBox, Math.max(2, lineWidth));
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
      result.labels.push({ x: camera.tileX(x, z) + tilePx / 2 - spriteOffset, y: camera.tileY(y, z) - spriteOffset - 2, text: creature.name, color: LABEL_COLORS.creatureName });
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
      const lineHeight = 11 * wholePixelRatio(globalThis.devicePixelRatio);
      lines.forEach((line, i) => result.labels.push({ x: centerX, y: centerY + (i - (lines.length - 1) / 2) * lineHeight, text: line, color: LABEL_COLORS.itemText, small: true, top: true }));
      return;
    }
    const firstLine = labelText(text);
    if (firstLine) result.labels.push({ x: centerX, y: centerY, text: firstLine, color: LABEL_COLORS.itemText, small: true, cull: true });
  });
}

/** The position of the tile under the mouse, in a dark tag above it (below it at the top edge). */
function labelHoverPosition({ scene, result, camera, batch, z, tilePx }: Overlay): void {
  const hover = scene.hover!;
  const ratio = wholePixelRatio(globalThis.devicePixelRatio);
  const text = `${hover.x}, ${hover.y}, ${hover.z}`;
  const tagWidth = (text.length * 5 + 8) * ratio;
  const tagHeight = 13 * ratio;
  const x = camera.tileX(hover.x, z);
  const y = camera.tileY(hover.y, z);
  const tagX = Math.min(Math.max(0, x + tilePx / 2 - tagWidth / 2), scene.width - tagWidth);
  const tagY = y - tagHeight - 2 * ratio >= 0 ? y - tagHeight - 2 * ratio : y + tilePx + 2 * ratio;
  batch.rect(x, y, tilePx, tilePx, COLORS.hoverTile);
  batch.rect(tagX, tagY, tagWidth, tagHeight, COLORS.positionTag);
  result.labels.push({ x: tagX + tagWidth / 2, y: tagY + tagHeight / 2, text, color: LABEL_COLORS.position, small: true });
}

/** The id of the first item on a tile with an action id, unique id, text,
 * teleport destination or door id, and what it has. */
function labelItemTooltips(overlay: Overlay): void {
  const { scene, result, camera, z, tilePx } = overlay;
  forEachTile(overlay, 0, 0, (x, y) => {
    const tile = scene.chunks.peekTile(x, y, z);
    if (!tile) return;
    for (let i = 0; i < tile.ids.length; i++) {
      const itemFlags = tile.itemFlags[i];
      if (!(itemFlags & (ItemFlags.ids | ItemFlags.text | ItemFlags.teleport | ItemFlags.doorId))) continue;
      const tags: string[] = [];
      if (itemFlags & ItemFlags.ids) tags.push("id");
      if (itemFlags & ItemFlags.text) tags.push("text");
      if (itemFlags & ItemFlags.teleport) tags.push("tp");
      if (itemFlags & ItemFlags.doorId) tags.push("door");
      result.labels.push({ x: camera.tileX(x, z) + tilePx / 2, y: camera.tileY(y, z) + tilePx / 2, text: `${tile.ids[i]} ${tags.join(" ")}`, color: LABEL_COLORS.tooltip });
      break;
    }
  });
}
