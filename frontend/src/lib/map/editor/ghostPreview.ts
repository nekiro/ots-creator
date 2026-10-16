// What the map shows of an action before it happens: ghost tiles of the
// brush, paste or doodad and rectangles of selections and erasing.
import type { Clip, CreatureType, DoodadBuffer, Position } from "../data/api";
import type { CreatureData } from "../data/chunks";
import { AREA_BRUSH_KINDS, MAX_RECT_PREVIEW_TILES, PREVIEW_COLORS } from "../constants";
import type { GhostTile, RectMark } from "../render/scene";
import type { BrushSel, Tool } from "../state.svelte";
import { brushArea, squareOutline } from "./tileGeometry";

/** A mouse drag on the map canvas. */
export type MapDrag =
  | { kind: "pan"; x: number; y: number; camX: number; camY: number; button: number; moved: boolean }
  | { kind: "paint"; last: Position; stroke: number; pending: Position[]; erase: boolean; alt: boolean; inflight: boolean }
  | { kind: "rect"; start: Position; cur: Position; erase: boolean; alt: boolean }
  | { kind: "box"; start: Position; cur: Position; add: boolean; remove: boolean; px: { x: number; y: number }; started: boolean }
  | { kind: "move"; start: Position; dx: number; dy: number }
  | null;

export interface PreviewInput {
  drag: MapDrag;
  hover: Position | null;
  floor: number;
  tool: Tool;
  brush: BrushSel | null;
  /** Ctrl is held: the brush erases. */
  erase: boolean;
  pasting: boolean;
  clip: Clip | null;
  doodad: DoodadBuffer | null;
  size: number;
  shape: string;
  fill: boolean;
  creatures: (CreatureType | null)[] | null | undefined;
  direction: number;
}

export interface Preview {
  ghosts: GhostTile[];
  rects: RectMark[];
}

/** Item shown for a brush (server id, 0 for none). */
export function lookOf(brush: BrushSel): number {
  return brush.look ?? brush.item ?? 0;
}

/** The creature of a creature brush, as drawn on a tile. */
export function creatureGhost(brush: BrushSel, creatures: PreviewInput["creatures"], direction: number): CreatureData[] {
  const creature = creatures?.find((candidate) => candidate?.name === brush.creature);
  if (!creature) return [];
  const outfit = creature.outfit;
  return [
    {
      npc: !!creature.npc,
      selected: false,
      dir: direction,
      lookType: outfit.lookType,
      head: outfit.head ?? 0,
      body: outfit.body ?? 0,
      legs: outfit.legs ?? 0,
      feet: outfit.feet ?? 0,
      addons: outfit.addons ?? 0,
      mount: outfit.mount ?? 0,
      lookItem: outfit.lookItem ?? 0,
      name: creature.name,
    },
  ];
}

const tileRect = (tile: Position, mark: Omit<RectMark, "x0" | "y0" | "x1" | "y1" | "z">): RectMark => ({ x0: tile.x, y0: tile.y, x1: tile.x, y1: tile.y, z: tile.z, ...mark });

export function computePreview(input: PreviewInput): Preview {
  const ghosts: GhostTile[] = [];
  const rects: RectMark[] = [];
  const result = { ghosts, rects };
  const { drag, hover, floor } = input;
  if (drag?.kind === "box") {
    rects.push({ x0: drag.start.x, y0: drag.start.y, x1: drag.cur.x, y1: drag.cur.y, z: floor, ...PREVIEW_COLORS.selectionBox });
  }
  if (drag?.kind === "rect") {
    const brush = input.brush;
    const colors = PREVIEW_COLORS.rect;
    rects.push({ x0: drag.start.x, y0: drag.start.y, x1: drag.cur.x, y1: drag.cur.y, z: floor, fill: drag.erase ? colors.eraseFill : colors.fill, stroke: colors.stroke });
    if (brush && !drag.erase && lookOf(brush) && AREA_BRUSH_KINDS.has(brush.kind)) {
      const [x0, x1] = [Math.min(drag.start.x, drag.cur.x), Math.max(drag.start.x, drag.cur.x)];
      const [y0, y1] = [Math.min(drag.start.y, drag.cur.y), Math.max(drag.start.y, drag.cur.y)];
      if ((x1 - x0 + 1) * (y1 - y0 + 1) <= MAX_RECT_PREVIEW_TILES) {
        for (let y = y0; y <= y1; y++) {
          for (let x = x0; x <= x1; x++) {
            const onBorder = x === x0 || x === x1 || y === y0 || y === y1;
            if (brush.kind !== "wall" || onBorder) ghosts.push({ x, y, z: floor, ids: [lookOf(brush)] });
          }
        }
      }
    }
    return result;
  }
  if (!hover || drag?.kind === "pan") return result;
  if (input.pasting && input.clip) {
    pastePreview(hover, input.clip, result);
    return result;
  }
  if (input.tool !== "brush" || !input.brush) return result;
  brushPreview(hover, input.brush, input, result);
  return result;
}

