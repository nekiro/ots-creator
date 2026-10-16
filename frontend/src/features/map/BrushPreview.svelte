<script lang="ts">
  // The whole group of tiles a stamp brush places, drawn from the item
  // textures into one canvas: each item sits at its tile with its
  // bottom-right corner on the tile's, like the client draws bigger objects.
  // One canvas has no seams between tiles; it is shown at most `max` px a
  // tile, smaller when the panel is narrower.
  import { Category, res } from "../../lib/api";
  import type { BrushPreview } from "../../lib/map/data/api";
  import { map } from "../../lib/map/state.svelte";
  import { versions } from "../../lib/state/app.svelte";
  import { clamp } from "../../lib/util/math";

  let {
    preview,
    max = 16,
    selected = [],
    onselect,
  }: {
    preview: BrushPreview;
    max?: number;
    /** Tiles outlined as chosen (preview coordinates). */
    selected?: { x: number; y: number }[];
    /** Makes tiles selectable: a click picks one, a drag a box; add keeps
        (Ctrl toggles a clicked tile, Ctrl+drag adds a box). */
    onselect?: (tiles: { x: number; y: number }[], add: boolean) => void;
  } = $props();
  type TilePoint = { x: number; y: number };
  let hover = $state<TilePoint | null>(null);
  let drag = $state<{ from: TilePoint; to: TilePoint; add: boolean } | null>(null);
  let hit: HTMLDivElement | undefined = $state();

  function tileAt(event: MouseEvent, keepInside = false): TilePoint | null {
    const bounds = hit!.getBoundingClientRect();
    let x = Math.floor(((event.clientX - bounds.left) / bounds.width) * preview.w);
    let y = Math.floor(((event.clientY - bounds.top) / bounds.height) * preview.h);
    if (keepInside) {
      x = clamp(x, 0, preview.w - 1);
      y = clamp(y, 0, preview.h - 1);
    }
    return x >= 0 && y >= 0 && x < preview.w && y < preview.h ? { x, y } : null;
  }

  function boxOf(area: { from: TilePoint; to: TilePoint }) {
    return { x0: Math.min(area.from.x, area.to.x), y0: Math.min(area.from.y, area.to.y), x1: Math.max(area.from.x, area.to.x), y1: Math.max(area.from.y, area.to.y) };
  }
  const box = $derived(drag && boxOf(drag));

  function down(event: MouseEvent) {
    if (event.button !== 0) return;
    const start = tileAt(event);
    if (!start) return;
    event.preventDefault();
    // Leave text fields: Delete then works on the chosen tiles.
    (document.activeElement as HTMLElement | null)?.blur();
    drag = { from: start, to: start, add: event.ctrlKey || event.metaKey || event.shiftKey };
    const move = (moveEvent: MouseEvent) => drag && (drag.to = tileAt(moveEvent, true)!);
    const up = () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
      const area = drag;
      drag = null;
      if (!area) return;
      const rect = boxOf(area);
      const single = rect.x0 === rect.x1 && rect.y0 === rect.y1;
      if (single && area.add) {
        // Ctrl+click toggles one tile.
        const on = selected.some((chosen) => chosen.x === rect.x0 && chosen.y === rect.y0);
        onselect?.(on ? selected.filter((chosen) => chosen.x !== rect.x0 || chosen.y !== rect.y0) : [...selected, { x: rect.x0, y: rect.y0 }], false);
        return;
      }
      const tiles: TilePoint[] = [];
      for (let y = rect.y0; y <= rect.y1; y++) for (let x = rect.x0; x <= rect.x1; x++) tiles.push({ x, y });
      onselect?.(tiles, area.add);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  }

  /** Pixels per tile of the drawn canvas. */
  const TILE_PIXELS = 32;
  let width = $state(0);
  let canvas = $state<HTMLCanvasElement>();
  const tile = $derived(Math.max(4, Math.min(max, Math.floor(width / preview.w) || max)));

  const images = new Map<string, Promise<HTMLImageElement | null>>();
  function load(src: string): Promise<HTMLImageElement | null> {
    let loading = images.get(src);
    if (!loading) {
      loading = new Promise((done) => {
        const img = new Image();
        img.onload = () => done(img);
        img.onerror = () => done(null);
        img.src = src;
      });
      images.set(src, loading);
    }
    return loading;
  }

  $effect(() => {
    const target = canvas;
    const shown = preview;
    const items = map.items;
    if (!target || !items) return;
    let stale = false;
    // Row by row, items bottom to top, as the client draws.
    const order = [...(shown.tiles ?? [])].filter((tile) => !!tile).sort((first, second) => first!.y - second!.y || first!.x - second!.x);
    // Client stacking within first tile: ground, borders, bottom, common, top.
    const rank = (id: number) => (items.is(id, "ground") ? 0 : ([3, 1, 2, 4][items.top[id]] ?? 3));
    const draws = order.flatMap((tile) =>
      [...(tile!.items ?? [])].map((id, stackIndex) => ({ id, stackIndex })).sort((first, second) => rank(first.id) - rank(second.id) || first.stackIndex - second.stackIndex).map(({ id }) => {
        const clientId = items.client[id] ?? 0;
        // Uncropped, in the pattern of its tile: thumbnails are cropped to
        // their pixels and would not line up.
        return { tile: tile!, src: clientId ? res.textureAt(Category.CategoryItem, clientId, tile!.x, tile!.y, 7, versions.thing(Category.CategoryItem, clientId)) : "" };
      }),
    );
    void Promise.all(draws.map((draw) => (draw.src ? load(draw.src) : Promise.resolve(null)))).then((imgs) => {
      if (stale) return;
      const ctx = target.getContext("2d");
      if (!ctx) return;
      target.width = shown.w * TILE_PIXELS;
      target.height = shown.h * TILE_PIXELS;
      ctx.fillStyle = "#000";
      ctx.fillRect(0, 0, target.width, target.height);
      imgs.forEach((img, i) => {
        if (!img) return;
        const { tile } = draws[i];
        ctx.drawImage(img, (tile.x + 1) * TILE_PIXELS - img.width, (tile.y + 1) * TILE_PIXELS - img.height);
      });
    });
    return () => (stale = true);
  });
