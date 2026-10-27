import { errorMessage } from "../api";
import { loadJson, saveJson } from "../state/storage";
import { toast } from "../state/app.svelte";
import { MapService, type DataInfo, type EditorSettings, type MapChange, type MapInfo, type Palette, type Position, type RecentMap, type TileInfo } from "./data/api";
import { ItemIndex } from "./data/items";
import type { ViewOptions } from "./render/scene";
import { GROUND_FLOOR } from "./constants";

export type EditorMode = "objects" | "map";
export type Tool = "select" | "brush";

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
  look?: number;
  label: string;
}

export type MapDialog = "data" | "import" | "minimap" | "brush" | "settings" | "tools" | "shortcuts" | "warnings" | "text" | null;

export type Focus =
  | { kind: "tile" }
  | { kind: "map" }
  | { kind: "town"; id: number }
  | { kind: "house"; id: number }
  | { kind: "waypoint"; name: string }
  | { kind: "zone"; id: number };

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

const DEFAULT_CAMERA: Readonly<Camera> = { x: 0, y: 0, zoom: 1, floor: GROUND_FLOOR };

const VIEW_KEY = "map:view";
const BRUSH_KEY = "map:brush";

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
  markedItems: true,
  clientBox: false,
  lights: false,
  ambientLight: 15,
};

class MapState {
  /** Not remembered: each start shows the start screen, which picks the editor. */
  mode = $state<EditorMode>("objects");
  data = $state<DataInfo | null>(null);
  maps = $state.raw<MapInfo[]>([]);
  recent = $state<RecentMap[]>([]);
  activeId = $state<number | null>(null);
  settings = $state<EditorSettings | null>(null);
  /** Keyboard shortcuts changed by the user: action id -> key combinations. */
  keys = $state.raw<Record<string, string[]>>({});
  loadWarnings = $state.raw<Record<number, string[]>>({});
  settingsTab = $state<"general" | "keys">("general");
  items = $state.raw<ItemIndex | null>(null);
  palette = $state.raw<Palette | null>(null);
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
  reroll = $state(0);
  recentBrushes = $state<BrushSel[]>([]);
  /** Item the library should show and scroll to (set by picking a brush from the map). */
  revealItem = $state<{ id: number } | null>(null);

  view = $state<ViewOptions>(loadJson(VIEW_KEY, DEFAULT_VIEW));
  cams = $state<Record<number, Camera>>({});
  hover = $state.raw<Position | null>(null);
  inspect = $state.raw<Position | null>(null);
  tile = $state.raw<TileInfo | null>(null);
  /** Inspector content; null shows the map. */
  focus = $state<Focus | null>(null);
  side = $state<SidePanel>("library");
  findOpen = $state(false);
  findPreset = $state<{ ids?: number[]; inSelection?: boolean; replace?: boolean } | null>(null);
  results = $state.raw<{ title: string; items: FindResultRow[]; more: boolean } | null>(null);
  /** Command palette: null closed, otherwise the text it opens with. */
  command = $state<string | null>(null);
  dialog = $state<MapDialog>(null);
  /** Item shown in the text dialog (index among the tile items, ground excluded). */
  textItem = $state<{ pos: Position; index: number } | null>(null);
  editBrush = $state<{ name: string; type: string } | null>(null);
  pasting = $state(false);
  /** The play view shows the map in place of the editor, the player starting here (null: editing). */
  play = $state<Position | null>(null);
  /** Where the player of the play view stands now (null: not playing here). */
  player = $state<Position | null>(null);
  /** A click on the map picks a position for this callback (teleport destinations). */
  pick = $state<{ label: string; done: (position: Position) => void } | null>(null);
  busy = $state<string | null>(null);
  loading = $state<{ name: string; step: string; percent: number; title?: string } | null>(null);

  readonly active = $derived<MapInfo | null>(this.maps.find((open) => open.id === this.activeId) ?? null);

  get cam(): Camera {
    const id = this.activeId;
    return (id !== null && this.cams[id]) || DEFAULT_CAMERA;
  }

  get floor(): number {
    return this.cam.floor;
  }
}

export const map = new MapState();

$effect.root(() => {
  $effect(() => saveJson(VIEW_KEY, map.view));
  $effect(() => saveJson(BRUSH_KEY, { size: map.size, shape: map.shape }));
});

{
  const brush = loadJson(BRUSH_KEY, { size: 0, shape: "square" as "square" | "circle" });
  map.size = brush.size;
  map.shape = brush.shape;
}

type ChangeListener = (id: number, change: MapChange) => void;
const listeners = new Set<ChangeListener>();

