// State of the map editor (Svelte 5 runes).
import { errorMessage } from "../api";
import { toast } from "../state.svelte";
import { MapService, type DataInfo, type EditorSettings, type MapChange, type MapInfo, type Palette, type Position, type RecentMap, type TileInfo } from "./api";
import { ItemIndex } from "./items";
import type { ViewOptions } from "./renderer";

export type EditorMode = "objects" | "map";
export type Tool = "select" | "brush";

/** The brush in use. Kind matches mapedit draw kinds. */
export interface BrushSel {
  kind: string;
  name?: string;
  item?: number;
  house?: number;
  waypoint?: string;
  creature?: string;
  npc?: boolean;
  flag?: number;
  zone?: number;
  doorType?: string;
  /** Item shown for the brush (server id). */
  look?: number;
  label: string;
}

export type MapDialog = "data" | "import" | "minimap" | "brush" | "settings" | "tools" | "shortcuts" | "warnings" | null;

/** What the inspector shows: a tile, a part of the world, or the map itself. */
export type Focus =
  | { kind: "tile" }
  | { kind: "map" }
  | { kind: "town"; id: number }
  | { kind: "house"; id: number }
  | { kind: "waypoint"; name: string }
  | { kind: "zone"; id: number };

/** Left panel: what to draw with, or what the map contains. */
export type SidePanel = "library" | "world";

export interface FindResultRow {
  pos: Position;
  label: string;
  detail: string;
}

export interface Camera {
  x: number;
  y: number;
  zoom: number;
  floor: number;
}

const VIEW_KEY = "map:view";
const BRUSH_KEY = "map:brush";
const MODE_KEY = "map:mode";

export const DEFAULT_VIEW: ViewOptions = {
  grid: false,
  lowerFloors: true,
  ghostHigher: false,
  shade: true,
  creatures: true,
  labels: true,
  spawns: true,
  houses: true,
  special: true,
  zones: false,
  pathing: false,
  animation: true,
  onlyModified: false,
  highlightItems: false,
  wallHooks: false,
  markers: true,
  tooltips: false,
  coords: false,
  ghostItems: false,
  clientBox: false,
  lights: false,
};

function load<T>(key: string, fallback: T): T {
  try {
    const v = localStorage.getItem(key);
    if (v) return { ...fallback, ...JSON.parse(v) } as T;
  } catch {
    /* storage unavailable */
  }
  return fallback;
}

function save(key: string, v: unknown): void {
  try {
    localStorage.setItem(key, JSON.stringify(v));
  } catch {
    /* ignore */
  }
}

/** Zoom steps (device pixels per world pixel, before the screen scale). */
export const ZOOMS = [0.03125, 0.0625, 0.125, 0.1875, 0.25, 0.375, 0.5, 0.75, 1, 1.5, 2, 3, 4, 6, 8];

class MapState {
  mode = $state<EditorMode>(load<{ mode: EditorMode }>(MODE_KEY, { mode: "objects" }).mode);
  data = $state<DataInfo | null>(null);
  maps = $state<MapInfo[]>([]);
  recent = $state<RecentMap[]>([]);
  activeId = $state<number | null>(null);
  settings = $state<EditorSettings | null>(null);
  /** Keyboard shortcuts changed by the user: action id -> key combinations. */
  keys = $state.raw<Record<string, string[]>>({});
  /** Warnings from loading each open map, by map id. */
  loadWarnings = $state.raw<Record<number, string[]>>({});
  /** Tab the settings window opens on. */
  settingsTab = $state<"general" | "keys">("general");
  items =$state.raw<ItemIndex | null>(null);
  palette = $state.raw<Palette | null>(null);
  /** Bumped when the item table or palette is reloaded. */
  dataRev = $state(0);

  /** Selection until a brush is picked: a click must not draw by surprise. */
  tool = $state<Tool>("select");
  brush = $state<BrushSel | null>(null);
  size = $state(0);
  shape = $state<"square" | "circle">("square");
  variation = $state(0);
  /** Doodad density override (0: the brush's own). */
  thickness = $state(0);
  spawnTime = $state(0);
  direction = $state(2);
  fill = $state(false);
  /** Bumped to generate a new doodad placement. */
  reroll = $state(0);
  recentBrushes = $state<BrushSel[]>([]);

  view = $state<ViewOptions>(load(VIEW_KEY, DEFAULT_VIEW));
  cams = $state<Record<number, Camera>>({});
  hover = $state<Position | null>(null);
  /** Tile shown in the inspector. */
  inspect = $state<Position | null>(null);
  tile = $state<TileInfo | null>(null);
  /** Inspector content; null shows the map. */
  focus = $state<Focus | null>(null);
  side = $state<SidePanel>("library");
  /** Find panel: open, the values it opens with, and its results. */
  findOpen = $state(false);
  findPreset = $state<{ ids?: number[]; inSelection?: boolean; replace?: boolean } | null>(null);
  results = $state<{ title: string; items: FindResultRow[]; more: boolean } | null>(null);
  /** Command palette: null closed, otherwise the text it opens with. */
  command = $state<string | null>(null);
  dialog = $state<MapDialog>(null);
  /** Brush edited in the brush editor ("" for a new brush). */
  editBrush = $state<{ name: string; type: string } | null>(null);
  /** The clipboard follows the mouse until a click pastes it. */
  pasting = $state(false);
  /** A click on the map picks a position for this callback (teleport destinations). */
  pick = $state<{ label: string; done: (p: Position) => void } | null>(null);
  busy = $state<string | null>(null);
  /** A map being opened: its file name, the current step and how far the whole load is (0-100). */
  loading = $state<{ name: string; step: string; percent: number } | null>(null);