function pastePreview(hover: Position, clip: Clip, { ghosts, rects }: Preview): void {
  for (const tile of clip.tiles ?? []) {
    if (!tile) continue;
    const ids: number[] = [];
    const subs: number[] = [];
    if (tile.ground) {
      ids.push(tile.ground.id);
      subs.push(tile.ground.subtype);
    }
    for (const item of tile.items ?? []) {
      ids.push(item.id);
      subs.push(item.subtype);
    }
    ghosts.push({ x: hover.x + tile.dx, y: hover.y + tile.dy, z: hover.z + tile.dz, ids, subs });
  }
  rects.push({ x0: hover.x, y0: hover.y, x1: hover.x + clip.width - 1, y1: hover.y + clip.height - 1, z: hover.z, stroke: PREVIEW_COLORS.pasteBounds });
}

function brushPreview(hover: Position, brush: BrushSel, input: PreviewInput, { ghosts, rects }: Preview): void {
  const { erase, size } = input;
  if (brush.kind === "doodad") {
    if (erase) {
      for (const tile of brushArea(hover, size, input.shape)) rects.push(tileRect(tile, { fill: PREVIEW_COLORS.erasedTile }));
      return;
    }
    for (const tile of input.doodad?.tiles ?? []) {
      if (tile) ghosts.push({ x: hover.x + tile.x, y: hover.y + tile.y, z: hover.z + tile.z, ids: (tile.items ?? []).map((item) => item.id) });
    }
    return;
  }
  if (brush.kind === "spawn" || brush.kind === "spawnNpc") {
    const radius = Math.max(1, size);
    const colors = PREVIEW_COLORS.spawn;
    rects.push({ x0: hover.x - radius, y0: hover.y - radius, x1: hover.x + radius, y1: hover.y + radius, z: hover.z, stroke: erase ? colors.eraseStroke : colors.stroke, fill: colors.fill });
    return;
  }
  if (brush.kind === "creature") {
    if (erase) rects.push(tileRect(hover, { fill: PREVIEW_COLORS.erasedCreature }));
    else ghosts.push({ x: hover.x, y: hover.y, z: hover.z, ids: [], creatures: creatureGhost(brush, input.creatures, input.direction) });
    return;
  }
  if (brush.kind === "door" || brush.kind === "houseExit" || brush.kind === "waypoint") {
    rects.push(tileRect(hover, { ...PREVIEW_COLORS.singleTile }));
    return;
  }
  if (input.fill && brush.kind === "ground") {
    rects.push(tileRect(hover, { ...PREVIEW_COLORS.fill }));
    return;
  }
  const tiles = brush.kind === "wall" && size > 0 ? squareOutline(hover, size) : brushArea(hover, size, input.shape);
  const look = lookOf(brush);
  const marksTiles = brush.kind === "house" || brush.kind === "flag" || brush.kind === "zone" || brush.kind === "optionalBorder";
  for (const tile of tiles) {
    if (erase || brush.kind === "eraser") rects.push(tileRect(tile, { fill: PREVIEW_COLORS.erasedTile }));
    else if (marksTiles) rects.push(tileRect(tile, { fill: brush.kind === "house" ? PREVIEW_COLORS.houseTile : PREVIEW_COLORS.markTile }));
    else if (look) ghosts.push({ x: tile.x, y: tile.y, z: tile.z, ids: [look] });
  }
  if (tiles.length > 1) {
    const tileXs = tiles.map((tile) => tile.x);
    const tileYs = tiles.map((tile) => tile.y);
    rects.push({ x0: Math.min(...tileXs), y0: Math.min(...tileYs), x1: Math.max(...tileXs), y1: Math.max(...tileYs), z: hover.z, stroke: PREVIEW_COLORS.brushBounds });
  }
}
