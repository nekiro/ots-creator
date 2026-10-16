// Render data of client items and outfits, loaded in batches from
// MapService.Appearances.
import { Category } from "../api";
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

export interface Look extends Omit<Appearance, "s" | "ox" | "oy" | "el"> {
  s: number[];
  ox: number;
  oy: number;
  el: number;
  /** Cumulative frame end times (ms) for animated things. */
  cum: number[] | null;
  total: number;
}

function prepare(appearance: Appearance): Look {
  const look = appearance as unknown as Look;
  look.s = appearance.s ?? [];
  look.ox = appearance.ox ?? 0;
  look.oy = appearance.oy ?? 0;
  look.el = appearance.el ?? 0;
  look.cum = null;
  look.total = 0;
  if (appearance.f > 1) {
    const durations = appearance.d && appearance.d.length === appearance.f ? appearance.d : new Array(appearance.f).fill(appearance.f > 1 ? 500 : 0);
    let end = 0;
    look.cum = durations.map((duration) => (end += Math.max(duration, 50)));
    look.total = end;
  }
  return look;
}

/** Frame of an animation at a time (all instances run in sync). */
export function frameAt(look: Look, time: number): number {
  if (!look.cum) return 0;
  const inLoop = time % look.total;
  for (let i = 0; i < look.cum.length; i++) if (inLoop < look.cum[i]) return i;
  return 0;
}

class Cache {
  private map = new Map<number, Look | null>();
  private queued = new Set<number>();
  private timer = 0;
  constructor(
    private cat: Category,
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
        const list = (await MapService.Appearances(this.cat, batch)) ?? [];
        const found = new Set<number>();
        for (const a of list) {
          this.map.set(a.id, prepare(a));
          found.add(a.id);
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