</script>

<div class="host" bind:clientWidth={width}>
  <div class="box" style="width:{preview.w * tile}px;height:{preview.h * tile}px">
    <canvas bind:this={canvas}></canvas>
    {#if onselect}
      {#each selected as chosen}<div class="mark sel" style="left:{chosen.x * tile}px;top:{chosen.y * tile}px;width:{tile}px;height:{tile}px"></div>{/each}
      {#if box}
        <div class="mark boxed" style="left:{box.x0 * tile}px;top:{box.y0 * tile}px;width:{(box.x1 - box.x0 + 1) * tile}px;height:{(box.y1 - box.y0 + 1) * tile}px"></div>
      {:else if hover}
        <div class="mark hover" style="left:{hover.x * tile}px;top:{hover.y * tile}px;width:{tile}px;height:{tile}px"></div>
      {/if}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="hit" bind:this={hit} onmousedown={down} onmousemove={(event) => (hover = tileAt(event))} onmouseleave={() => (hover = null)}></div>
    {/if}
  </div>
</div>

<style>
  .host {
    width: 100%;
    min-width: 0;
  }
  .box {
    position: relative;
    margin: 0 auto;
  }
  canvas {
    display: block;
    width: 100%;
    height: 100%;
  }
  .hit {
    position: absolute;
    inset: 0;
    cursor: pointer;
  }
  .mark {
    position: absolute;
    pointer-events: none;
    box-sizing: border-box;
  }
  .mark.hover {
    border: 1px solid rgba(255, 255, 255, 0.6);
  }
  .mark.sel {
    background: rgba(232, 196, 94, 0.25);
    border: 1px solid var(--gold);
  }
  .mark.boxed {
    border: 1px dashed #fff;
  }
</style>
