// User actions of the map editor, shared by menus, toolbars and shortcuts.
import { Events } from "@wailsio/runtime";
import { DialogService, encodeBytes, errorMessage } from "../api";
import { ask } from "../confirm.svelte";
import { openContextMenu, type MenuEntry } from "../menu.svelte";
import { app, toast } from "../state.svelte";
import { MapService, SpawnFormat, pos, type Position } from "./api";
import { whenRendered } from "./rendered";
import { applyChange, edit, loadMapData, map, refreshTile, selectBrush, setCam, warningCount, ZOOMS, type Focus } from "./state.svelte";

const MAPS = [{ name: "OTBM maps (*.otbm)", pattern: "*.otbm" }];

/** Sprite size of the open client (world pixels per tile). */
export function spriteSize(): number {
  return app.project?.info.features.spriteSize || 32;
}

/** Tile offset of a floor when drawn from the current floor. */
export function floorAdjust(z: number, floor: number): number {
  return floor <= 7 ? 7 - z : floor - z;
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
    const info = map.maps.find((m) => m.id === id);
    let c = center;
    if (!c && info?.hasBounds) {
      const [x0, y0, x1, y1] = info.bounds;
      c = pos(Math.round((x0 + x1) / 2), Math.round((y0 + y1) / 2), 7);
      // Prefer a temple: towns are where people start.
      const t = info.towns?.[0];
      if (t && t.temple.x) c = t.temple;
    }
    c ??= pos(Math.min(1000, Math.floor((info?.props.width ?? 2048) / 2)), Math.min(1000, Math.floor((info?.props.height ?? 2048) / 2)), 7);
    goTo(c, id);
  }
}

/** Centers the view of a map on a position. */
export function goTo(p: Position, id = map.activeId): void {
  if (id === null) return;
  const S = spriteSize();
  const a = p.z <= 7 ? 7 - p.z : 0;
  setCam(id, { x: (p.x - a + 0.5) * S, y: (p.y - a + 0.5) * S, floor: p.z, zoom: map.cams[id]?.zoom ?? 1 });
}

