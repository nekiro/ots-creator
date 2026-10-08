// Global UI state (Svelte 5 runes). Components read fields directly; only
// the fields they touch re-render.

import { Events } from "@wailsio/runtime";
import { Category, ProjectService, ThingService, errorMessage, normalizeThing, type State, type Thing } from "./api";
import { spriteCache } from "./render/sprites";

export type ToastKind = "info" | "success" | "error";
export interface Toast {
  id: number;
  kind: ToastKind;
  text: string;
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
  | null;

class AppState {
  project = $state<State | null>(null);
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
  /** Path for the open dialog (dropped file); "" asks for a folder. */
  openPath = $state("");
  /** Frame group shown by the sprite sheet export window. */
  sheetGroup = $state(0);
  /** File for the OBD viewer; "" asks for one. */
  obdPath = $state("");

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
  private tick = $state(0);
  private epoch = 0;
  private things = new Map<string, number>();
  private sprites = new Map<number, number>();

  apply(rev: number, delta: State["delta"]): void {
    if (!delta || delta.all) {
      this.epoch = rev;
      this.things.clear();
      this.sprites.clear();
      spriteCache.invalidate(null);
    } else {
      for (const t of delta.things ?? []) this.things.set(`${t.category}:${t.id}`, rev);
      for (const id of delta.sprites ?? []) this.sprites.set(id, rev);
      spriteCache.invalidate(delta.sprites ?? []);
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

export const versions = new ResourceVersions();

let toastSeq = 0;
export function toast(text: string, kind: ToastKind = "info", ms = 3500): void {
  const t = { id: ++toastSeq, kind, text };
  app.toasts = [...app.toasts, t];
  setTimeout(() => (app.toasts = app.toasts.filter((x) => x.id !== t.id)), kind === "error" ? ms * 2 : ms);
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

export async function initState(): Promise<void> {
  Events.On("project:changed", (ev) => applyState(ev.data));
  applyState(await ProjectService.State());
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
