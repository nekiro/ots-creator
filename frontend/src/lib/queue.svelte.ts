// Queue of the compare window: objects of the second client (B) waiting to
// be dropped into A under ids chosen in A's slots. Each drop takes the
// first queued object of the category.

import { CATEGORY_NAMES, CompareService, type Category } from "./api";
import { ask } from "./confirm.svelte";
import { app, hasToast, run, toast } from "./state.svelte";

/** Queued ids of B in category c, in queue order. */
export function queued(c: Category): number[] {
  return app.queue.filter((q) => q.category === c).map((q) => q.id);
}

/** Adds objects of B to the end of the queue (each one once). */
export function enqueue(c: Category, ids: number[]): void {
  const have = new Set(queued(c));
  const add = ids.filter((id) => !have.has(id));
  app.queue = [...app.queue, ...add.map((id) => ({ category: c, id }))];
}

export function dequeue(c: Category, ids: number[]): void {
  app.queue = app.queue.filter((q) => q.category !== c || !ids.includes(q.id));
}

export function clearQueue(c: Category): void {
  app.queue = app.queue.filter((q) => q.category !== c);
}

/** Moves a queued object to the front, so it is dropped next. */
export function toFront(c: Category, id: number): void {
  const i = app.queue.findIndex((q) => q.category === c && q.id === id);
  if (i < 0) return;
  const q = app.queue[i];
  app.queue = [q, ...app.queue.slice(0, i), ...app.queue.slice(i + 1)];
}

/**
 * Drops queued objects of c into A from id at: the first n (all when n is
 * 0), into free ids from at, or into the ids in a row. at 0 appends them to
 * the end of A. Objects of A that would be replaced are confirmed first.
 * Returns the ids they got.
 */
export async function drop(c: Category, at: number, n = 1, free = false): Promise<number[]> {
  const all = queued(c);
  const ids = n > 0 ? all.slice(0, n) : all;
  if (!ids.length) {
    toast("The queue is empty. Queue objects of B first (double click a row).");
    return [];
  }
  let r;
  if (at === 0) {
    r = await run("Dropping", () => CompareService.Transfer(false, c, ids, true));
  } else {
    const plan = await run("Dropping", () => CompareService.PlanTargets(false, c, at, ids.length, free));
    if (!plan) return [];
    const taken = plan.taken ?? [];
    if (taken.length) {
      const list = taken.slice(0, 5).map((id) => `#${id}`).join(", ") + (taken.length > 5 ? "…" : "");
      if (!(await ask({ title: "Replace objects", message: `Replace ${taken.length} object(s) of A (${list})? Ctrl+Z brings them back.`, ok: "Replace" }))) return [];
    }
    r = await run("Dropping", () => CompareService.TransferTo(false, c, ids, plan.ids ?? []));
  }
  if (!r) return [];
  dequeue(c, ids);
  const got = r.ids ?? [];
  // Drops in a row add up in one toast while it is shown.
  if (!hasToast(DROP_TOAST) || streak.c !== c) streak = { c, ids: [], sprites: 0 };
  streak.ids.push(...got);
  streak.sprites += r.sprites;
  toast(`Dropped ${streak.ids.length} ${CATEGORY_NAMES[c]}(s) at ${idRange(streak.ids)}, ${streak.sprites} new sprite(s).`, "success", 3500, DROP_TOAST);
  return got;
}

const DROP_TOAST = "compare-drop";
let streak: { c: Category; ids: number[]; sprites: number } = { c: 0 as Category, ids: [], sprites: 0 };

/** Short text of ids: "#5", "#5-#8" or "#5, #9, #12 (+3)". */
function idRange(ids: number[]): string {
  const sorted = [...ids].sort((a, b) => a - b);
  if (sorted.every((id, i) => id === sorted[0] + i)) return sorted.length === 1 ? `#${sorted[0]}` : `#${sorted[0]}-#${sorted.at(-1)}`;
  const head = sorted.slice(0, 3).map((id) => `#${id}`).join(", ");
  return sorted.length > 3 ? `${head} (+${sorted.length - 3})` : head;
}
