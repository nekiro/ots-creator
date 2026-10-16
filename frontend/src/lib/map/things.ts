// Items and creatures drawn as screen quads: brush previews, pasted
// things and dragged selections (chunk quads are built by the backend).
import { AF, frameAt, type Look } from "./appearances";
import { IF, TF, type CreatureData } from "./chunks";
import { composedKey, composeOutfit, packColors } from "./composed";
import { ALPHA, COLORS } from "./constants";
import type { QuadBatch } from "./quadBatch";
import type { Scene } from "./scene";
import type { Camera, TileRange } from "./view";

/** Draws an item with its sprite's top-left corner at screenX, screenY, tinted
 * with r, g, b, a. subtype: count, fluid or charges; hookSide: wall hook pattern
 * (0: none). Returns the item's elevation: how far things on top move up. */
export type DrawItem = (itemId: number, subtype: number, x: number, y: number, z: number, screenX: number, screenY: number, r: number, g: number, b: number, a: number, hookSide: number) => number;
export type DrawCreature = (creature: CreatureData, screenX: number, screenY: number, x: number, y: number, z: number, alpha: number) => void;

/** What drawing found missing in a frame. */
interface DrawStatus {
  pending: boolean;
  animated: boolean;
}

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

    const drawItem: DrawItem = (itemId, subtype, x, y, z, screenX, screenY, r, g, b, a, hookSide) => {
      if (!items.has(itemId)) {
        this.batch.rect(screenX, screenY, tilePx, tilePx, [COLORS.unknownItem[0], COLORS.unknownItem[1], COLORS.unknownItem[2], COLORS.unknownItem[3] * a]);
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
        this.batch.rect(screenX, screenY, tilePx, tilePx, [COLORS.itemWithoutLook[0], COLORS.itemWithoutLook[1], COLORS.itemWithoutLook[2], COLORS.itemWithoutLook[3] * a]);
        return 0;
      }
      let patternX = 0;
      let patternY = 0;
      if (itemFlags & fluidBits) {
        const fluidColor = subtype % 8;
        patternX = (fluidColor % 4) % look.px;
        patternY = Math.floor(fluidColor / 4) % look.py;
      } else if (look.fl & AF.hangable && hookSide) {
        patternX = Math.min(hookSide, look.px - 1);
      } else if (itemFlags & stackBit && look.px === 4 && look.py === 2) {
        [patternX, patternY] = stackPattern(subtype);
      } else {
        patternX = x % look.px;
        patternY = y % look.py;
      }
      const patternZ = z % look.pz;
      let frame = 0;
      if (look.f > 1) {
        status.animated = scene.opts.animation || status.animated;
        frame = frameAt(look, time);
      }
      this.drawLook(scene, look, patternX, patternY, patternZ, frame, screenX - look.ox * scale, screenY - look.oy * scale, tilePx, r, g, b, a, null);
      return look.el;
    };

    const drawCreature: DrawCreature = (creature, screenX, screenY, x, y, z, alpha) => {
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
      const direction = Math.min(creature.dir, look.px - 1);
      let mounted = 0;
      if (creature.mount && look.pz > 1) {
        const mount = scene.looks.outfits.get(creature.mount);
        if (mount === undefined) status.pending = true;
        else if (mount) {
          this.drawLook(scene, mount, Math.min(direction, mount.px - 1), 0, 0, 0, screenX - mount.ox * scale, screenY - mount.oy * scale, tilePx, shade, shade, shade, alpha, null);
          mounted = 1;
        }
      }
      const colors = look.l > 1 ? creature : null;
      const outfitX = screenX - look.ox * scale;
      const outfitY = screenY - look.oy * scale;
      this.drawLook(scene, look, direction, 0, mounted, 0, outfitX, outfitY, tilePx, shade, shade, shade, alpha, colors);
      // Addons are drawn from the next pattern rows: bit 1 row 1, bit 2 row 2.
      for (const addon of [1, 2]) {
        if (creature.addons & addon && look.py > addon) {
          this.drawLook(scene, look, direction, addon, mounted, 0, outfitX, outfitY, tilePx, shade, shade, shade, alpha, colors);
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
        if (!tile || !(tile.flags & TF.selected)) continue;
        const movedX = x + offset.dx;
        const movedY = y + offset.dy;
        const screenX = camera.tileX(movedX, z);
        const screenY = camera.tileY(movedY, z);
        let elevation = 0;
        for (let i = 0; i < tile.ids.length; i++) {
          if (!(tile.iflags[i] & IF.selected)) continue;
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
    const stackRank = (itemId: number) => (scene.items.is(itemId, "ground") ? 0 : ([3, 1, 2, 4][scene.items.top[itemId]] ?? 3));
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
    r: number,
    g: number,
    b: number,
    a: number,
    colors: CreatureData | null = null,
  ): void {
    const atlas = this.batch.atlas!;
    const sprites = scene.sprites;
    const layers = colors ? 1 : look.l;
    const animationFrame = frame % Math.max(look.f, 1);
    for (let layer = 0; layer < layers; layer++) {
      for (let pieceY = 0; pieceY < look.h; pieceY++) {
        for (let pieceX = 0; pieceX < look.w; pieceX++) {
          const spriteIndex = (((((animationFrame * look.pz + patternZ) * look.py + patternY) * look.px + patternX) * look.l + layer) * look.h + pieceY) * look.w + pieceX;
          const spriteId = look.s[spriteIndex];
          if (!spriteId) continue;
          let cell: number;
          if (colors) {
            // The template is layer 1 of the same piece.
            const templateId = look.s[spriteIndex + look.h * look.w] ?? 0;
            const key = composedKey(spriteId, templateId, packColors(colors));
            cell = atlas.cellOf(key, () => composeOutfit(sprites, spriteId, templateId, colors));
            if (cell < 0) {
              if (!sprites.get(spriteId)) this.missing.add(spriteId);
              if (templateId && !sprites.get(templateId)) this.missing.add(templateId);
              continue;
            }
          } else {
            cell = atlas.cellOf(spriteId, () => sprites.get(spriteId)?.pixels);
            if (cell < 0) {
              if (!sprites.get(spriteId)) this.missing.add(spriteId);
              continue;
            }
          }
          this.batch.push(screenX - pieceX * tilePx, screenY - pieceY * tilePx, tilePx, tilePx, cell, r, g, b, a);
        }
      }
    }
  }
}
