// User actions of the map editor, shared by menus, toolbars and shortcuts.
import { Events } from "@wailsio/runtime";
import { DialogService, WindowService, encodeBytes, errorMessage } from "../../api";
import { clamp } from "../../util/math";
import { ask } from "../../confirm.svelte";
import { openContextMenu, type MenuEntry } from "../../menu.svelte";
import { app, toast } from "../../state/app.svelte";
import { MapService, SpawnFormat, type Position } from "../data/api";
import { MAX_FLOOR } from "../constants";
import { tileToWorld, worldToTile } from "./coords";
import { whenRendered } from "../render/rendered";
import { panTiles, startPosition, stepZoom, zoomAround } from "./camera";
import { applyChange, edit, loadMapData, map, refreshTile, selectBrush, setCam, warningCount, type Focus } from "../state.svelte";

const MAPS = [{ name: "OTBM maps (*.otbm)", pattern: "*.otbm" }];

/** Sprite size of the open client (world pixels per tile). */
export function spriteSize(): number {
  return app.project?.info.features.spriteSize || 32;
}

function needClient(): boolean {
  if (!app.open) {
    toast("Open a client first: the map editor draws with its sprites.");
    return false;
  }
  return true;
}

async function afterOpen(id: number, center?: Position): Promise<void> {
  map.activeId = id;
  if (!map.cams[id]) {
    const info = map.maps.find((open) => open.id === id);
    goTo(center ?? startPosition(info), id);
  }
}

/** Centers the view of a map on a position. */
export function goTo(position: Position, id = map.activeId): void {
  if (id === null) return;
  const center = tileToWorld(position, spriteSize());
  setCam(id, { x: center.x, y: center.y, floor: position.z, zoom: map.cams[id]?.zoom ?? 1 });
}

