// Items and creatures drawn as screen quads: brush previews, pasted
// things and dragged selections (chunk quads are built by the backend).
import { AF, frameAt, spriteIndex, type Look } from "./appearances";
import { ItemFlags, TileFlags, type CreatureData } from "./chunks";
import { TOP_ORDER } from "./items";
import { composedKey, composeOutfit, packColors } from "../render/composed";
import { ALPHA, COLORS } from "../constants";
import type { QuadBatch } from "../render/quadBatch";
import type { Scene } from "../render/scene";
import type { Camera, TileRange } from "../view/view";

/** Draws an item with its sprite's top-left corner at screenX, screenY, tinted
 * with red, green, blue, alpha. subtype: count, fluid or charges; hookSide: wall hook pattern
 * (0: none). Returns the item's elevation: how far things on top move up. */
export type DrawItem = (itemId: number, subtype: number, x: number, y: number, z: number, screenX: number, screenY: number, red: number, green: number, blue: number, alpha: number, hookSide: number) => number;
/** footPhase: null draws the first frame (the editor); a number animates as the client does, 0 standing, from 1 walking. */
export type DrawCreature = (creature: CreatureData, screenX: number, screenY: number, x: number, y: number, z: number, alpha: number, footPhase?: number | null) => void;

/** What drawing found missing in a frame. */
interface DrawStatus {
  pending: boolean;
  animated: boolean;
}

/** Client stacking rank of non-ground items by top order: borders, bottom, common, top items (grounds rank 0). */
const STACK_RANK_BY_TOP_ORDER: Record<number, number> = { [TOP_ORDER.common]: 3, [TOP_ORDER.border]: 1, [TOP_ORDER.bottom]: 2, [TOP_ORDER.top]: 4 };

/** Sprite pattern of a stackable item by its count, as the client picks it. */
function stackPattern(count: number): [number, number] {
  if (count <= 0) return [0, 0];
  if (count < 5) return [count - 1, 0];
  if (count < 10) return [0, 1];
  if (count < 25) return [1, 1];
  if (count < 50) return [2, 1];
  return [3, 1];
}

export class ThingDrawer {
  /** missing: where sprites not loaded yet are added. */
  constructor(
    private batch: QuadBatch,
    private missing: Set<number>,
  ) {}

