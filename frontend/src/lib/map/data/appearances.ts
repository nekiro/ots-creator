// Render data of client items and outfits, loaded in batches from
// MapService.Appearances.
import { Category } from "../../api";
import { MapService, type Appearance } from "./api";

/** Appearance flags (see app.Appearance). */
export const AF = {
  animateAlways: 1,
  hangable: 2,
  hookSouth: 4,
  hookEast: 8,
  lying: 16,
  translucent: 32,
  top: 64,
  bottom: 128,
  border: 256,
  ground: 512,
  dontHide: 1024,
  fullGround: 2048,
} as const;

export interface Look extends Omit<Appearance, "sprites" | "offsetX" | "offsetY" | "elevation" | "moving"> {
  sprites: number[];
  /** The walking frame group of an outfit that has one. */
  moving: Look | null;
  offsetX: number;
  offsetY: number;
  elevation: number;
  /** Cumulative frame end times (ms) for animated things. */
  frameEnds: number[] | null;
  loopDuration: number;
}

function prepare(appearance: Appearance): Look {
  const look = appearance as unknown as Look;
  look.sprites = appearance.sprites ?? [];
  look.offsetX = appearance.offsetX ?? 0;
  look.offsetY = appearance.offsetY ?? 0;
  look.elevation = appearance.elevation ?? 0;
  look.frameEnds = null;
  look.loopDuration = 0;
  look.moving = appearance.moving ? prepare(appearance.moving) : null;
  if (appearance.frames > 1) {
    const durations = appearance.durations && appearance.durations.length === appearance.frames ? appearance.durations : new Array(appearance.frames).fill(appearance.frames > 1 ? 500 : 0);
    let end = 0;
    look.frameEnds = durations.map((duration) => (end += Math.max(duration, 50)));
    look.loopDuration = end;
  }
  return look;
}

/** Frame of an animation at a time (all instances run in sync). */
export function frameAt(look: Look, time: number): number {
  if (!look.frameEnds) return 0;
  const inLoop = time % look.loopDuration;
  for (let i = 0; i < look.frameEnds.length; i++) if (inLoop < look.frameEnds[i]) return i;
  return 0;
}

/** Index in look.sprites of the first piece of a layer of a frame and pattern (pieces follow row by row, width a row). */
export function spriteIndex(look: Look, frame: number, patternX: number, patternY: number, patternZ: number, layer: number): number {
  return ((((frame * look.patternZ + patternZ) * look.patternY + patternY) * look.patternX + patternX) * look.layers + layer) * look.height * look.width;
}

class Cache {
  private map = new Map<number, Look | null>();
  private queued = new Set<number>();
  private timer = 0;
  constructor(
    private category: Category,
    private onload: () => void,
  ) {}

  get(id: number): Look | null | undefined {
    const look = this.map.get(id);
    if (look === undefined && !this.queued.has(id)) {
      this.queued.add(id);
      if (!this.timer) this.timer = window.setTimeout(() => this.flush(), 0);
    }
    return look;
  }

  private async flush(): Promise<void> {
    this.timer = 0;
    const ids = [...this.queued];
    for (let i = 0; i < ids.length; i += 400) {
      const batch = ids.slice(i, i + 400);
      try {
        const list = (await MapService.Appearances(this.category, batch)) ?? [];
        const found = new Set<number>();
        for (const appearance of list) {
          this.map.set(appearance.id, prepare(appearance));
          found.add(appearance.id);
        }
        for (const id of batch) {
          if (!found.has(id)) this.map.set(id, null);
          this.queued.delete(id);
        }
      } catch {
        for (const id of batch) this.queued.delete(id);
      }
    }
    this.onload();
  }

  clear(): void {
    this.map.clear();
  }
}

/** Item and outfit appearances. */
export class Appearances {
  readonly items: Cache;
  readonly outfits: Cache;
  constructor(onload: () => void) {
    this.items = new Cache(Category.CategoryItem, onload);
    this.outfits = new Cache(Category.CategoryOutfit, onload);
  }
  /** Bumped by clear(): what was built from older appearances is stale. */
  rev = 0;
  clear(): void {
    this.items.clear();
    this.outfits.clear();
    this.rev++;
  }
}
