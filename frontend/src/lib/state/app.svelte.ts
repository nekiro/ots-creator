import { Events } from "@wailsio/runtime";
import { Category, CompareService, ProjectService, ThingService, errorMessage, normalizeThing, type State, type Thing } from "../api";
import { otherSpriteCache, spriteCache, type SpriteCache } from "../render/sprites";
import type { QueueItem } from "./queue.svelte";

export type ToastKind = "info" | "success" | "error";
export interface Toast {
  id: number;
  kind: ToastKind;
  text: string;
  /** Toasts with the same key replace each other instead of stacking. */
  key?: string;
  action?: { label: string; run: () => void };
}

export type DialogName =
  | "open"
  | "new"
  | "compile"
  | "about"
  | "update"
  | "settings"
  | "optimize"
  | "durations"
  | "slicer"
  | "obd"
  | "sheet"
  | "market"
  | "share"
  | "compare"
  | "reorder"
  | "replaceRefs"
  | "exportSprites"
  | "itemsOtb"
  | null;

export type ShareTarget = (
  | { kind: "object"; category: Category; id: number }
  | { kind: "sprites"; ids: number[] }
  | { kind: "file"; path: string }
) & {
  pick?: boolean;
};

class AppState {
  project = $state.raw<State | null>(null);
  other = $state.raw<State | null>(null);
  category = $state<Category>(Category.CategoryItem);
  /** Selected thing ids in the current category (first = focused). */
  selection = $state.raw<number[]>([]);
  readonly selectionSet = $derived(new Set(this.selection));
  /** Working copy of the focused thing; edits apply on "Apply". */
  draft = $state<Thing | null>(null);
  original = $state<string>("");
  selectedSprites = $state.raw<number[]>([]);
  readonly selectedSpriteSet = $derived(new Set(this.selectedSprites));
  toasts = $state<Toast[]>([]);
  dialog = $state<DialogName>(null);
  busy = $state<string | null>(null);
  progress = $state<{ done: number; total: number } | null>(null);
  /** Label of the running operation that blocks the window with a loader (see run). */
  modal = $state<string | null>(null);
  stoppable = $state(false);
  stopping = $state(false);
  openPath = $state("");
  openOther = $state(false);
  sheetGroup = $state(0);
  obdPath = $state("");
  shareTarget = $state<ShareTarget | null>(null);
  queue = $state<QueueItem<number>[]>([]);
  fileQueue = $state<QueueItem<string>[]>([]);

  get open(): boolean {
    return !!this.project?.open;
  }
  get rev(): number {
    return this.project?.rev ?? 0;
  }
  get focused(): number | null {
    return this.selection[0] ?? null;
  }
  dirty = $derived(!!this.draft && JSON.stringify(this.draft) !== this.original);
  supported = $derived(new Set(this.project?.info?.supported ?? []));
  maxId(category: Category): number {
    const counts = this.project?.info?.counts;
    if (!counts) return 0;
    return [0, counts.items, counts.outfits, counts.effects, counts.missiles][category] ?? 0;
  }
}

export const app = new AppState();

/**
 * Cache keys of thumbnails and sprite images. A change bumps only the keys
 * of what it touched, so other images stay cached and do not flicker.
 */
class ResourceVersions {
  constructor(private cache: SpriteCache) {}
  private tick = $state(0);
  private epoch = 0;
  private things = new Map<string, number>();
  private sprites = new Map<number, number>();

  apply(rev: number, delta: State["delta"]): void {
    if (!delta || delta.all) {
      this.epoch = rev;
      this.things.clear();
      this.sprites.clear();
      this.cache.invalidate(null);
    } else {
      for (const thing of delta.things ?? []) this.things.set(`${thing.category}:${thing.id}`, rev);
      for (const id of delta.sprites ?? []) this.sprites.set(id, rev);
      this.cache.invalidate(delta.sprites ?? []);
    }
    this.tick++;
  }

  thing(category: Category, id: number): string {
    void this.tick;
    return `${this.epoch}.${this.things.get(`${category}:${id}`) ?? 0}`;
  }

  sprite(id: number): string {
    void this.tick;
    return `${this.epoch}.${this.sprites.get(id) ?? 0}`;
  }
}

export const versions = new ResourceVersions(spriteCache);
export const otherVersions = new ResourceVersions(otherSpriteCache);

