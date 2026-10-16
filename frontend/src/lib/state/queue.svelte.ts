// Queues of objects waiting to be dropped into the open client (A) under
// ids chosen in its slots (DropPanel). Each drop takes the first queued
// object of the category. The compare window queues objects of the second
// client (B) by id; the object viewer queues object files by path.

import { CATEGORY_NAMES, CompareService, res, ThingService, type Category } from "../api";
import { ask } from "../confirm.svelte";
import { formatIdList } from "../util/format";
import { app, hasToast, otherVersions, run, toast } from "./app.svelte";

export interface QueueItem<K extends number | string = number | string> {
  category: Category;
  key: K;
  /** Thumbnail URL; the queue's thumb() is used when missing. */
  thumb?: string;
  /** Text shown for the item; "#key" when missing. */
  label?: string;
}

/** What a dropped batch got: ids in A, and new sprites when known. */
export interface DropResult {
  ids: number[];
  sprites?: number;
}

export interface QueueSource<K extends number | string> {
  /** Where the queued objects come from, for texts ("from B"). */
  from: string;
  /** Hint shown while the queue is empty. */
  hint: string;
  items: () => QueueItem<K>[];
  setItems: (items: QueueItem<K>[]) => void;
  thumb?: (category: Category, key: K) => string;
  /** Puts keys into A at targets, or at the end of A when targets is null. */
  put: (category: Category, keys: K[], targets: number[] | null) => Promise<DropResult>;
}

export class DropQueue<K extends number | string = number | string> {
  constructor(readonly src: QueueSource<K>) {}

  /** Queued items of category category, in queue order. */
  list(category: Category): QueueItem<K>[] {
    return this.src.items().filter((item) => item.category === category);
  }

  /** Queued keys of category category, in queue order. */
  queued(category: Category): K[] {
    return this.list(category).map((item) => item.key);
  }

  /** Total number of queued items per category. */
  counts(): Map<Category, number> {
    const counts = new Map<Category, number>();
    for (const item of this.src.items()) counts.set(item.category, (counts.get(item.category) ?? 0) + 1);
    return counts;
  }

  label(item: QueueItem<K>): string {
    return item.label ?? `#${item.key}`;
  }

  thumb(item: QueueItem<K>): string {
    return item.thumb ?? this.src.thumb?.(item.category, item.key) ?? "";
  }

  /** Adds items to the end of the queue (each one once). */
  enqueue(category: Category, items: (K | Omit<QueueItem<K>, "category">)[]): void {
    const have = new Set(this.queued(category));
    const add = items
      .map((entry) => (typeof entry === "object" ? { ...entry, category } : { category, key: entry }))
      .filter((item) => !have.has(item.key) && (have.add(item.key), true));
    this.src.setItems([...this.src.items(), ...add]);
  }

  dequeue(category: Category, keys: K[]): void {
    this.src.setItems(this.src.items().filter((item) => item.category !== category || !keys.includes(item.key)));
  }

  clear(category: Category): void {
    this.src.setItems(this.src.items().filter((item) => item.category !== category));
  }

  /** Moves a queued item to the front, so it is dropped next. */
  toFront(category: Category, key: K): void {
    const all = this.src.items();
    const index = all.findIndex((item) => item.category === category && item.key === key);
    if (index < 0) return;
    this.src.setItems([all[index], ...all.slice(0, index), ...all.slice(index + 1)]);
  }

  /**
   * Drops queued items of category into A from id at: the first n (all when n is
   * 0), into free ids from at, or into the ids in a row. at 0 appends them
   * to the end of A. Objects of A that would be replaced are confirmed
   * first. Returns the ids they got.
   */
  async drop(category: Category, at: number, n = 1, free = false): Promise<number[]> {
    const all = this.queued(category);
    const keys = n > 0 ? all.slice(0, n) : all;
    if (!keys.length) {
      toast(`The queue is empty. ${this.src.hint}`);
      return [];
    }
    let targets: number[] | null = null;
    if (at !== 0) {
      const plan = await run("Dropping", () => ThingService.PlanTargets(category, at, keys.length, free));
      if (!plan) return [];
      const taken = plan.taken ?? [];
      if (taken.length) {
        const list = taken.slice(0, 5).map((id) => `#${id}`).join(", ") + (taken.length > 5 ? "…" : "");
        if (!(await ask({ title: "Replace objects", message: `Replace ${taken.length} object(s) of A (${list})? Ctrl+Z brings them back.`, ok: "Replace" }))) return [];
      }
      targets = plan.ids ?? [];
    }
    const result = await run("Dropping", () => this.src.put(category, keys, targets));
    if (!result) return [];
    this.dequeue(category, keys);
    // Drops in a row add up in one toast while it is shown.
    if (!hasToast(DROP_TOAST) || streak.category !== category || streak.queue !== this) streak = { queue: this, category, ids: [], sprites: 0 };
    streak.ids.push(...result.ids);
    streak.sprites += result.sprites ?? 0;
    const sprites = result.sprites === undefined ? "" : `, ${streak.sprites} new sprite(s)`;
    toast(`Dropped ${streak.ids.length} ${CATEGORY_NAMES[category]}(s) at ${formatIdList(streak.ids)}${sprites}.`, "success", 3500, DROP_TOAST);
    return result.ids;
  }
}

const DROP_TOAST = "queue-drop";
let streak: { queue: unknown; category: Category; ids: number[]; sprites: number } = { queue: null, category: 0 as Category, ids: [], sprites: 0 };

/** Objects of B queued in the compare window. */
export const compareQueue = new DropQueue<number>({
  from: "from B",
  hint: "Queue objects of B first (double click a row).",
  items: () => app.queue,
  setItems: (items) => (app.queue = items),
  thumb: (category, id) => res.otherThumb(category, id, otherVersions.thing(category, id)),
  put: async (category, ids, targets) => {
    const result = targets ? await CompareService.TransferTo(false, category, ids, targets) : await CompareService.Transfer(false, category, ids, true);
    return { ids: result.ids ?? [], sprites: result.sprites };
  },
});

/** Object files queued in the object viewer. */
export const fileQueue = new DropQueue<string>({
  from: "files",
  hint: "Queue object files first (double click a file).",
  items: () => app.fileQueue,
  setItems: (items) => (app.fileQueue = items),
  put: async (category, paths, targets) => {
    const end = app.maxId(category) + 1;
    const ids = await ThingService.ImportObjectsTo(category, paths, targets ?? paths.map((_, i) => end + i));
    return { ids: ids ?? [] };
  },
});