  get active(): MapInfo | null {
    return this.maps.find((m) => m.id === this.activeId) ?? null;
  }

  get cam(): Camera {
    const id = this.activeId;
    return (id !== null && this.cams[id]) || { x: 0, y: 0, zoom: 1, floor: 7 };
  }

  get floor(): number {
    return this.cam.floor;
  }
}

export const map = new MapState();

$effect.root(() => {
  $effect(() => save(VIEW_KEY, map.view));
  $effect(() => save(MODE_KEY, { mode: map.mode }));
  $effect(() => save(BRUSH_KEY, { size: map.size, shape: map.shape }));
});

{
  const b = load(BRUSH_KEY, { size: 0, shape: "square" as "square" | "circle" });
  map.size = b.size;
  map.shape = b.shape;
}

/** Listeners of map changes (the canvas redraws touched chunks). */
type ChangeListener = (id: number, ch: MapChange) => void;
const listeners = new Set<ChangeListener>();

export function onMapChange(fn: ChangeListener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/** Applies a change from the backend: map info and redraw. */
export function applyChange(id: number, ch: MapChange | null | undefined): void {
  if (!ch) return;
  const info = ch.info;
  if (info) {
    const i = map.maps.findIndex((m) => m.id === id);
    if (i >= 0) {
      // Towns, houses and waypoints are only sent with meta changes.
      const old = map.maps[i];
      map.maps[i] = ch.meta ? info : { ...info, towns: old.towns, houses: old.houses, waypoints: old.waypoints, zones: old.zones, bounds: old.bounds, hasBounds: old.hasBounds };
    }
  }
  for (const fn of listeners) fn(id, ch);
  for (const w of ch.warnings ?? []) toast(w, "info", 6000);
  if (map.inspect && id === map.activeId) void refreshTile();
}

/** Runs a backend call that returns a change of the active map. */
export async function edit(fn: (id: number) => Promise<MapChange>): Promise<MapChange | null> {
  const id = map.activeId;
  if (id === null) return null;
  try {
    const ch = await fn(id);
    applyChange(id, ch);
    return ch;
  } catch (e) {
    toast(errorMessage(e), "error");
    return null;
  }
}

export async function refreshTile(): Promise<void> {
  const id = map.activeId;
  const p = map.inspect;
  if (id === null || !p) {
    map.tile = null;
    return;
  }
  try {
    const t = await MapService.Tile(id, p);
    if (map.inspect === p) map.tile = t;
  } catch {
    map.tile = null;
  }
}

/** Loads status, the item table and the palette. */
export async function loadMapData(force = false): Promise<void> {
  try {
    const st = await MapService.Status();
    map.data = st.data;
    map.recent = st.recent ?? [];
    map.settings = st.settings;
    map.keys = Object.fromEntries(Object.entries(st.keys ?? {}).map(([k, v]) => [k, v ?? []]));
    map.maps = st.maps ?? [];
    if (map.activeId === null || !map.maps.some((m) => m.id === map.activeId)) map.activeId = map.maps[0]?.id ?? null;
    if (st.data?.ready && (force || !map.items)) {
      const [items, palette] = await Promise.all([MapService.Items(), MapService.Palette()]);
      map.items = new ItemIndex(items);
      map.palette = palette;
      map.dataRev++;
    }
    if (!st.data?.ready) {
      map.items = null;
      map.palette = null;
    }
  } catch (e) {
    toast(errorMessage(e), "error");
  }
}

/** Reloads the palette (after brush changes). */
export async function reloadPalette(): Promise<void> {
  try {
    map.palette = await MapService.Palette();
    map.dataRev++;
  } catch (e) {
    toast(errorMessage(e), "error");
  }
}

/** Warnings in a load warning list: "N more not listed: …" lines count N. */
export function warningCount(list: string[] | undefined): number {
  let n = 0;
  for (const w of list ?? []) n += Number(w.match(/^(\d+) more not listed: /)?.[1] ?? 1);
  return n;
}

export function setCam(id: number, c: Partial<Camera>): void {
  const cur = map.cams[id] ?? { x: 0, y: 0, zoom: 1, floor: 7 };
  map.cams[id] = { ...cur, ...c };
}

export function selectBrush(b: BrushSel): void {
  map.brush = b;
  map.tool = "brush";
  map.pasting = false;
  if (b.kind !== "raw" || b.item) {
    map.recentBrushes = [b, ...map.recentBrushes.filter((r) => brushKey(r) !== brushKey(b))].slice(0, 12);
  }
}

/** Selects a creature brush (from a creature on the map or the palette). */
export function useCreature(c: { name: string; npc?: boolean }): void {
  selectBrush({ kind: "creature", creature: c.name, npc: !!c.npc, label: c.name });
}

function brushKey(x: BrushSel): string {
  return `${x.kind}:${x.name ?? ""}:${x.item ?? ""}:${x.house ?? ""}:${x.creature ?? ""}:${x.flag ?? ""}:${x.zone ?? ""}:${x.waypoint ?? ""}:${x.doorType ?? ""}`;
}

/** True when b is the brush in use. */
export function isBrushActive(b: BrushSel): boolean {
  return !!map.brush && map.tool === "brush" && brushKey(map.brush) === brushKey(b);
}

/** Outfit look type of a creature brush (0 when unknown). */
export function creatureOutfit(name: string | undefined): number {
  if (!name) return 0;
  return map.palette?.creatures?.find((c) => c?.name === name)?.outfit.lookType ?? 0;
}
