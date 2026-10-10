// Global UI state (Svelte 5 runes). Components read fields directly; only
// the fields they touch re-render.

import { Events } from "@wailsio/runtime";
import { Category, CompareService, ProjectService, ThingService, errorMessage, normalizeThing, type State, type Thing } from "./api";
import { otherSpriteCache, spriteCache, type SpriteCache } from "./render/sprites";

export type ToastKind = "info" | "success" | "error";
export interface Toast {
  id: number;
  kind: ToastKind;
  text: string;
  /** Toasts with the same key replace each other instead of stacking. */
  key?: string;
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
  | null;

/** What the share window publishes. */
export type ShareTarget = (
  | { kind: "object"; category: Category; id: number }
  | { kind: "sprites"; ids: number[] }
  /** An OBD file or an image from disk; works without an open client. */
  | { kind: "file"; path: string }
) & {
  /** The share window lets the user pick another object or other sprites. */
  pick?: boolean;
};

class AppState {
  project = $state<State | null>(null);
  /** Second client of the compare window (B). */
  other = $state<State | null>(null);
  category = $state<Category>(Category.CategoryItem);
  /** Selected thing ids in the current category (first = focused). */
  selection = $state<number[]>([]);
  /** Working copy of the focused thing; edits apply on "Apply". */
  draft = $state<Thing | null>(null);
  original = $state<string>("");
  selectedSprites = $state<number[]>([]);
  toasts = $state<Toast[]>([]);
  dialog = $state<DialogName>(null);
  busy = $state<string | null>(null);
  /** Progress of the running long operation, while it reports one. */
  progress = $state<{ done: number; total: number } | null>(null);
  /** The running operation can be stopped (commands.stopTask). */
  stoppable = $state(false);
  stopping = $state(false);
  /** Path for the open dialog (dropped file); "" asks for a folder. */
  openPath = $state("");
  /** The open dialog loads the second client of the compare window. */
  openOther = $state(false);
  /** Frame group shown by the sprite sheet export window. */
  sheetGroup = $state(0);
  /** File for the OBD viewer; "" asks for one. */
  obdPath = $state("");
  /** What the share window publishes. */
  shareTarget = $state<ShareTarget | null>(null);
  /** Objects of B queued in the compare window to be dropped into A. */
  queue = $state<{ category: Category; id: number }[]>([]);

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
  maxId(c: Category): number {
    const k = this.project?.info?.counts;
    if (!k) return 0;
    return [0, k.items, k.outfits, k.effects, k.missiles][c] ?? 0;
  }
}

export const app = new AppState();

/**
 * Cache keys of thumbnails and sprite images. A change bumps only the keys
 * of what it touched, so other images stay cached and do not flicker.
 */
class ResourceVersions {
  /** cache: the sprite cache that shows this client. */
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
      for (const t of delta.things ?? []) this.things.set(`${t.category}:${t.id}`, rev);
      for (const id of delta.sprites ?? []) this.sprites.set(id, rev);
      this.cache.invalidate(delta.sprites ?? []);
    }
    this.tick++;
  }

  thing(c: Category, id: number): string {
    void this.tick;
    return `${this.epoch}.${this.things.get(`${c}:${id}`) ?? 0}`;
  }

  sprite(id: number): string {
    void this.tick;
    return `${this.epoch}.${this.sprites.get(id) ?? 0}`;
  }
}

export const versions = new ResourceVersions(spriteCache);
/** Cache keys of the second client of the compare window. */
export const otherVersions = new ResourceVersions(otherSpriteCache);

let toastSeq = 0;
const toastTimers = new Map<number, ReturnType<typeof setTimeout>>();
/** Shows a toast. One with the key of a shown toast replaces it and keeps it up longer. */
export function toast(text: string, kind: ToastKind = "info", ms = 3500, key?: string): void {
  const old = key ? app.toasts.find((x) => x.key === key) : undefined;
  const t = old ? { ...old, kind, text } : { id: ++toastSeq, kind, text, key };
  app.toasts = old ? app.toasts.map((x) => (x.id === t.id ? t : x)) : [...app.toasts, t];
  clearTimeout(toastTimers.get(t.id));
  toastTimers.set(
    t.id,
    setTimeout(
      () => {
        app.toasts = app.toasts.filter((x) => x.id !== t.id);
        toastTimers.delete(t.id);
      },
      kind === "error" ? ms * 2 : ms,
    ),
  );
}

/** Whether a toast with the key is shown. */
export function hasToast(key: string): boolean {
  return app.toasts.some((x) => x.key === key);
}

/** Runs an async action with a busy indicator and error toast. */
export async function run<T>(label: string, fn: () => Promise<T>): Promise<T | undefined> {
  app.busy = label;
  try {
    return await fn();
  } catch (e) {
    toast(errorMessage(e), "error");
    return undefined;
  } finally {
    app.busy = null;
    app.progress = null;
  }
}

function applyState(s: State): void {
  const wasOpen = app.open;
  app.project = s;
  versions.apply(s.rev, s.delta);
  if (!s.open) {
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

function applyOther(s: State): void {
  // Queued ids belong to the client they were queued from.
  if (!s.open || s.info?.datPath !== app.other?.info?.datPath) app.queue = [];
  app.other = s;
  otherVersions.apply(s.rev, s.delta);
}

export async function initState(): Promise<void> {
  Events.On("project:changed", (ev) => applyState(ev.data));
  Events.On("compare:changed", (ev) => applyOther(ev.data));
  // Long operations report progress in the busy label.
  Events.On("app:progress", (ev) => {
    const p = ev.data;
    if ((app.busy || app.stoppable) && p.done < p.total) {
      app.busy = `${p.label} ${p.done.toLocaleString()}/${p.total.toLocaleString()}`;
      app.progress = { done: p.done, total: p.total };
    }
  });
  applyState(await ProjectService.State());
  applyOther(await CompareService.State());
}

async function reloadDraft(): Promise<void> {
  const id = app.focused;
  if (id === null || !app.open) return;
  try {
    const t = await ThingService.Get(app.category, id);
    if (t && app.focused === id) setDraft(t);
  } catch {
    /* thing vanished; selection is fixed by applyState */
  }
}

function setDraft(t: Parameters<typeof normalizeThing>[0]): void {
  const plain = normalizeThing(t);
  app.draft = plain;
  app.original = JSON.stringify(plain);
}

/** Focuses a thing. Unsaved draft changes are discarded. */
export async function select(id: number, additive = false): Promise<void> {
  if (!app.open) return;
  if (additive) {
    app.selection = app.selection.includes(id) ? app.selection.filter((x) => x !== id) : [...app.selection, id];
    return;
  }
  app.selection = [id];
  try {
    const t = await ThingService.Get(app.category, id);
    if (t && app.focused === id) setDraft(t);
  } catch (e) {
    toast(errorMessage(e), "error");
  }
}

export async function setCategory(c: Category): Promise<void> {
  app.category = c;
  app.selection = [];
  await select(c === Category.CategoryItem ? 100 : 1);
}

export async function applyDraft(): Promise<void> {
  if (!app.draft || !app.dirty) return;
  const d = $state.snapshot(app.draft);
  try {
    await ThingService.Update(d as Thing);
    app.original = JSON.stringify(d);
  } catch (e) {
    toast(errorMessage(e), "error");
  }
}

export function revertDraft(): void {
  if (app.original) app.draft = JSON.parse(app.original);
}