export const mapCommands = {
  /** Walks the map as a player, from a tile (default: the one under the mouse, or the middle of the view). */
  play(position?: Position) {
    if (map.activeId === null || !map.items) return;
    const cam = map.cam;
    map.play = position ?? map.hover ?? { ...worldToTile(cam.x, cam.y, cam.floor, spriteSize()), z: cam.floor };
  },

  /** Back to editing, the view on where the player stands. */
  exitPlay(position: Position) {
    map.play = null;
    goTo(position);
  },

  /** Moves the play view to its own window; editing goes on here. */
  popOutPlay(position: Position) {
    const id = map.activeId;
    map.play = null;
    if (id === null) return;
    goTo(position);
    void WindowService.OpenPlayWindow(`play=${id}&x=${position.x}&y=${position.y}&z=${position.z}`);
  },

  async setMode(mode: "objects" | "map") {
    map.mode = mode;
    if (mode === "map") await loadMapData();
  },

  /** Creates an empty map right away; its properties are in the inspector. */
  async newMap() {
    if (!needClient()) return;
    try {
      // Servers on client ids (no items.otb) keep monsters and NPCs in separate files.
      const separate = map.data?.mode === "client";
      const info = await MapService.NewMap({ width: 2048, height: 2048, spawnFormat: separate ? SpawnFormat.SpawnsSeparate : SpawnFormat.SpawnsSingle, description: "", fill: "" });
      await mapCommands.created(info.id);
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  },

  async open(path = "") {
    if (!needClient()) return;
    if (!path) {
      const paths = await DialogService.OpenFiles("openMap", "Open map", MAPS, false);
      if (!paths?.length) return;
      path = paths[0];
    }
    const name = path.split(/[\\/]/).pop() ?? path;
    map.loading = { name, step: "Starting", percent: 0 };
    const off = Events.On("map:progress", (ev: { data: { label: string; done: number; total: number } }) => {
      const progress = ev.data;
      if (map.loading) map.loading = { name, step: progress.label, percent: progress.total > 0 ? (progress.done / progress.total) * 100 : 0 };
    });
    const started = performance.now();
    try {
      const opened = await MapService.OpenMap(path);
      off();
      map.loading = { name, step: "Preparing the view", percent: 98 };
      await loadMapData(!map.items);
      if (!map.maps.some((open) => open.id === opened.info.id)) map.maps = [...map.maps, opened.info];
      await afterOpen(opened.info.id);
      // The loader stays until the first view is drawn, not on a black map.
      map.loading = { name, step: "Drawing the map", percent: 99 };
      await whenRendered(opened.info.id);
      const warnings = (opened.warnings ?? []).filter((x): x is string => !!x);
      map.loadWarnings = { ...map.loadWarnings, [opened.info.id]: warnings };
      // Whole open: reading the file, the item data and the first view.
      const took = `${((performance.now() - started) / 1000).toFixed(1)} s`;
      if (warnings.length) {
        const show = () => ((map.activeId = opened.info.id), (map.dialog = "warnings"));
        const count = warningCount(warnings);
        toast(`Opened ${opened.info.name} in ${took} with ${count.toLocaleString()} warning${count > 1 ? "s" : ""}.`, "info", 12000, undefined, { label: "Show warnings", run: show });
      } else toast(`Opened ${opened.info.name} in ${took}.`, "success");
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      off();
      map.loading = null;
    }
  },

  async created(id: number) {
    await loadMapData();
    await afterOpen(id);
  },

  async save(as = false): Promise<boolean> {
    const active = map.active;
    if (!active) return false;
    let path = active.path;
    if (as || !path) {
      path = await DialogService.SaveFile("saveMap", "Save map", active.name, MAPS);
      if (!path) return false;
    }
    const name = path.split(/[\\/]/).pop() ?? path;
    const title = "Saving map";
    map.loading = { name, step: "Starting", percent: 0, title };
    const off = Events.On("map:progress", (ev: { data: { label: string; done: number; total: number } }) => {
      const progress = ev.data;
      if (map.loading) map.loading = { name, step: progress.label, percent: progress.total > 0 ? (progress.done / progress.total) * 100 : 0, title };
    });
    try {
      const change = await MapService.SaveMap(active.id, path);
      off();
      map.loading = { name, step: "Reloading the item data", percent: 100, title };
      applyChange(active.id, change);
      await loadMapData();
      toast(`Saved ${change.info.name}.`, "success");
      return true;
    } catch (err) {
      toast(errorMessage(err), "error");
      return false;
    } finally {
      off();
      map.loading = null;
    }
  },

  async close(id = map.activeId) {
    const closing = map.maps.find((open) => open.id === id);
    if (!closing) return;
    if (closing.changed) {
      const ok = await ask({ title: "Unsaved map", message: `${closing.name} has unsaved changes. Close it anyway?`, ok: "Close" });
      if (!ok) return;
    }
    await MapService.CloseMap(closing.id);
    map.maps = map.maps.filter((open) => open.id !== closing.id);
    if (map.activeId === closing.id) map.activeId = map.maps[0]?.id ?? null;
    map.inspect = null;
    map.tile = null;
    map.focus = null;
  },

  /** Lists the undo history at the mouse; a click goes back or forward to that step. */
  async historyMenu(event: MouseEvent) {
    const id = map.activeId;
    if (id === null) return;
    const at = { clientX: event.clientX, clientY: event.clientY, preventDefault() {}, stopPropagation() {} } as MouseEvent;
    const history = await MapService.History(id);
    const undo = (history?.undo ?? []).filter((step) => !!step);
    const redo = (history?.redo ?? []).filter((step) => !!step);
    const steps = async (count: number, back: boolean) => {
      for (let i = 0; i < count; i++) applyChange(id, back ? await MapService.Undo(id) : await MapService.Redo(id));
    };
    const items: MenuEntry[] = [];
    // Newest first: redo steps (faded in the label), then what is applied.
    for (let i = redo.length - 1; i >= 0; i--) items.push({ label: `↻ ${redo[i].label} · ${redo[i].tiles} tiles`, action: () => steps(i + 1, false) });
    if (redo.length) items.push("-");
    for (let i = undo.length - 1; i >= 0; i--) items.push({ label: `${i === undo.length - 1 ? "● " : ""}${undo[i].label} · ${undo[i].tiles} tiles`, action: () => steps(undo.length - 1 - i, true) });
    items.push("-", { label: "The map as opened", action: () => steps(undo.length, true), disabled: () => !undo.length });
    openContextMenu(at, items.slice(0, 60));
  },

  undo: () => edit((id) => MapService.Undo(id)),
  redo: () => edit((id) => MapService.Redo(id)),

  async copy() {
    const id = map.activeId;
    if (id === null) return;
    try {
      const count = await MapService.Copy(id);
      toast(`Copied ${count} tile${count === 1 ? "" : "s"}.`);
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  },
  cut: () => edit((id) => MapService.Cut(id)),
  async startPaste() {
    if (map.activeId === null) return;
    const clip = await MapService.Clipboard();
    if (!clip?.tiles?.length) {
      toast("The clipboard is empty: copy a selection first.");
      return;
    }
    map.pasting = true;
    map.tool = "select";
  },
  remove: () => edit((id) => MapService.DeleteSelection(id)),
  selectAll: () => edit((id) => MapService.SelectAll(id, map.floor)),
  clearSelection: () => edit((id) => MapService.ClearSelection(id)),

  tool(name: string, inSelection = false) {
    return edit((id) => MapService.Tool(id, name, inSelection)).then((change) => {
      if (change && change.count >= 0 && name !== "clearModified") toast(`${change.count.toLocaleString()} changed.`);
    });
  },

  floor(z: number) {
    const id = map.activeId;
    if (id === null) return;
    setCam(id, { floor: clamp(z, 0, MAX_FLOOR) });
  },
  floorUp: () => mapCommands.floor(map.floor - 1),
  floorDown: () => mapCommands.floor(map.floor + 1),

  zoom(step: number, anchor?: { x: number; y: number; w: number; h: number }) {
    const id = map.activeId;
    if (id === null) return;
    const cam = map.cam;
    const next = stepZoom(cam.zoom, step);
    if (anchor) {
      // Keep the point under the cursor in place.
      const dpr = window.devicePixelRatio || 1;
      const offsetX = (anchor.x - anchor.w / 2) / dpr;
      const offsetY = (anchor.y - anchor.h / 2) / dpr;
      setCam(id, { zoom: next, ...zoomAround(cam.x, cam.y, cam.zoom, next, offsetX, offsetY) });
    } else setCam(id, { zoom: next });
  },
  zoomReset: () => map.activeId !== null && setCam(map.activeId, { zoom: 1 }),

  /** Moves the view by about dx, dy tiles at normal zoom: the further out, the more tiles (the same distance on screen). */
  pan(dx: number, dy: number) {
    const id = map.activeId;
    if (id === null) return;
    const tileSize = spriteSize();
    const zoom = map.cam.zoom;
    setCam(id, { x: map.cam.x + panTiles(dx, zoom) * tileSize, y: map.cam.y + panTiles(dy, zoom) * tileSize });
  },

  eraser() {
    selectBrush({ kind: "eraser", label: "Eraser" });
  },

  async toggleAutomagic() {
    const settings = map.settings;
    if (!settings) return;
    const next = { ...settings, automagic: !settings.automagic };
    await MapService.SetEditorSettings(next);
    map.settings = next;
    toast(next.automagic ? "Automatic borders and walls are on." : "Automatic borders and walls are off.", "info", 2000, "automagic");
  },

  inspect(position: Position | null) {
    map.inspect = position;
    map.focus = position ? { kind: "tile" } : null;
    void refreshTile();
  },

  /** Shows a part of the world (or the map with null) in the inspector. */
  focus(focus: Focus | null) {
    map.focus = focus;
  },

  find(preset: { ids?: number[]; inSelection?: boolean; replace?: boolean } | null = null) {
    if (map.activeId === null) return;
    map.findPreset = preset;
    map.findOpen = true;
  },

  commandPalette(text = "") {
    map.command = text;
  },

  async exportImage() {
    const canvas = document.querySelector<HTMLCanvasElement>("canvas.map-gl");
    if (!canvas) return;
    const path = await DialogService.SaveFile("screenshot", "Save screenshot", `${map.active?.name.replace(/\.otbm$/i, "") ?? "map"}.png`, [{ name: "PNG image (*.png)", pattern: "*.png" }]);
    if (!path) return;
    canvas.toBlob(async (blob) => {
      if (!blob) return;
      try {
        const bytes = new Uint8Array(await blob.arrayBuffer());
        await MapService.SaveImage(path, encodeBytes(bytes));
        toast("Screenshot saved.", "success");
      } catch (err) {
        toast(errorMessage(err), "error");
      }
    }, "image/png");
  },
};
