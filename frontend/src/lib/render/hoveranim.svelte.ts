// Animated thumbnail of the hovered list cell: after a short delay the
// hovered object is loaded with its sprites, and the cell swaps its static
// thumbnail for a ThingCanvas. Only animated objects are shown.

import { Category, type Thing } from "../api";
import type { SpriteCache } from "./sprites";

export class HoverAnim {
  /** Hovered object, once it and its sprites are loaded. */
  current = $state<{ id: number; thing: Thing } | null>(null);
  /** Bumped when the sprites are ready; pass to ThingCanvas `ready`. */
  ready = $state(0);
  private id: number | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;

  /** load returns the object, or null when it is gone or stale. */
  constructor(
    readonly cache: SpriteCache,
    private load: (id: number) => Promise<Thing | null>,
  ) {}

  /** Frame group to animate: outfits walk when they have a walking group. */
  static group(thing: Thing): number {
    return thing.category === Category.CategoryOutfit && thing.frameGroups.length > 1 ? 1 : 0;
  }

  enter(id: number): void {
    this.id = id;
    clearTimeout(this.timer);
    this.timer = setTimeout(async () => {
      try {
        const thing = await this.load(id);
        if (!thing || this.id !== id) return;
        const group = thing.frameGroups[HoverAnim.group(thing)];
        if (!group || group.frames < 2) return;
        await this.cache.load(group.sprites);
        if (this.id !== id) return;
        this.current = { id, thing };
        this.ready++;
      } catch {
        /* thumbnail stays static */
      }
    }, 150);
  }

  leave(): void {
    this.id = null;
    clearTimeout(this.timer);
    this.current = null;
  }
}
