<script lang="ts">
  import { onMount } from "svelte";
  import { errorMessage } from "../../lib/api";
  import { openContextMenu, type MenuEntry } from "../../lib/menu.svelte";
  import { spriteCache } from "../../lib/render/sprites";
  import { app, toast } from "../../lib/state.svelte";
  import { MapService, fmtPos, pos, type Clip, type DoodadBuffer, type DrawRequest, type Position } from "../../lib/map/api";
  import { Appearances } from "../../lib/map/appearances";
  import { ChunkStore, IF, TF, keyOf, type CreatureData } from "../../lib/map/chunks";
  import { GeoStore } from "../../lib/map/geo";
  import { rendered } from "../../lib/map/rendered";
  import { floorAdjust, mapCommands, spriteSize } from "../../lib/map/commands";
  import { MapRenderer } from "../../lib/map/renderer";
  import type { GhostTile, Label, Marker, RectMark } from "../../lib/map/scene";
  import { applyChange, edit, map, onMapChange, selectBrush, setCam, useCreature, type BrushSel } from "../../lib/map/state.svelte";
  import { mapContextMenu, openText, readableIndex } from "../../lib/map/menus";
  import FloorLadder from "./FloorLadder.svelte";
  import SelectionBar from "./SelectionBar.svelte";
  import ZoomControl from "./ZoomControl.svelte";

  let { hoverText = $bindable("") }: { hoverText?: string } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let overlay = $state<HTMLCanvasElement>();
  let host = $state<HTMLDivElement>();
  let renderer: MapRenderer | null = null;
  let glError = $state("");
  let width = 0;
  let height = 0;

  const stores = new Map<number, ChunkStore>();
  const geos = new Map<number, GeoStore>();
  const looks = new Appearances(() => requestDraw());

  function store(id: number): ChunkStore {
    let s = stores.get(id);
    if (!s) {
      s = new ChunkStore(id, () => requestDraw());
      stores.set(id, s);
    }
    return s;
  }

  function geoOf(id: number): GeoStore {
    let g = geos.get(id);
    if (!g) {
      g = new GeoStore(id, () => requestDraw());
      geos.set(id, g);
    }
    return g;
  }

  // ----- drawing -----

  let raf = 0;
  let animTimer = 0;
  function requestDraw() {
    if (raf) return;
    raf = requestAnimationFrame(draw);
  }

  /** Ghost tiles of the brush, paste or doodad preview. */
  let ghosts: GhostTile[] = [];
  let rects: RectMark[] = [];
  let doodad: DoodadBuffer | null = null;
  let clip: Clip | null = null;

  // Frames drawn in the last second. The map draws on demand, so it reads 0 when nothing changes.
  let frameTimes: number[] = [];
  let fps = $state(0);
  /** Chunk geometry loading in the last second (shown next to the FPS). */
  let loadInfo = $state("");
  $effect(() => {
    const t = setInterval(() => {
      const now = performance.now();
      frameTimes = frameTimes.filter((f) => now - f < 1000);
      fps = frameTimes.length;
      const id = map.activeId;
      const st = id !== null ? geos.get(id)?.stats() : undefined;
      loadInfo = st?.count ? `${st.count} req/s · ${Math.round(st.total)} ms (Go ${st.build.toFixed(1)} ms)` : "";
    }, 250);
    return () => clearInterval(t);
  });

  function draw() {
    raf = 0;
    frameTimes.push(performance.now());
    const id = map.activeId;
    if (!renderer || id === null || !map.items || !width) return;
    const cam = map.cam;
    const S = spriteSize();
    const dpr = window.devicePixelRatio || 1;
    const scale = cam.zoom * dpr;
    const info = map.active;
    const markers: Marker[] = [];
    if (info) {
      // The color tells the kind (green waypoint, blue house entrance, gold
      // temple), so labels carry only a short name.
      for (const w of info.waypoints ?? []) if (w) markers.push({ ...w.pos, label: shortLabel(w.name), full: w.name, color: [0.3, 1, 0.4] });
      for (const h of info.houses ?? []) if (h.exit && (h.exit.x || h.exit.y)) markers.push({ ...h.exit, label: shortLabel(houseLabel(h.name)), full: h.name, color: [0.4, 0.7, 1] });
      for (const t of info.towns ?? []) if (t && t.temple.x) markers.push({ ...t.temple, label: shortLabel(t.name), full: t.name, color: [1, 0.85, 0.3] });
    }
    computePreview();
    let result;
    try {
      result = renderer.render({
        width,
        height,
        spriteSize: S,
        camX: cam.x,
        camY: cam.y,
        scale,
        floor: cam.floor,
        chunks: store(id),
        geo: geoOf(id),
        items: map.items,
        looks,
        sprites: spriteCache,
        opts: map.view,
        time: performance.now(),
        house: map.brush?.kind === "house" || map.brush?.kind === "houseExit" ? (map.brush.house ?? 0) : 0,
        ghosts,
        ghostAlpha: map.pasting ? 0.65 : 0.55,
        rects,
        markers,
        moveOffset: drag?.kind === "move" && (drag.dx || drag.dy) ? { dx: drag.dx, dy: drag.dy, dz: 0 } : null,
        hover: map.hover,
      });
    } catch (e) {
      glError = errorMessage(e);
      return;
    }
    drawLabels(result.labels);
    clearTimeout(animTimer);
    if (result.more) requestDraw();
    else if (result.pending) animTimer = window.setTimeout(requestDraw, 60);
    else if (result.animated) animTimer = window.setTimeout(requestDraw, 90);
    if (!result.more && !result.pending) rendered(id);
    // Keep memory bounded on long sessions.
    store(id).trim((k) => (k & 15) === cam.floor);
    geoOf(id).trim((k) => Math.abs((k & 15) - cam.floor) < 4);
  }

  function drawLabels(labels: Label[]) {
    const ctx = overlay?.getContext("2d");
    if (!ctx || !overlay) return;
    ctx.clearRect(0, 0, overlay.width, overlay.height);
    if (!labels.length) return;
    // The client's name font: the pixel font at a whole multiple of its 14px
    // size, with a hard 1px black outline drawn as offset copies.
    const s = Math.max(1, Math.round(window.devicePixelRatio || 1));
    ctx.textAlign = "center";
    const big = `bold ${14 * s}px "Tibia Verdana", Verdana, sans-serif`;
    // Labels under the mouse come last: drawn over the others.
    const byTop = (a: Label, b: Label) => Number(!!a.top) - Number(!!b.top);
    const shown = labels
      .filter((l) => !l.small)
      .slice(0, 1000)
      .sort(byTop)
      .map((l) => ({ ...l, x: Math.round(l.x), y: Math.round(l.y) }));
    // Item texts are many and long: one that would cover a name (or an
    // earlier text) is left out.
    const boxes: [number, number, number, number][] = [];
    const pad = 2 * s;
    const free = (b: [number, number, number, number]) => !boxes.some((o) => b[0] < o[2] && b[2] > o[0] && b[1] < o[3] && b[3] > o[1]);
    const smallBox = (l: Label, w: number): [number, number, number, number] => [l.x - w / 2 - pad, l.y - 6 * s, l.x + w / 2 + pad, l.y + 6 * s];
    ctx.font = big;
    for (const l of shown) {
      const w = ctx.measureText(l.text).width;
      boxes.push([l.x - w / 2 - pad, l.y - 15 * s, l.x + w / 2 + pad, l.y]);
    }
    // Small labels (tile coordinates, item texts): the small pixel font, many
    // of them, so one stroke for the outline instead of eight copies.
    const small = labels
      .filter((l) => l.small)
      .sort(byTop)
      .map((l) => ({ ...l, x: Math.round(l.x), y: Math.round(l.y) }));
    if (small.length) {
      ctx.font = `${9 * s}px "Tibia Small", Verdana, sans-serif`;
      for (const l of small) if (l.top) boxes.push(smallBox(l, ctx.measureText(l.text).width));
      ctx.textBaseline = "middle";
      ctx.lineWidth = 2 * s;
      ctx.lineJoin = "round";
      ctx.strokeStyle = "#000";
      for (const l of small) {
        ctx.textAlign = l.align ?? "center";
        if (l.cull) {
          const box = smallBox(l, ctx.measureText(l.text).width);
          if (!free(box)) continue;
          boxes.push(box);
        }
        ctx.strokeText(l.text, l.x, l.y);
        ctx.fillStyle = l.color;
        ctx.fillText(l.text, l.x, l.y);
      }
    }
    ctx.font = big;
    ctx.textAlign = "center";
    ctx.textBaseline = "bottom";
    ctx.fillStyle = "#000";
    for (const l of shown) {
      for (const [dx, dy] of OUTLINE) ctx.fillText(l.text, l.x + dx * s, l.y + dy * s);
    }
    for (const l of shown) {
      ctx.fillStyle = l.color;
      ctx.fillText(l.text, l.x, l.y);
    }
  }
  // "Paupers Palace, Flat 07" -> "Flat 07": neighbors share the building.
  const houseLabel = (name: string) => name.slice(name.lastIndexOf(",") + 1).trim() || name;
  const shortLabel = (s: string) => (s.length > 18 ? s.slice(0, 17).trimEnd() + "…" : s);
  const OUTLINE = [
    [-1, -1], [0, -1], [1, -1],
    [-1, 0], [1, 0],
    [-1, 1], [0, 1], [1, 1],
  ];
  // Labels drawn before the font loaded used the fallback.
  void Promise.all([document.fonts?.load('bold 14px "Tibia Verdana"'), document.fonts?.load('9px "Tibia Small"')]).then(() => requestDraw());

  // ----- coordinates -----

  function devicePoint(e: MouseEvent): { x: number; y: number } {
    const r = canvas!.getBoundingClientRect();
    return { x: ((e.clientX - r.left) / r.width) * width, y: ((e.clientY - r.top) / r.height) * height };
  }

  function tileAt(e: MouseEvent): Position {
    const { x, y } = devicePoint(e);
    const cam = map.cam;
    const dpr = window.devicePixelRatio || 1;
    const scale = cam.zoom * dpr;
    const S = spriteSize();
    const wx = (x - width / 2) / scale + cam.x;
    const wy = (y - height / 2) / scale + cam.y;
    const adj = floorAdjust(cam.floor, cam.floor);
    return pos(Math.max(0, Math.floor(wx / S) + adj), Math.max(0, Math.floor(wy / S) + adj), cam.floor);
  }

  // ----- brush preview -----

  function area(c: Position, size: number, shape: string): Position[] {
    const out: Position[] = [];
    for (let dy = -size; dy <= size; dy++) {
      for (let dx = -size; dx <= size; dx++) {
        if (shape === "circle" && Math.sqrt(dx * dx + dy * dy) >= size + 0.005) continue;
        if (c.x + dx < 0 || c.y + dy < 0) continue;
        out.push(pos(c.x + dx, c.y + dy, c.z));
      }
    }
    return out;
  }

  const AREA_KINDS = new Set(["raw", "ground", "wall", "carpet", "table", "eraser", "house", "flag", "zone", "optionalBorder"]);

  function lookOf(b: BrushSel): number {
    return b.look ?? b.item ?? 0;
  }

  function creatureGhost(b: BrushSel): CreatureData[] {
    const c = map.palette?.creatures?.find((x) => x?.name === b.creature);
    if (!c) return [];
    const o = c.outfit;
    return [{ npc: !!c.npc, selected: false, dir: map.direction, lookType: o.lookType, head: o.head ?? 0, body: o.body ?? 0, legs: o.legs ?? 0, feet: o.feet ?? 0, addons: o.addons ?? 0, mount: o.mount ?? 0, lookItem: o.lookItem ?? 0, name: c.name }];
  }

  function computePreview() {
    ghosts = [];
    rects = [];
    const h = map.hover;
    if (drag?.kind === "box") {
      rects.push({ x0: drag.start.x, y0: drag.start.y, x1: drag.cur.x, y1: drag.cur.y, z: map.floor, fill: [0.4, 0.6, 1, 0.12], stroke: [0.6, 0.8, 1, 0.9] });
    }
    if (drag?.kind === "rect") {
      const b = map.brush;
      rects.push({ x0: drag.start.x, y0: drag.start.y, x1: drag.cur.x, y1: drag.cur.y, z: map.floor, fill: drag.erase ? [1, 0.2, 0.2, 0.25] : [1, 1, 1, 0.12], stroke: [1, 1, 1, 0.7] });
      if (b && !drag.erase && lookOf(b) && AREA_KINDS.has(b.kind)) {
        const [x0, x1] = [Math.min(drag.start.x, drag.cur.x), Math.max(drag.start.x, drag.cur.x)];
        const [y0, y1] = [Math.min(drag.start.y, drag.cur.y), Math.max(drag.start.y, drag.cur.y)];
        if ((x1 - x0 + 1) * (y1 - y0 + 1) <= 4000) {
          for (let y = y0; y <= y1; y++) for (let x = x0; x <= x1; x++) if (b.kind !== "wall" || x === x0 || x === x1 || y === y0 || y === y1) ghosts.push({ x, y, z: map.floor, ids: [lookOf(b)] });
        }
      }
      return;
    }
    if (!h || drag?.kind === "pan") return;
    if (map.pasting && clip) {
      for (const t of clip.tiles ?? []) {
        if (!t) continue;
        const ids: number[] = [];
        const subs: number[] = [];
        if (t.ground) {
          ids.push(t.ground.id);
          subs.push(t.ground.subtype);
        }
        for (const it of t.items ?? []) {
          ids.push(it.id);
          subs.push(it.subtype);
        }
        ghosts.push({ x: h.x + t.dx, y: h.y + t.dy, z: h.z + t.dz, ids, subs });
      }
      rects.push({ x0: h.x, y0: h.y, x1: h.x + clip.width - 1, y1: h.y + clip.height - 1, z: h.z, stroke: [1, 0.85, 0.3, 0.8] });
      return;
    }
    if (map.tool !== "brush" || !map.brush) return;
    const b = map.brush;
    const erase = modifiers.ctrl;
    if (b.kind === "doodad") {
      if (erase) {
        for (const p of area(h, map.size, map.shape)) rects.push({ x0: p.x, y0: p.y, x1: p.x, y1: p.y, z: p.z, fill: [1, 0.2, 0.2, 0.3] });
        return;
      }
      for (const t of doodad?.tiles ?? []) {
        if (t) ghosts.push({ x: h.x + t.x, y: h.y + t.y, z: h.z + t.z, ids: (t.items ?? []).map((i) => i.id) });
      }
      return;
    }
    if (b.kind === "spawn" || b.kind === "spawnNpc") {
      const r = Math.max(1, map.size);
      rects.push({ x0: h.x - r, y0: h.y - r, x1: h.x + r, y1: h.y + r, z: h.z, stroke: erase ? [1, 0.3, 0.3, 0.9] : [1, 0.6, 0.6, 0.9], fill: [1, 0.5, 0.5, 0.08] });
      return;
    }
    if (b.kind === "creature") {
      if (erase) rects.push({ x0: h.x, y0: h.y, x1: h.x, y1: h.y, z: h.z, fill: [1, 0.2, 0.2, 0.35] });
      else ghosts.push({ x: h.x, y: h.y, z: h.z, ids: [], creatures: creatureGhost(b) });
      return;
    }
    if (b.kind === "door" || b.kind === "houseExit" || b.kind === "waypoint") {
      rects.push({ x0: h.x, y0: h.y, x1: h.x, y1: h.y, z: h.z, stroke: [1, 1, 1, 0.9], fill: [1, 1, 1, 0.15] });
      return;
    }
    if (map.fill && b.kind === "ground") {
      rects.push({ x0: h.x, y0: h.y, x1: h.x, y1: h.y, z: h.z, stroke: [1, 0.85, 0.3, 1], fill: [1, 0.85, 0.3, 0.25] });
      return;
    }
    const tiles = b.kind === "wall" && map.size > 0 ? area(h, map.size, "square").filter((p) => Math.abs(p.x - h.x) === map.size || Math.abs(p.y - h.y) === map.size) : area(h, map.size, map.shape);
    const look = lookOf(b);
    for (const p of tiles) {
      if (erase || b.kind === "eraser") rects.push({ x0: p.x, y0: p.y, x1: p.x, y1: p.y, z: p.z, fill: [1, 0.2, 0.2, 0.3] });
      else if (b.kind === "house" || b.kind === "flag" || b.kind === "zone" || b.kind === "optionalBorder") rects.push({ x0: p.x, y0: p.y, x1: p.x, y1: p.y, z: p.z, fill: b.kind === "house" ? [0.4, 0.6, 1, 0.3] : [0.3, 1, 0.4, 0.25] });
      else if (look) ghosts.push({ x: p.x, y: p.y, z: p.z, ids: [look] });
    }
    if (tiles.length > 1) {
      const xs = tiles.map((p) => p.x);
      const ys = tiles.map((p) => p.y);
      rects.push({ x0: Math.min(...xs), y0: Math.min(...ys), x1: Math.max(...xs), y1: Math.max(...ys), z: h.z, stroke: [1, 1, 1, 0.35] });
    }
  }

  async function refreshDoodad() {
    const b = map.brush;
    const id = map.activeId;
    if (!b || b.kind !== "doodad" || id === null || !b.name) {
      doodad = null;
      return;
    }
    try {
      doodad = await MapService.DoodadPreview(id, b.name, map.variation, map.size, map.shape, map.thickness);
    } catch {
      doodad = null;
    }
    requestDraw();
  }

  $effect(() => {
    void [map.brush, map.size, map.shape, map.variation, map.thickness, map.activeId, map.reroll];
    void refreshDoodad();
  });

  $effect(() => {
    if (map.pasting) {
      void MapService.Clipboard().then((c) => {
        clip = c;
        requestDraw();
      });
    } else clip = null;
  });

  // ----- input -----

  const modifiers = { ctrl: false, shift: false, alt: false };
  let spaceHeld = $state(false);

  type Drag =
    | { kind: "pan"; x: number; y: number; camX: number; camY: number; button: number; moved: boolean }
    | { kind: "paint"; last: Position; stroke: number; pending: Position[]; erase: boolean; alt: boolean; inflight: boolean }
    | { kind: "rect"; start: Position; cur: Position; erase: boolean; alt: boolean }
    | { kind: "box"; start: Position; cur: Position; add: boolean; remove: boolean; px: { x: number; y: number }; started: boolean }
    | { kind: "move"; start: Position; dx: number; dy: number }
    | null;
  let drag: Drag = null;

  const SMEAR = new Set(["raw", "ground", "wall", "carpet", "table", "eraser", "house", "flag", "zone", "optionalBorder", "creature"]);
  const DRAGGABLE = new Set(["ground", "wall", "carpet", "table", "eraser", "house", "flag", "zone", "raw", "optionalBorder"]);

  function request(b: BrushSel, extra: Partial<DrawRequest>): DrawRequest {
    return {
      kind: b.kind,
      name: b.name ?? "",
      item: b.item ?? 0,
      house: b.house ?? 0,
      waypoint: b.waypoint ?? "",
      creature: b.creature ?? "",
      npc: !!b.npc,
      spawnTime: map.spawnTime,
      direction: map.direction,
      flag: b.flag ?? 0,
      zone: b.zone ?? 0,
      doorType: b.doorType ?? "",
      points: [],
      size: map.size,
      shape: map.shape,
      erase: false,
      alt: false,
      fill: false,
      rect: null,
      variation: map.variation,
      thickness: map.thickness,
      stroke: 0,
      ...extra,
    };
  }

  async function send(r: DrawRequest) {
    const ch = await edit((id) => MapService.Draw(id, r));
    if (ch && map.brush?.kind === "doodad" && !r.erase) void refreshDoodad();
    return ch;
  }

  /** Tiles on the line between two tiles (strokes never skip tiles). */
  function line(a: Position, b: Position): Position[] {
    const out: Position[] = [];
    let x = a.x;
    let y = a.y;
    const dx = Math.abs(b.x - a.x);
    const dy = -Math.abs(b.y - a.y);
    const sx = a.x < b.x ? 1 : -1;
    const sy = a.y < b.y ? 1 : -1;
    let err = dx + dy;
    for (;;) {
      if (x !== a.x || y !== a.y) out.push(pos(x, y, a.z));
      if (x === b.x && y === b.y) break;
      const e2 = 2 * err;
      if (e2 >= dy) {
        err += dy;
        x += sx;
      }
      if (e2 <= dx) {
        err += dx;
        y += sy;
      }
    }
    return out;
  }

  async function flushPaint() {
    if (drag?.kind !== "paint" || drag.inflight || !drag.pending.length || !map.brush) return;
    const d = drag;
    const points = d.pending.splice(0);
    d.inflight = true;
    await send(request(map.brush, { points, erase: d.erase, alt: d.alt, stroke: d.stroke }));
    d.inflight = false;
    if (d.pending.length) void flushPaint();
  }

  let strokeSeq = Date.now();

  function onpointerdown(e: PointerEvent) {
    if (map.activeId === null) return;
    canvas!.setPointerCapture(e.pointerId);
    modifiers.ctrl = e.ctrlKey || e.metaKey;
    modifiers.shift = e.shiftKey;
    modifiers.alt = e.altKey;
    const p = tileAt(e);
    if (e.button === 1 || e.button === 2 || (e.button === 0 && spaceHeld)) {
      e.preventDefault();
      drag = { kind: "pan", x: e.clientX, y: e.clientY, camX: map.cam.x, camY: map.cam.y, button: e.button, moved: false };
      return;
    }
    if (e.button !== 0) return;
    if (map.pick) {
      const pk = map.pick;
      map.pick = null;
      pk.done(p);
      return;
    }
    if (map.pasting) {
      void edit((id) => MapService.Paste(id, p)).then(() => {
        if (!e.shiftKey) map.pasting = false;
      });
      return;
    }
    if (modifiers.ctrl && modifiers.alt) {
      pickBrush(p);
      return;
    }
    if (map.tool === "select") {
      const t = store(map.activeId).peekTile(p.x, p.y, p.z);
      if (modifiers.shift) {
        drag = { kind: "box", start: p, cur: p, add: modifiers.ctrl, remove: modifiers.alt, px: { x: e.clientX, y: e.clientY }, started: true };
      } else if (modifiers.ctrl) {
        void edit((id) => MapService.SelectAt(id, p, true, map.view.creatures, map.view.spawns));
      } else if (t && t.flags & TF.selected) {
        drag = { kind: "move", start: p, dx: 0, dy: 0 };
      } else {
        drag = { kind: "box", start: p, cur: p, add: false, remove: false, px: { x: e.clientX, y: e.clientY }, started: false };
      }
      mapCommands.inspect(p);
      requestDraw();
      return;
    }
    const b = map.brush;
    if (!b) {
      map.side = "library";
      toast("Pick something to draw with in the library first.");
      return;
    }
    if (map.fill && b.kind === "ground") {
      void send(request(b, { points: [p], fill: true, erase: false }));
      return;
    }
    if (modifiers.shift && DRAGGABLE.has(b.kind)) {
      drag = { kind: "rect", start: p, cur: p, erase: modifiers.ctrl, alt: modifiers.alt };
      requestDraw();
      return;
    }
    const smear = SMEAR.has(b.kind) || (b.kind === "doodad" && !!map.palette && isDraggableDoodad(b));
    if (smear) {
      drag = { kind: "paint", last: p, stroke: ++strokeSeq, pending: [p], erase: modifiers.ctrl, alt: modifiers.alt, inflight: false };
      void flushPaint();
    } else {
      void send(request(b, { points: [p], erase: modifiers.ctrl, alt: modifiers.alt, stroke: ++strokeSeq }));
    }
  }

  function isDraggableDoodad(_b: BrushSel): boolean {
    return true;
  }

  function onpointermove(e: PointerEvent) {
    modifiers.ctrl = e.ctrlKey || e.metaKey;
    modifiers.shift = e.shiftKey;
    modifiers.alt = e.altKey;
    if (map.activeId === null) return;
    if (drag?.kind === "pan") {
      const dpr = window.devicePixelRatio || 1;
      const r = canvas!.getBoundingClientRect();
      const k = width / r.width / (map.cam.zoom * dpr);
      const dx = (e.clientX - drag.x) * k;
      const dy = (e.clientY - drag.y) * k;
      if (Math.abs(e.clientX - drag.x) + Math.abs(e.clientY - drag.y) > 3) drag.moved = true;
      setCam(map.activeId, { x: drag.camX - dx, y: drag.camY - dy });
      requestDraw();
      return;
    }
    const p = tileAt(e);
    const changed = !map.hover || map.hover.x !== p.x || map.hover.y !== p.y || map.hover.z !== p.z;
    if (changed) {
      map.hover = p;
      updateHoverText(p);
    }
    if (drag?.kind === "paint" && changed) {
      drag.pending.push(...line(drag.last, p));
      drag.last = p;
      void flushPaint();
    } else if ((drag?.kind === "rect" || drag?.kind === "box") && changed) {
      drag.cur = p;
      if (drag.kind === "box" && !drag.started && Math.abs(e.clientX - drag.px.x) + Math.abs(e.clientY - drag.px.y) > 4) drag.started = true;
    } else if (drag?.kind === "move" && changed) {
      drag.dx = p.x - drag.start.x;
      drag.dy = p.y - drag.start.y;
    }
    if (changed || drag) requestDraw();
  }

  function onpointerup(e: PointerEvent) {
    const d = drag;
    drag = null;
    if (map.activeId === null || !d) return;
    const p = tileAt(e);
    if (d.kind === "pan") {
      if (!d.moved && d.button === 2) {
        // A right click drops the brush (or paste, or pick) back to the
        // selection; the menu opens only in selection mode.
        if (map.pick) map.pick = null;
        else if (map.pasting) map.pasting = false;
        else if (map.tool === "brush") map.tool = "select";
        else showMenu(e, p);
      }
    } else if (d.kind === "rect" && map.brush) {
      void send(request(map.brush, { rect: { from: d.start, to: d.cur }, erase: d.erase, alt: d.alt, stroke: ++strokeSeq }));
    } else if (d.kind === "box") {
      if (d.started) {
        void edit((id) => MapService.SelectRect(id, { from: d.start, to: d.cur }, selectionFloors(), d.add, d.remove, true));
      } else {
        void edit((id) => MapService.SelectAt(id, p, false, map.view.creatures, map.view.spawns));
      }
    } else if (d.kind === "move") {
      if (d.dx || d.dy) void edit((id) => MapService.MoveSelection(id, d.dx, d.dy, 0));
    }
    requestDraw();
  }

  function selectionFloors(): string {
    return map.view.lowerFloors && modifiers.alt ? "lower" : "current";
  }

  function onwheel(e: WheelEvent) {
    e.preventDefault();
    if (map.activeId === null) return;
    const dir = e.deltaY < 0 ? 1 : -1;
    if (e.ctrlKey || e.metaKey) {
      mapCommands.floor(map.floor + dir);
    } else if (e.altKey) {
      map.size = Math.min(15, Math.max(0, map.size + dir));
    } else if (e.shiftKey) {
      mapCommands.pan(-dir * 3, 0);
    } else {
      const { x, y } = devicePoint(e);
      mapCommands.zoom(dir, { x, y, w: width, h: height });
    }
    requestDraw();
  }

  function onkey(e: KeyboardEvent) {
    const was = { ...modifiers };
    modifiers.ctrl = e.ctrlKey || e.metaKey;
    modifiers.shift = e.shiftKey;
    modifiers.alt = e.altKey;
    if (e.code === "Space" && !(e.target as HTMLElement).closest("input, textarea")) {
      spaceHeld = e.type === "keydown";
      if (map.mode === "map" && map.activeId !== null) e.preventDefault();
    }
    if (was.ctrl !== modifiers.ctrl || was.shift !== modifiers.shift) requestDraw();
  }

  function updateHoverText(p: Position) {
    const id = map.activeId;
    const items = map.items;
    if (id === null || !items) return;
    const t = store(id).peekTile(p.x, p.y, p.z);
    let text = fmtPos(p);
    if (t) {
      const top = t.ids.length ? t.ids[t.ids.length - 1] : 0;
      if (top) text += ` · ${items.name(top)} (${top})`;
      if (t.creatures?.length) text += ` · ${t.creatures.map((c) => c.name).join(", ")}`;
      if (t.house) {
        const h = map.active?.houses?.find((x) => x.id === t.house);
        text += ` · house ${h?.name ?? t.house}`;
      }
      if (t.flags & TF.pz) text += " · PZ";    }
    hoverText = text;
  }

  function pickBrush(p: Position) {
    const id = map.activeId;
    const items = map.items;
    if (id === null || !items) return;
    const t = store(id).peekTile(p.x, p.y, p.z);
    if (!t || !t.ids.length) return;
    const top = t.ids[t.ids.length - 1];
    selectBrush({ kind: "raw", item: top, look: top, label: items.name(top) });
    toast(`Brush: ${items.name(top)}`);
  }

  /** Double click: an item with text opens it, a creature becomes the
   * brush, anything else is inspected. */
  function ondblclick(e: MouseEvent) {
    const id = map.activeId;
    if (map.tool !== "select" || id === null) return;
    const p = tileAt(e);
    const t = store(id).peekTile(p.x, p.y, p.z);
    const readable = readableIndex(t);
    if (readable >= 0) return openText(p, readable);
    const c = map.view.creatures ? t?.creatures?.at(-1) : undefined;
    if (c) {
      useCreature(c);
      toast(`Brush: ${c.name}`);
    } else mapCommands.inspect(p);
  }

  function showMenu(e: MouseEvent, p: Position) {
    const id = map.activeId;
    if (id === null) return;
    const t = store(id).peekTile(p.x, p.y, p.z);
    const entries: MenuEntry[] = mapContextMenu(p, t ?? null, { pick: () => pickBrush(p) });
    openContextMenu(e, entries);
  }

  // ----- lifecycle -----

  onMount(() => {
    try {
      renderer = new MapRenderer(canvas!);
    } catch (e) {
      glError = errorMessage(e);
    }
    const ro = new ResizeObserver((entries) => {
      const en = entries[0];
      const box = en.devicePixelContentBoxSize?.[0];
      const dpr = window.devicePixelRatio || 1;
      const w = Math.max(1, Math.round(box ? box.inlineSize : en.contentRect.width * dpr));
      const h = Math.max(1, Math.round(box ? box.blockSize : en.contentRect.height * dpr));
      if (w === width && h === height) return;
      width = w;
      height = h;
      // Resizing clears the canvases; draw now (observers run before the
      // browser paints) instead of on the next frame, or the map blinks.
      canvas!.width = width;
      canvas!.height = height;
      overlay!.width = width;
      overlay!.height = height;
      cancelAnimationFrame(raf);
      draw();
    });
    ro.observe(host!);
    const offChange = onMapChange((id, ch) => {
      const s = stores.get(id);
      s?.invalidate(ch.all ? null : (ch.chunks ?? []), ch.info?.rev ?? 0);
      geos.get(id)?.invalidate(ch.all ? null : (ch.chunks ?? []));
      if (id === map.activeId) requestDraw();
    });
    const offSprites = spriteCache.onInvalidate((ids) => {
      renderer?.invalidateSprites(ids);
      requestDraw();
    });
    return () => {
      ro.disconnect();
      offChange();
      offSprites();
      cancelAnimationFrame(raf);
      clearTimeout(animTimer);
      renderer?.dispose();
      renderer = null;
    };
  });

  // Redraw on any state the frame depends on.
  $effect(() => {
    void [map.activeId, map.cam.x, map.cam.y, map.cam.zoom, map.cam.floor, map.dataRev, map.tool, map.brush, map.size, map.shape, map.fill, map.pasting, map.active?.houses, map.active?.waypoints, map.active?.towns];
    void JSON.stringify(map.view);
    requestDraw();
  });

  // Object edits change items and sprites: reload their appearances.
  $effect(() => {
    void app.rev;
    void map.items;
    looks.clear();
    for (const g of geos.values()) g.invalidate(null);
    requestDraw();
  });

  // Maps that were closed free their caches.
  $effect(() => {
    const ids = new Set(map.maps.map((m) => m.id));
    for (const id of [...stores.keys()]) {
      if (!ids.has(id)) {
        stores.delete(id);
        geos.delete(id);
      }
    }
  });