  /** Item and creature drawing in screen pixels: scale device pixels per world pixel, tilePx per tile. */
  drawers(scene: Scene, scale: number, tilePx: number, status: DrawStatus): { drawItem: DrawItem; drawCreature: DrawCreature } {
    const items = scene.items;
    const bits = items.bits;
    const metaBit = bits.meta ?? 0;
    const fluidBits = (bits.splash ?? 0) | (bits.fluidContainer ?? 0);
    const stackBit = bits.stackable ?? 0;
    const time = scene.opts.animation ? scene.time : 0;

    const drawItem: DrawItem = (itemId, subtype, x, y, z, screenX, screenY, red, green, blue, alpha, hookSide) => {
      if (!items.has(itemId)) {
        this.batch.rect(screenX, screenY, tilePx, tilePx, [COLORS.unknownItem[0], COLORS.unknownItem[1], COLORS.unknownItem[2], COLORS.unknownItem[3] * alpha]);
        return 0;
      }
      const itemFlags = items.flags[itemId];
      if (itemFlags & metaBit) return 0;
      const look = scene.looks.items.get(items.client[itemId]);
      if (look === undefined) {
        status.pending = true;
        return 0;
      }
      if (look === null) {
        this.batch.rect(screenX, screenY, tilePx, tilePx, [COLORS.itemWithoutLook[0], COLORS.itemWithoutLook[1], COLORS.itemWithoutLook[2], COLORS.itemWithoutLook[3] * alpha]);
        return 0;
      }
      let patternX = 0;
      let patternY = 0;
      if (itemFlags & fluidBits) {
        const fluidColor = subtype % 8;
        patternX = (fluidColor % 4) % look.patternX;
        patternY = Math.floor(fluidColor / 4) % look.patternY;
      } else if (look.flags & AF.hangable && hookSide) {
        patternX = Math.min(hookSide, look.patternX - 1);
      } else if (itemFlags & stackBit && look.patternX === 4 && look.patternY === 2) {
        [patternX, patternY] = stackPattern(subtype);
      } else {
        patternX = x % look.patternX;
        patternY = y % look.patternY;
      }
      const patternZ = z % look.patternZ;
      let frame = 0;
      if (look.frames > 1) {
        status.animated = scene.opts.animation || status.animated;
        frame = frameAt(look, time);
      }
      this.drawLook(scene, look, patternX, patternY, patternZ, frame, screenX - look.offsetX * scale, screenY - look.offsetY * scale, tilePx, red, green, blue, alpha, null);
      return look.elevation;
    };

    // The frame group and frame a creature is drawn with (client:
    // Creature::getCurrentAnimationPhase): standing, the idle animation;
    // walking, the walk phase in the walking group (or after frame 0).
    // A walk frame whose sprites are not loaded yet shows the standing frame:
    // the creature must not blink while they load.
    const animationOf = (look: Look, footPhase: number | null, patternX = 0, patternZ = 0): [Look, number] => {
      if (footPhase === null) return [look, 0];
      if (footPhase > 0) {
        const [frames, frame] = look.moving ? [look.moving, Math.min(footPhase - 1, look.moving.frames - 1)] : [look, Math.min(footPhase, look.frames - 1)];
        if (this.frameLoaded(scene, frames, patternX, patternZ, frame)) return [frames, frame];
        footPhase = 0;
      }
      if (look.frames > 1 && (look.moving || look.flags & AF.animateAlways)) {
        status.animated = true;
        return [look, frameAt(look, time)];
      }
      return [look, 0];
    };

    const drawCreature: DrawCreature = (creature, screenX, screenY, x, y, z, alpha, footPhase = null) => {
      const shade = creature.selected ? ALPHA.selectedShade : 1;
      if (creature.lookItem) {
        drawItem(creature.lookItem, 0, x, y, z, screenX, screenY, shade, shade, shade, alpha, 0);
        return;
      }
      if (!creature.lookType) {
        // Unknown creature: a marker so it is not invisible.
        const color = creature.npc ? COLORS.unknownNpc : COLORS.unknownMonster;
        this.batch.rect(screenX + tilePx * 0.25, screenY + tilePx * 0.25, tilePx * 0.5, tilePx * 0.5, [color[0], color[1], color[2], color[3] * alpha]);
        return;
      }
      const look = scene.looks.outfits.get(creature.lookType);
      if (look === undefined) {
        status.pending = true;
        return;
      }
      if (!look) return;
      const direction = Math.min(creature.dir, look.patternX - 1);
      let mounted = 0;
      let outfitX = screenX - look.offsetX * scale;
      let outfitY = screenY - look.offsetY * scale;
      if (creature.mount && look.patternZ > 1) {
        const mount = scene.looks.outfits.get(creature.mount);
        if (mount === undefined) status.pending = true;
        else if (mount) {
          // Creature::internalDraw: the point moves back by the mount's
          // displacement, the mount is drawn there (less its displacement
          // again), and the rider at the point plus its own displacement,
          // so the rider ends up back by the mount's displacement.
          const [mountFrames, mountFrame] = animationOf(mount, footPhase, Math.min(direction, mount.patternX - 1), 0);
          this.drawLook(scene, mountFrames, Math.min(direction, mount.patternX - 1), 0, 0, mountFrame, screenX - 2 * mount.offsetX * scale, screenY - 2 * mount.offsetY * scale, tilePx, shade, shade, shade, alpha, null);
          mounted = 1;
          outfitX = screenX - mount.offsetX * scale;
          outfitY = screenY - mount.offsetY * scale;
        }
      }
      const colors = look.layers > 1 ? creature : null;
      const [frames, frame] = animationOf(look, footPhase, direction, mounted);
      this.drawLook(scene, frames, direction, 0, mounted, frame, outfitX, outfitY, tilePx, shade, shade, shade, alpha, colors);
      // Addons are drawn from the next pattern rows: bit 1 row 1, bit 2 row 2.
      for (const addon of [1, 2]) {
        if (creature.addons & addon && look.patternY > addon) {
          this.drawLook(scene, frames, direction, addon, mounted, frame, outfitX, outfitY, tilePx, shade, shade, shade, alpha, colors);
        }
      }
    };
    return { drawItem, drawCreature };
  }

  /** Draws the selected things of the current floor at the drag offset. */
  drawMoved(scene: Scene, z: number, range: TileRange, camera: Camera, drawItem: DrawItem, drawCreature: DrawCreature): void {
    const tilePx = camera.tilePx;
    const offset = scene.moveOffset!;
    for (let y = Math.max(0, range.y0 - offset.dy); y <= range.y1 - offset.dy; y++) {
      for (let x = Math.max(0, range.x0 - offset.dx); x <= range.x1 - offset.dx; x++) {
        const tile = scene.chunks.peekTile(x, y, z);
        if (!tile || !(tile.flags & TileFlags.selected)) continue;
        const movedX = x + offset.dx;
        const movedY = y + offset.dy;
        const screenX = camera.tileX(movedX, z);
        const screenY = camera.tileY(movedY, z);
        let elevation = 0;
        for (let i = 0; i < tile.ids.length; i++) {
          if (!(tile.itemFlags[i] & ItemFlags.selected)) continue;
          elevation += drawItem(tile.ids[i], tile.subs[i], movedX, movedY, z, screenX - elevation * scene.scale, screenY - elevation * scene.scale, 1, 1, 1, ALPHA.moved, 0);
        }
        for (const creature of tile.creatures ?? []) if (creature.selected) drawCreature(creature, screenX, screenY, movedX, movedY, z, ALPHA.moved);
        this.batch.rect(screenX, screenY, tilePx, tilePx, COLORS.movedTile);
      }
    }
  }