let toastSeq = 0;
const toastTimers = new Map<number, ReturnType<typeof setTimeout>>();
const MAX_TOASTS = 4;
/** Shows a toast. One with the key of a shown toast replaces it and keeps it up longer. */
export function toast(text: string, kind: ToastKind = "info", duration = 3500, key?: string, action?: Toast["action"]): void {
  const old = key ? app.toasts.find((other) => other.key === key) : undefined;
  const shown = old ? { ...old, kind, text, action } : { id: ++toastSeq, kind, text, key, action };
  app.toasts = old ? app.toasts.map((other) => (other.id === shown.id ? shown : other)) : [...app.toasts, shown];
  while (app.toasts.length > MAX_TOASTS) {
    const gone = app.toasts[0];
    clearTimeout(toastTimers.get(gone.id));
    toastTimers.delete(gone.id);
    app.toasts = app.toasts.slice(1);
  }
  clearTimeout(toastTimers.get(shown.id));
  toastTimers.set(
    shown.id,
    setTimeout(
      () => {
        app.toasts = app.toasts.filter((other) => other.id !== shown.id);
        toastTimers.delete(shown.id);
      },
      kind === "error" ? duration * 2 : duration,
    ),
  );
}

export function hasToast(key: string): boolean {
  return app.toasts.some((shown) => shown.key === key);
}

/**
 * Runs an async action with a busy indicator and error toast. A modal run
 * also covers the window with a loader (BusyOverlay) until it ends.
 */
export async function run<T>(label: string, task: () => Promise<T>, options?: { modal?: boolean }): Promise<T | undefined> {
  app.busy = label;
  if (options?.modal) app.modal = label;
  try {
    return await task();
  } catch (err) {
    toast(errorMessage(err), "error");
    return undefined;
  } finally {
    app.busy = null;
    app.progress = null;
    app.modal = null;
  }
}

function applyState(state: State): void {
  const wasOpen = app.open;
  app.project = state;
  versions.apply(state.rev, state.delta);
  if (!state.open) {
    app.selection = [];
    app.draft = null;
    app.original = "";
    return;
  }
  const max = app.maxId(app.category);
  const min = app.category === Category.CategoryItem ? 100 : 1;
  if (!wasOpen || app.focused === null) {
    void select(min);
  } else if (app.focused > max) {
    void select(max);
  } else if (!app.dirty) {
    void reloadDraft();
  }
}

function applyOther(state: State): void {
  // Queued ids belong to the client they were queued from.
  if (!state.open || state.info?.datPath !== app.other?.info?.datPath) app.queue = [];
  app.other = state;
  otherVersions.apply(state.rev, state.delta);
}

export async function initState(): Promise<void> {
  Events.On("project:changed", (event) => applyState(event.data));
  Events.On("compare:changed", (event) => applyOther(event.data));
  Events.On("app:progress", (event) => {
    const progress = event.data;
    if ((app.busy || app.stoppable) && progress.done < progress.total) {
      app.busy = progress.percent ? `${progress.label} ${Math.floor((progress.done / progress.total) * 100)}%` : `${progress.label} ${progress.done.toLocaleString()}/${progress.total.toLocaleString()}`;
      app.progress = { done: progress.done, total: progress.total };
    }
  });
  applyState(await ProjectService.State());
  applyOther(await CompareService.State());
}

async function reloadDraft(): Promise<void> {
  const id = app.focused;
  if (id === null || !app.open) return;
  try {
    const thing = await ThingService.Get(app.category, id);
    if (thing && app.focused === id) setDraft(thing);
  } catch {
    /* thing vanished; selection is fixed by applyState */
  }
}

function setDraft(thing: Parameters<typeof normalizeThing>[0]): void {
  const plain = normalizeThing(thing);
  app.draft = plain;
  app.original = JSON.stringify(plain);
}

export async function select(id: number, additive = false): Promise<void> {
  if (!app.open) return;
  if (additive) {
    app.selection = app.selectionSet.has(id) ? app.selection.filter((selected) => selected !== id) : [...app.selection, id];
    return;
  }
  app.selection = [id];
  try {
    const thing = await ThingService.Get(app.category, id);
    if (thing && app.focused === id) setDraft(thing);
  } catch (err) {
    toast(errorMessage(err), "error");
  }
}

export async function setCategory(category: Category): Promise<void> {
  app.category = category;
  app.selection = [];
  await select(category === Category.CategoryItem ? 100 : 1);
}

export async function applyDraft(): Promise<void> {
  if (!app.draft || !app.dirty) return;
  const snapshot = $state.snapshot(app.draft);
  try {
    await ThingService.Update(snapshot as Thing);
    app.original = JSON.stringify(snapshot);
  } catch (err) {
    toast(errorMessage(err), "error");
  }
}

export function revertDraft(): void {
  if (app.original) app.draft = JSON.parse(app.original);
}
