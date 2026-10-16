<script lang="ts">
  import { onMount } from "svelte";
  import { errorMessage } from "../../lib/api";
  import { openContextMenu, type MenuEntry } from "../../lib/menu.svelte";
  import { spriteCache } from "../../lib/render/sprites";
  import { app, toast } from "../../lib/state/app.svelte";
  import { MapService, type Clip, type DoodadBuffer, type DrawRequest, type Position } from "../../lib/map/data/api";
  import { Appearances } from "../../lib/map/data/appearances";
  import { ChunkStore, TileFlags } from "../../lib/map/data/chunks";
  import { GeoStore } from "../../lib/map/render/geo";
  import { rendered } from "../../lib/map/render/rendered";
  import { mapCommands, spriteSize } from "../../lib/map/view/commands";
  import { DRAG_THRESHOLD, FPS_POLL_MS, GHOST_ALPHA, MAX_BRUSH_SIZE, RECT_BRUSH_KINDS, REDRAW_DELAY, SMEAR_BRUSH_KINDS, WHEEL_PAN_TILES } from "../../lib/map/constants";
  import { computePreview, type MapDrag } from "../../lib/map/editor/ghostPreview";
  import { describeTile } from "../../lib/map/editor/hoverText";
  import { drawLabels, loadLabelFonts } from "../../lib/map/render/labels";
  import { markersOf } from "../../lib/map/render/markers";
  import { MapRenderer } from "../../lib/map/render/renderer";
  import { bresenhamLine, screenToTile } from "../../lib/map/editor/tileGeometry";
  import { clamp } from "../../lib/util/math";
  import { applyChange, edit, map, onMapChange, selectBrush, setCam, useCreature, type BrushSel } from "../../lib/map/state.svelte";
  import { mapContextMenu, openText, readableIndex } from "../../lib/map/view/menus";
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
    let chunkStore = stores.get(id);
    if (!chunkStore) {
      chunkStore = new ChunkStore(id, () => requestDraw());
      stores.set(id, chunkStore);
    }
    return chunkStore;
  }

  function geoOf(id: number): GeoStore {
    let geoStore = geos.get(id);
    if (!geoStore) {
      geoStore = new GeoStore(id, () => requestDraw());
      geos.set(id, geoStore);
    }
    return geoStore;
  }

  // ----- drawing -----

  let raf = 0;
  let animTimer = 0;
  function requestDraw() {
    if (raf) return;
    raf = requestAnimationFrame(draw);
  }

  let doodad: DoodadBuffer | null = null;
  let clip: Clip | null = null;

  // Frames drawn in the last second. The map draws on demand, so it reads 0 when nothing changes.
  let frameTimes: number[] = [];
  let fps = $state(0);
  /** Chunk geometry loading in the last second (shown next to the FPS). */
  let loadInfo = $state("");
  $effect(() => {
    const timer = setInterval(() => {
      const now = performance.now();
      frameTimes = frameTimes.filter((time) => now - time < 1000);
      fps = frameTimes.length;
      const id = map.activeId;
      const stats = id !== null ? geos.get(id)?.stats() : undefined;
      loadInfo = stats?.count ? `${stats.count} req/s · ${Math.round(stats.total)} ms (Go ${stats.build.toFixed(1)} ms)` : "";
    }, FPS_POLL_MS);
    return () => clearInterval(timer);
  });

  function draw() {
    raf = 0;
    frameTimes.push(performance.now());
    const id = map.activeId;
    if (!renderer || id === null || !map.items || !width) return;
    const cam = map.cam;
    const tileSize = spriteSize();
    const dpr = window.devicePixelRatio || 1;
    const scale = cam.zoom * dpr;
    const markers = markersOf(map.active);
    const { ghosts, rects } = computePreview({
      drag,
      hover: map.hover,
      floor: map.floor,
      tool: map.tool,
      brush: map.brush,
      erase: modifiers.ctrl,
      pasting: map.pasting,
      clip,
      doodad,
      size: map.size,
      shape: map.shape,
      fill: map.fill,
      creatures: map.palette?.creatures,
      direction: map.direction,
    });
    let result;
    try {
      result = renderer.render({
        width,
        height,
        spriteSize: tileSize,
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
        ghostAlpha: map.pasting ? GHOST_ALPHA.paste : GHOST_ALPHA.brush,
        rects,
        markers,
        moveOffset: drag?.kind === "move" && (drag.dx || drag.dy) ? { dx: drag.dx, dy: drag.dy, dz: 0 } : null,
        hover: map.hover,
      });
    } catch (err) {
      glError = errorMessage(err);
      return;
    }
    const ctx = overlay?.getContext("2d");
    if (ctx && overlay) drawLabels(ctx, overlay.width, overlay.height, result.labels, window.devicePixelRatio);
    clearTimeout(animTimer);
    if (result.more) requestDraw();
    else if (result.pending) animTimer = window.setTimeout(requestDraw, REDRAW_DELAY.pending);
    else if (result.animated) animTimer = window.setTimeout(requestDraw, REDRAW_DELAY.animated);
    if (!result.more && !result.pending) rendered(id);
    // Keep memory bounded on long sessions.
    store(id).trim((key) => (key & 15) === cam.floor);
    geoOf(id).trim((key) => Math.abs((key & 15) - cam.floor) < 4);
  }

  void loadLabelFonts().then(() => requestDraw());


  // ----- coordinates -----

  function devicePoint(event: MouseEvent): { x: number; y: number } {
    const bounds = canvas!.getBoundingClientRect();
    return { x: ((event.clientX - bounds.left) / bounds.width) * width, y: ((event.clientY - bounds.top) / bounds.height) * height };
  }

  function tileAt(event: MouseEvent): Position {
    const { x, y } = devicePoint(event);
    const cam = map.cam;
    const dpr = window.devicePixelRatio || 1;
    return screenToTile(x, y, { width, height, camX: cam.x, camY: cam.y, scale: cam.zoom * dpr, floor: cam.floor, tileSize: spriteSize() });
  }

  // ----- brush preview -----

  async function refreshDoodad() {
    const brush = map.brush;
    const id = map.activeId;
    if (!brush || brush.kind !== "doodad" || id === null || !brush.name) {
      doodad = null;
      return;
    }
    try {
      doodad = await MapService.DoodadPreview(id, brush.name, map.variation, map.size, map.shape, map.thickness);
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
      void MapService.Clipboard().then((clipboard) => {
        clip = clipboard;
        requestDraw();
      });
    } else clip = null;
  });

  // ----- input -----

  const modifiers = { ctrl: false, shift: false, alt: false };
  let spaceHeld = $state(false);

  let drag: MapDrag = null;

  function request(brush: BrushSel, extra: Partial<DrawRequest>): DrawRequest {
    return {
      kind: brush.kind,
      name: brush.name ?? "",
      item: brush.item ?? 0,
      house: brush.house ?? 0,
      waypoint: brush.waypoint ?? "",
      creature: brush.creature ?? "",
      npc: !!brush.npc,
      spawnTime: map.spawnTime,
      direction: map.direction,
      flag: brush.flag ?? 0,
      zone: brush.zone ?? 0,
      doorType: brush.doorType ?? "",
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

  async function send(drawRequest: DrawRequest) {
    const change = await edit((id) => MapService.Draw(id, drawRequest));
    if (change && map.brush?.kind === "doodad" && !drawRequest.erase) void refreshDoodad();
    return change;
  }

  async function flushPaint() {
    if (drag?.kind !== "paint" || drag.inflight || !drag.pending.length || !map.brush) return;
    const paint = drag;
    const points = paint.pending.splice(0);
    paint.inflight = true;
    await send(request(map.brush, { points, erase: paint.erase, alt: paint.alt, stroke: paint.stroke }));
    paint.inflight = false;
    if (paint.pending.length) void flushPaint();
  }

  let strokeSeq = Date.now();

  function onpointerdown(event: PointerEvent) {
    if (map.activeId === null) return;
    canvas!.setPointerCapture(event.pointerId);
    readModifiers(event);
    const tile = tileAt(event);
    if (event.button === 1 || event.button === 2 || (event.button === 0 && spaceHeld)) {
      event.preventDefault();
      drag = { kind: "pan", x: event.clientX, y: event.clientY, camX: map.cam.x, camY: map.cam.y, button: event.button, moved: false };
      return;
    }
    if (event.button !== 0) return;
    if (map.pick) {
      const pick = map.pick;
      map.pick = null;
      pick.done(tile);
      return;
    }
    if (map.pasting) {
      void edit((id) => MapService.Paste(id, tile)).then(() => {
        if (!event.shiftKey) map.pasting = false;
      });
      return;
    }
    if (modifiers.ctrl && modifiers.alt) {
      pickBrush(tile);
      return;
    }
    if (map.tool === "select") {
      const tileData = store(map.activeId).peekTile(tile.x, tile.y, tile.z);
      const pressedAt = { x: event.clientX, y: event.clientY };
      if (modifiers.shift) {
        drag = { kind: "box", start: tile, cur: tile, add: modifiers.ctrl, remove: modifiers.alt, px: pressedAt, started: true };
      } else if (modifiers.ctrl) {
        void edit((id) => MapService.SelectAt(id, tile, true, map.view.creatures, map.view.spawns));
      } else if (tileData && tileData.flags & TileFlags.selected) {
        drag = { kind: "move", start: tile, dx: 0, dy: 0 };
      } else {
        drag = { kind: "box", start: tile, cur: tile, add: false, remove: false, px: pressedAt, started: false };
      }
      mapCommands.inspect(tile);
      requestDraw();
      return;
    }
    const brush = map.brush;
    if (!brush) {
      map.side = "library";
      toast("Pick something to draw with in the library first.");
      return;
    }
    if (map.fill && brush.kind === "ground") {
      void send(request(brush, { points: [tile], fill: true, erase: false }));
      return;
    }
    if (modifiers.shift && RECT_BRUSH_KINDS.has(brush.kind)) {
      drag = { kind: "rect", start: tile, cur: tile, erase: modifiers.ctrl, alt: modifiers.alt };
      requestDraw();
      return;
    }
    const smear = SMEAR_BRUSH_KINDS.has(brush.kind) || (brush.kind === "doodad" && !!map.palette);
    if (smear) {
      drag = { kind: "paint", last: tile, stroke: ++strokeSeq, pending: [tile], erase: modifiers.ctrl, alt: modifiers.alt, inflight: false };
      void flushPaint();
    } else {
      void send(request(brush, { points: [tile], erase: modifiers.ctrl, alt: modifiers.alt, stroke: ++strokeSeq }));
    }
  }

  function readModifiers(event: MouseEvent | KeyboardEvent) {
    modifiers.ctrl = event.ctrlKey || event.metaKey;
    modifiers.shift = event.shiftKey;
    modifiers.alt = event.altKey;
  }

  function onpointermove(event: PointerEvent) {
    readModifiers(event);
    if (map.activeId === null) return;
    if (drag?.kind === "pan") {
      const dpr = window.devicePixelRatio || 1;
      const bounds = canvas!.getBoundingClientRect();
      const worldPerCssPixel = width / bounds.width / (map.cam.zoom * dpr);
      const dx = (event.clientX - drag.x) * worldPerCssPixel;
      const dy = (event.clientY - drag.y) * worldPerCssPixel;
      if (Math.abs(event.clientX - drag.x) + Math.abs(event.clientY - drag.y) > DRAG_THRESHOLD.pan) drag.moved = true;
      setCam(map.activeId, { x: drag.camX - dx, y: drag.camY - dy });
      requestDraw();
      return;
    }
    const tile = tileAt(event);
    const changed = !map.hover || map.hover.x !== tile.x || map.hover.y !== tile.y || map.hover.z !== tile.z;
    if (changed) {
      map.hover = tile;
      updateHoverText(tile);
    }
    if (drag?.kind === "paint" && changed) {
      drag.pending.push(...bresenhamLine(drag.last, tile));
      drag.last = tile;
      void flushPaint();
    } else if ((drag?.kind === "rect" || drag?.kind === "box") && changed) {
      drag.cur = tile;
      const travel = drag.kind === "box" ? Math.abs(event.clientX - drag.px.x) + Math.abs(event.clientY - drag.px.y) : 0;
      if (drag.kind === "box" && !drag.started && travel > DRAG_THRESHOLD.selectionBox) drag.started = true;
    } else if (drag?.kind === "move" && changed) {
      drag.dx = tile.x - drag.start.x;
      drag.dy = tile.y - drag.start.y;
    }
    if (changed || drag) requestDraw();
  }

  function onpointerup(event: PointerEvent) {
    const ended = drag;
    drag = null;
    if (map.activeId === null || !ended) return;
    const tile = tileAt(event);
    if (ended.kind === "pan") {
      if (!ended.moved && ended.button === 2) {
        // A right click drops the brush (or paste, or pick) back to the
        // selection; the menu opens only in selection mode.
        if (map.pick) map.pick = null;
        else if (map.pasting) map.pasting = false;
        else if (map.tool === "brush") map.tool = "select";
        else showMenu(event, tile);
      }
    } else if (ended.kind === "rect" && map.brush) {
      void send(request(map.brush, { rect: { from: ended.start, to: ended.cur }, erase: ended.erase, alt: ended.alt, stroke: ++strokeSeq }));
    } else if (ended.kind === "box") {
      if (ended.started) {
        void edit((id) => MapService.SelectRect(id, { from: ended.start, to: ended.cur }, selectionFloors(), ended.add, ended.remove, true));
      } else {
        void edit((id) => MapService.SelectAt(id, tile, false, map.view.creatures, map.view.spawns));
      }
    } else if (ended.kind === "move") {
      if (ended.dx || ended.dy) void edit((id) => MapService.MoveSelection(id, ended.dx, ended.dy, 0));
    }
    requestDraw();
  }

  function selectionFloors(): string {
    return map.view.lowerFloors && modifiers.alt ? "lower" : "current";
  }

  function onwheel(event: WheelEvent) {
    event.preventDefault();
    if (map.activeId === null) return;
    const dir = event.deltaY < 0 ? 1 : -1;
    if (event.ctrlKey || event.metaKey) {
      mapCommands.floor(map.floor + dir);
    } else if (event.altKey) {
      map.size = clamp(map.size + dir, 0, MAX_BRUSH_SIZE);
    } else if (event.shiftKey) {
      mapCommands.pan(-dir * WHEEL_PAN_TILES, 0);
    } else {
      const { x, y } = devicePoint(event);
      mapCommands.zoom(dir, { x, y, w: width, h: height });
    }
    requestDraw();
  }

  function onkey(event: KeyboardEvent) {
    const was = { ...modifiers };
    readModifiers(event);
    if (event.code === "Space" && !(event.target as HTMLElement).closest("input, textarea")) {
      spaceHeld = event.type === "keydown";
      if (map.mode === "map" && map.activeId !== null) event.preventDefault();
    }
    if (was.ctrl !== modifiers.ctrl || was.shift !== modifiers.shift) requestDraw();
  }

  function updateHoverText(tile: Position) {
    const id = map.activeId;
    const items = map.items;
    if (id === null || !items) return;
    hoverText = describeTile(tile, store(id).peekTile(tile.x, tile.y, tile.z), (itemId) => items.name(itemId), map.active?.houses);
  }

  function pickBrush(tile: Position) {
    const id = map.activeId;
    const items = map.items;
    if (id === null || !items) return;
    const tileData = store(id).peekTile(tile.x, tile.y, tile.z);
    if (!tileData || !tileData.ids.length) return;
    const top = tileData.ids[tileData.ids.length - 1];
    selectBrush({ kind: "raw", item: top, look: top, label: items.name(top) });
    toast(`Brush: ${items.name(top)}`);
  }

  /** Double click: an item with text opens it, a creature becomes the
   * brush, anything else is inspected. */
  function ondblclick(event: MouseEvent) {
    const id = map.activeId;
    if (map.tool !== "select" || id === null) return;
    const tile = tileAt(event);
    const tileData = store(id).peekTile(tile.x, tile.y, tile.z);
    const readable = readableIndex(tileData);
    if (readable >= 0) return openText(tile, readable);
    const creature = map.view.creatures ? tileData?.creatures?.at(-1) : undefined;
    if (creature) {
      useCreature(creature);
      toast(`Brush: ${creature.name}`);
    } else mapCommands.inspect(tile);
  }

  function showMenu(event: MouseEvent, tile: Position) {
    const id = map.activeId;
    if (id === null) return;
    const tileData = store(id).peekTile(tile.x, tile.y, tile.z);
    const entries: MenuEntry[] = mapContextMenu(tile, tileData ?? null, { pick: () => pickBrush(tile) });
    openContextMenu(event, entries);
  }

  // ----- lifecycle -----

  onMount(() => {
    try {
      renderer = new MapRenderer(canvas!);
    } catch (err) {
      glError = errorMessage(err);
    }
    const resizeObserver = new ResizeObserver((entries) => {
      const entry = entries[0];
      const box = entry.devicePixelContentBoxSize?.[0];
      const dpr = window.devicePixelRatio || 1;
      const newWidth = Math.max(1, Math.round(box ? box.inlineSize : entry.contentRect.width * dpr));
      const newHeight = Math.max(1, Math.round(box ? box.blockSize : entry.contentRect.height * dpr));
      if (newWidth === width && newHeight === height) return;
      width = newWidth;
      height = newHeight;
      // Resizing clears the canvases; draw now (observers run before the
      // browser paints) instead of on the next frame, or the map blinks.
      canvas!.width = width;
      canvas!.height = height;
      overlay!.width = width;
      overlay!.height = height;
      cancelAnimationFrame(raf);
      draw();
    });
    resizeObserver.observe(host!);
    const offChange = onMapChange((id, change) => {
      stores.get(id)?.invalidate(change.all ? null : (change.chunks ?? []), change.info?.rev ?? 0);
      geos.get(id)?.invalidate(change.all ? null : (change.chunks ?? []));
      if (id === map.activeId) requestDraw();
    });
    const offSprites = spriteCache.onInvalidate((ids) => {
      renderer?.invalidateSprites(ids);
      requestDraw();
    });
    return () => {
      resizeObserver.disconnect();
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
    for (const geoStore of geos.values()) geoStore.invalidate(null);
    requestDraw();
  });

  // Maps that were closed free their caches.
  $effect(() => {
    const ids = new Set(map.maps.map((open) => open.id));
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
    oncontextmenu={(event) => event.preventDefault()}
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