</script>

<svelte:window onkeydown={onkey} onkeyup={onkey} onblur={() => ((spaceHeld = false), (drag = null))} />

<div class="host" bind:this={host} class:pan={spaceHeld} class:brush={map.tool === "brush" || !!map.pick}>
  <canvas
    class="map-gl"
    bind:this={canvas}
    onpointerdown={onpointerdown}
    onpointermove={onpointermove}
    onpointerup={onpointerup}
    onpointerleave={() => ((map.hover = null), (hoverText = ""), requestDraw())}
    onwheel={onwheel}
    oncontextmenu={(e) => e.preventDefault()}
    ondblclick={ondblclick}
  ></canvas>
  <canvas class="labels" bind:this={overlay}></canvas>
  <FloorLadder />
  <ZoomControl />
  <SelectionBar />
  {#if map.pick}
    <div class="pickhint">{map.pick.label} · Esc cancels</div>
  {:else if map.pasting}
    <div class="pickhint">Click to paste · Shift+click pastes again · Esc cancels</div>
  {/if}
  <div class="fps t-label">{fps} FPS{loadInfo ? ` · ${loadInfo}` : ""}</div>
  {#if glError}
    <div class="err t-label">The map cannot be drawn: {glError}</div>
  {/if}
</div>

<style>
  .host {
    position: relative;
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    background: #000;
    cursor: default;
  }
  .host.brush {
    cursor: crosshair;
  }
  .host.pan {
    cursor: grab;
  }
  canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    display: block;
  }
  .labels {
    pointer-events: none;
  }
  .pickhint {
    position: absolute;
    top: 6px;
    left: 50%;
    transform: translateX(-50%);
    padding: 3px 10px;
    background: rgba(0, 0, 0, 0.75);
    border: 1px solid var(--gold);
    color: var(--gold);
    pointer-events: none;
  }
  .fps {
    position: absolute;
    left: 6px;
    top: 6px;
    padding: 1px 5px;
    background: rgba(0, 0, 0, 0.6);
    pointer-events: none;
    font-variant-numeric: tabular-nums;
  }
  .err {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    padding: 20px;
    text-align: center;
  }
</style>