  /** Brush previews and pasted things not placed yet, on the floor in view. */
  drawGhosts(scene: Scene, camera: Camera, drawItem: DrawItem, drawCreature: DrawCreature): void {
    // Client stacking: ground, borders, bottom, common, top items.
    const stackRank = (itemId: number) => (scene.items.is(itemId, "ground") ? 0 : (STACK_RANK_BY_TOP_ORDER[scene.items.top[itemId]] ?? 3));
    for (const ghost of scene.ghosts) {
      if (ghost.z !== scene.floor) continue;
      const screenX = camera.tileX(ghost.x, ghost.z);
      const screenY = camera.tileY(ghost.y, ghost.z);
      let elevation = 0;
      const order = ghost.ids.map((_, i) => i).sort((first, second) => stackRank(ghost.ids[first]) - stackRank(ghost.ids[second]) || first - second);
      for (const i of order) {
        elevation += drawItem(ghost.ids[i], ghost.subs?.[i] ?? 1, ghost.x, ghost.y, ghost.z, screenX - elevation * scene.scale, screenY - elevation * scene.scale, 1, 1, 1, scene.ghostAlpha, 0);
      }
      for (const creature of ghost.creatures ?? []) drawCreature(creature, screenX - elevation * scene.scale, screenY - elevation * scene.scale, ghost.x, ghost.y, ghost.z, scene.ghostAlpha);
    }
  }

  /** Every sprite of a frame of a look in a direction (all pattern rows: addons) is loaded. */
  private frameLoaded(scene: Scene, look: Look, patternX: number, patternZ: number, frame: number): boolean {
    const animationFrame = frame % Math.max(look.frames, 1);
    for (let patternY = 0; patternY < look.patternY; patternY++) {
      for (let layer = 0; layer < look.layers; layer++) {
        for (let piece = 0; piece < look.height * look.width; piece++) {
          const spriteId = look.sprites[spriteIndex(look, animationFrame, patternX, patternY, patternZ, layer) + piece];
          if (spriteId && !scene.sprites.get(spriteId)) {
            this.missing.add(spriteId);
            return false;
          }
        }
      }
    }
    return true;
  }

  /** Draws the sprites of a look's pattern and frame, its bottom-right
   * piece's top-left corner at screenX, screenY. With colors, only the
   * first layer is drawn, colored with the second (outfits). */
  private drawLook(
    scene: Scene,
    look: Look,
    patternX: number,
    patternY: number,
    patternZ: number,
    frame: number,
    screenX: number,
    screenY: number,
    tilePx: number,
    red: number,
    green: number,
    blue: number,
    alpha: number,
    colors: CreatureData | null = null,
  ): void {
    const atlas = this.batch.atlas!;
    const sprites = scene.sprites;
    const layers = colors ? 1 : look.layers;
    const animationFrame = frame % Math.max(look.frames, 1);
    for (let layer = 0; layer < layers; layer++) {
      const layerIndex = spriteIndex(look, animationFrame, patternX, patternY, patternZ, layer);
      for (let pieceY = 0; pieceY < look.height; pieceY++) {
        for (let pieceX = 0; pieceX < look.width; pieceX++) {
          const pieceIndex = layerIndex + pieceY * look.width + pieceX;
          const spriteId = look.sprites[pieceIndex];
          if (!spriteId) continue;
          let cell: number;
          if (colors) {
            // The template is layer 1 of the same piece.
            const templateId = look.sprites[pieceIndex + look.height * look.width] ?? 0;
            const key = composedKey(spriteId, templateId, packColors(colors));
            cell = atlas.cached(key);
            if (cell < 0) cell = atlas.cellOf(key, () => composeOutfit(sprites, spriteId, templateId, colors));
            if (cell < 0) {
              if (!sprites.get(spriteId)) this.missing.add(spriteId);
              if (templateId && !sprites.get(templateId)) this.missing.add(templateId);
              continue;
            }
          } else {
            cell = atlas.cached(spriteId);
            if (cell < 0) cell = atlas.cellOf(spriteId, () => sprites.get(spriteId)?.pixels);
            if (cell < 0) {
              if (!sprites.get(spriteId)) this.missing.add(spriteId);
              continue;
            }
          }
          this.batch.push(screenX - pieceX * tilePx, screenY - pieceY * tilePx, tilePx, tilePx, cell, red, green, blue, alpha);
        }
      }
    }
  }
}