export const mapCommands = {
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
    } catch (e) {
      toast(errorMessage(e), "error");
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
      const p = ev.data;
      if (map.loading) map.loading = { name, step: p.label, percent: p.total > 0 ? (p.done / p.total) * 100 : 0 };
    });
    const started = performance.now();
    try {
      const r = await MapService.OpenMap(path);
      off();
      map.loading = { name, step: "Preparing the view", percent: 98 };
      await loadMapData(!map.items);
      if (!map.maps.some((m) => m.id === r.info.id)) map.maps = [...map.maps, r.info];
      await afterOpen(r.info.id);
      // The loader stays until the first view is drawn, not on a black map.
      map.loading = { name, step: "Drawing the map", percent: 99 };
      await whenRendered(r.info.id);
      const w = (r.warnings ?? []).filter((x): x is string => !!x);
      map.loadWarnings = { ...map.loadWarnings, [r.info.id]: w };
      // Whole open: reading the file, the item data and the first view.
      const took = `${((performance.now() - started) / 1000).toFixed(1)} s`;
      if (w.length) {
        const show = () => ((map.activeId = r.info.id), (map.dialog = "warnings"));
        const n = warningCount(w);
        toast(`Opened ${r.info.name} in ${took} with ${n.toLocaleString()} warning${n > 1 ? "s" : ""}.`, "info", 12000, undefined, { label: "Show warnings", run: show });
      } else toast(`Opened ${r.info.name} in ${took}.`, "success");
    } catch (e) {
      toast(errorMessage(e), "error");
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
    const m = map.active;
    if (!m) return false;
    let path = m.path;
    if (as || !path) {
      path = await DialogService.SaveFile("saveMap", "Save map", m.name, MAPS);
      if (!path) return false;
    }
    const name = path.split(/[\\/]/).pop() ?? path;
    const title = "Saving map";
    map.loading = { name, step: "Starting", percent: 0, title };
    const off = Events.On("map:progress", (ev: { data: { label: string; done: number; total: number } }) => {
      const p = ev.data;
      if (map.loading) map.loading = { name, step: p.label, percent: p.total > 0 ? (p.done / p.total) * 100 : 0, title };
    });
    try {
      const ch = await MapService.SaveMap(m.id, path);
      off();
      map.loading = { name, step: "Reloading the item data", percent: 100, title };
      applyChange(m.id, ch);
      await loadMapData();
      toast(`Saved ${ch.info.name}.`, "success");
      return true;
    } catch (e) {
      toast(errorMessage(e), "error");
      return false;
    } finally {
      off();
      map.loading = null;
    }
  },

  async close(id = map.activeId) {
    const m = map.maps.find((x) => x.id === id);
    if (!m) return;
    if (m.changed) {
      const ok = await ask({ title: "Unsaved map", message: `${m.name} has unsaved changes. Close it anyway?`, ok: "Close" });
      if (!ok) return;
    }
    await MapService.CloseMap(m.id);
    map.maps = map.maps.filter((x) => x.id !== m.id);
    if (map.activeId === m.id) map.activeId = map.maps[0]?.id ?? null;
    map.inspect = null;
    map.tile = null;
    map.focus = null;
  },

  /** Lists the undo history at the mouse; a click goes back or forward to that step. */
  async historyMenu(e: MouseEvent) {
    const id = map.activeId;
    if (id === null) return;
    const ev = { clientX: e.clientX, clientY: e.clientY, preventDefault() {}, stopPropagation() {} } as MouseEvent;
    const h = await MapService.History(id);
    const undo = (h?.undo ?? []).filter((x) => !!x);
    const redo = (h?.redo ?? []).filter((x) => !!x);
    const steps = async (n: number, back: boolean) => {
      for (let i = 0; i < n; i++) applyChange(id, back ? await MapService.Undo(id) : await MapService.Redo(id));
    };
    const items: MenuEntry[] = [];
    // Newest first: redo steps (faded in the label), then what is applied.
    for (let i = redo.length - 1; i >= 0; i--) items.push({ label: `↻ ${redo[i].label} · ${redo[i].tiles} tiles`, action: () => steps(i + 1, false) });
    if (redo.length) items.push("-");
    for (let i = undo.length - 1; i >= 0; i--) items.push({ label: `${i === undo.length - 1 ? "● " : ""}${undo[i].label} · ${undo[i].tiles} tiles`, action: () => steps(undo.length - 1 - i, true) });
    items.push("-", { label: "The map as opened", action: () => steps(undo.length, true), disabled: () => !undo.length });
    openContextMenu(ev, items.slice(0, 60));
  },

  undo: () => edit((id) => MapService.Undo(id)),
  redo: () => edit((id) => MapService.Redo(id)),

  async copy() {
    const id = map.activeId;
    if (id === null) return;
    try {
      const n = await MapService.Copy(id);
      toast(`Copied ${n} tile${n === 1 ? "" : "s"}.`);
    } catch (e) {
      toast(errorMessage(e), "error");
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
    return edit((id) => MapService.Tool(id, name, inSelection)).then((ch) => {
      if (ch && ch.count >= 0 && name !== "clearModified") toast(`${ch.count.toLocaleString()} changed.`);
    });
  },

  floor(z: number) {
    const id = map.activeId;
    if (id === null) return;
    setCam(id, { floor: Math.min(15, Math.max(0, z)) });
  },
  floorUp: () => mapCommands.floor(map.floor - 1),
  floorDown: () => mapCommands.floor(map.floor + 1),

  zoom(step: number, anchor?: { x: number; y: number; w: number; h: number }) {
    const id = map.activeId;
    if (id === null) return;
    const cam = map.cam;
    let i = ZOOMS.findIndex((z) => z >= cam.zoom - 1e-6);
    if (i < 0) i = ZOOMS.length - 1;
    const next = ZOOMS[Math.min(ZOOMS.length - 1, Math.max(0, i + step))];
    if (anchor) {
      // Keep the point under the cursor in place.
      const dpr = window.devicePixelRatio || 1;
      const ox = (anchor.x - anchor.w / 2) / dpr;
      const oy = (anchor.y - anchor.h / 2) / dpr;
      setCam(id, { zoom: next, x: cam.x + ox / cam.zoom - ox / next, y: cam.y + oy / cam.zoom - oy / next });
    } else setCam(id, { zoom: next });
  },
  zoomReset: () => map.activeId !== null && setCam(map.activeId, { zoom: 1 }),

  /** Moves the view by about dx, dy tiles at normal zoom: the further out, the more tiles (the same distance on screen). */
  pan(dx: number, dy: number) {
    const id = map.activeId;
    if (id === null) return;
    const S = spriteSize();
    const k = 1 / map.cam.zoom;
    const tiles = (n: number) => (n ? Math.sign(n) * Math.max(1, Math.round(Math.abs(n) * k)) : 0);
    setCam(id, { x: map.cam.x + tiles(dx) * S, y: map.cam.y + tiles(dy) * S });
  },

  eraser() {
    selectBrush({ kind: "eraser", label: "Eraser" });
  },

  async toggleAutomagic() {
    const s = map.settings;
    if (!s) return;
    const next = { ...s, automagic: !s.automagic };
    await MapService.SetEditorSettings(next);
    map.settings = next;
    toast(next.automagic ? "Automatic borders and walls are on." : "Automatic borders and walls are off.", "info", 2000, "automagic");
  },

  inspect(p: Position | null) {
    map.inspect = p;
    map.focus = p ? { kind: "tile" } : null;
    void refreshTile();
  },

  /** Shows a part of the world (or the map with null) in the inspector. */
  focus(f: Focus | null) {
    map.focus = f;
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
      } catch (e) {
        toast(errorMessage(e), "error");
      }
    }, "image/png");
  },
};