export function onMapChange(listener: ChangeListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function applyChange(id: number, change: MapChange | null | undefined): void {
  if (!change) return;
  const info = change.info;
  if (info) {
    const index = map.maps.findIndex((open) => open.id === id);
    if (index >= 0) {
      // Towns, houses and waypoints are only sent with meta changes.
      const old = map.maps[index];
      map.maps = map.maps.with(index, change.meta ? info : { ...info, towns: old.towns, houses: old.houses, waypoints: old.waypoints, zones: old.zones, bounds: old.bounds, hasBounds: old.hasBounds });
    }
  }
  for (const listener of listeners) listener(id, change);
  for (const warning of change.warnings ?? []) toast(warning, "info", 6000);
  if (map.inspect && id === map.activeId) void refreshTile();
}

export async function edit(call: (id: number) => Promise<MapChange>): Promise<MapChange | null> {
  const id = map.activeId;
  if (id === null) return null;
  try {
    const change = await call(id);
    applyChange(id, change);
    return change;
  } catch (err) {
    toast(errorMessage(err), "error");
    return null;
  }
}

export async function refreshTile(): Promise<void> {
  const id = map.activeId;
  const position = map.inspect;
  if (id === null || !position) {
    map.tile = null;
    return;
  }
  try {
    const tile = await MapService.Tile(id, position);
    if (map.inspect === position) map.tile = tile;
  } catch {
    map.tile = null;
  }
}

let dataRequest = 0;

export async function loadMapData(force = false, activate = true): Promise<void> {
  const request = ++dataRequest;
  try {
    const status = await MapService.Status();
    if (request !== dataRequest) return;
    map.recent = status.recent ?? [];
    map.settings = status.settings;
    map.keys = Object.fromEntries(Object.entries(status.keys ?? {}).map(([key, keys]) => [key, keys ?? []]));
    map.maps = status.maps ?? [];
    if (map.activeId === null || !map.maps.some((open) => open.id === map.activeId)) map.activeId = map.maps[0]?.id ?? null;
    const id = map.activeId;
    if (!status.data?.ready) {
      map.data = status.data;
      map.items = null;
      map.palette = null;
      return;
    }
    const view = await MapService.ViewData(id ?? 0, activate);
    if (request !== dataRequest || id !== map.activeId) return;
    const modeChanged = !!map.data && map.data.mode !== view.data.mode;
    map.data = view.data;
    if (force || !map.items || modeChanged) {
      map.items = new ItemIndex(view.items);
      map.palette = view.palette;
      map.dataRev++;
    }
    if (modeChanged) {
      map.brush = null;
      map.tool = "select";
      map.recentBrushes = [];
      map.pasting = false;
      map.results = null;
    }
  } catch (err) {
    if (request === dataRequest) toast(errorMessage(err), "error");
  }
}

export async function reloadPalette(): Promise<void> {
  try {
    map.palette = await MapService.Palette();
    map.dataRev++;
  } catch (err) {
    toast(errorMessage(err), "error");
  }
}

/** Warnings in a load warning list: "N more not listed: …" lines count N. */
export function warningCount(list: string[] | undefined): number {
  let count = 0;
  for (const warning of list ?? []) count += Number(warning.match(/^(\d+) more not listed: /)?.[1] ?? 1);
  return count;
}

export function setCam(id: number, change: Partial<Camera>): void {
  const current = map.cams[id] ?? DEFAULT_CAMERA;
  map.cams[id] = { ...current, ...change };
}

export function selectBrush(brush: BrushSel): void {
  map.brush = brush;
  map.tool = "brush";
  map.pasting = false;
  if (brush.kind !== "raw" || brush.item) {
    map.recentBrushes = [brush, ...map.recentBrushes.filter((recent) => brushKey(recent) !== brushKey(brush))].slice(0, 12);
  }
}

export function useCreature(creature: { name: string; npc?: boolean }): void {
  selectBrush({ kind: "creature", creature: creature.name, npc: !!creature.npc, label: creature.name });
}

function brushKey(brush: BrushSel): string {
  return `${brush.kind}:${brush.name ?? ""}:${brush.item ?? ""}:${brush.house ?? ""}:${brush.creature ?? ""}:${brush.flag ?? ""}:${brush.zone ?? ""}:${brush.waypoint ?? ""}:${brush.doorType ?? ""}`;
}

export function isBrushActive(brush: BrushSel): boolean {
  return !!map.brush && map.tool === "brush" && brushKey(map.brush) === brushKey(brush);
}

export function creatureOutfit(name: string | undefined): number {
  if (!name) return 0;
  return map.palette?.creatures?.find((creature) => creature?.name === name)?.outfit.lookType ?? 0;
}
