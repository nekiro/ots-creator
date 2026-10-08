<script lang="ts">
  // Every texture of a frame group at once (ObjectBuilder sheet layout:
  // columns pattern Z × X × layers, rows frames × pattern Y). Each tile is a
  // drop target; dropping several sprites fills the following slots.
  import type { FrameGroup } from "../lib/api";
  import { compose } from "../lib/render/compose";
  import { sheetCellPos, sheetGrid, spriteIndex, tileOffset, type TexturePos } from "../lib/render/layout";
  import { spriteCache } from "../lib/render/sprites";
  import { dragsMany, droppedSprites } from "../lib/dragsprites";

  let {
    g,
    size,
    loaded,
    selected,
    showGrid,
    current,
    onassign,
    onpick,
    onopen,
    manualZoom = null,
    fitZoom = $bindable(1),
    onwheelzoom,
  }: {
    g: FrameGroup;
    size: number;
    /** Bumps when sprites arrive. */
    loaded: number;
    selected: number[];
    showGrid: boolean;
    /** Texture shown in the single view, highlighted here. */
    current: TexturePos;
    onassign: (slot: number, ids: number[]) => void;
    onpick: (slot: number, pos: TexturePos) => void;
    /** Double click: show this texture in the single view. */
    onopen: (pos: TexturePos) => void;
    /** Zoom chosen by hand; null fits the window. */
    manualZoom?: number | null;
    /** Out: the zoom that fits the window. */
    fitZoom?: number;
    /** Ctrl+wheel: +1 or -1. */
    onwheelzoom?: (delta: number) => void;
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let stage = $state<HTMLDivElement>();
  let stageSize = $state<[number, number]>([0, 0]);
  let dropSlot = $state<number | null>(null);
  let dropCount = $state(1);

  const GAP = 4; // pixels between textures (unscaled)
  const grid = $derived(sheetGrid(g));
  const texW = $derived(g.width * size);
  const texH = $derived(g.height * size);
  const sheetW = $derived(grid.columns * (texW + GAP) - GAP);
  const sheetH = $derived(grid.rows * (texH + GAP) - GAP);
  const fit = $derived.by(() => {
    if (!stageSize[0]) return 1;
    const r = Math.min((stageSize[0] - 24) / sheetW, (stageSize[1] - 24) / sheetH);
    return Math.min(8, Math.max(1, Math.floor(r)));
  });
  $effect(() => {
    fitZoom = fit;
  });
  const zoom = $derived(manualZoom ?? fit);

  function onwheel(e: WheelEvent) {
    if (!e.ctrlKey) return;
    e.preventDefault();
    onwheelzoom?.(e.deltaY < 0 ? 1 : -1);
  }

  // Every slot with its position on the (unscaled) sheet.
  const tiles = $derived.by(() => {
    const out: { slot: number; x: number; y: number; pos: TexturePos }[] = [];
    for (let r = 0; r < grid.rows; r++)
      for (let c = 0; c < grid.columns; c++) {
        const pos = sheetCellPos(g, c, r);
        for (let h = 0; h < g.height; h++)
          for (let w = 0; w < g.width; w++) {
            const [ox, oy] = tileOffset(g, w, h, size);
            out.push({ slot: spriteIndex(g, w, h, pos), x: c * (texW + GAP) + ox, y: r * (texH + GAP) + oy, pos });
          }
      }
    return out;
  });
  // Slots a drop of dropCount sprites at dropSlot would fill.
  const filling = $derived(dropSlot === null ? null : new Set(Array.from({ length: dropCount }, (_, i) => dropSlot! + i)));
  const selectedSet = $derived(new Set(selected));
  const isCurrent = (p: TexturePos) =>
    p.layer === current.layer && p.x === current.x && p.y === current.y && p.z === current.z && p.frame === current.frame;

  $effect(() => {
    if (!stage) return;
    const ro = new ResizeObserver(() => (stageSize = [stage!.clientWidth, stage!.clientHeight]));
    ro.observe(stage);
    return () => ro.disconnect();
  });

  // Draw the whole sheet at 1:1; CSS scales it with pixelated rendering.
  $effect(() => {
    void loaded;
    if (!canvas) return;
    canvas.width = Math.max(1, sheetW);
    canvas.height = Math.max(1, sheetH);
    const ctx = canvas.getContext("2d")!;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    const get = (id: number) => spriteCache.get(id)?.pixels;
    for (let r = 0; r < grid.rows; r++)
      for (let c = 0; c < grid.columns; c++) {
        const img = compose(g, g.sprites, size, { pos: sheetCellPos(g, c, r) }, get);
        ctx.putImageData(img, c * (texW + GAP), r * (texH + GAP));
      }
  });

  function dragOver(e: DragEvent, slot: number) {
    e.preventDefault();
    dropSlot = slot;
    // The ids are only readable on drop; a multi drag is the selection.
    dropCount = dragsMany(e) ? Math.max(1, selected.length) : 1;
  }

  function drop(e: DragEvent, slot: number) {
    e.preventDefault();
    dropSlot = null;
    const ids = droppedSprites(e);
    if (ids.length) onassign(slot, ids);
  }
</script>

<div class="sheet checker" bind:this={stage} {onwheel}>
  <div class="wrap" style="width:{sheetW * zoom}px;height:{sheetH * zoom}px">
    <canvas bind:this={canvas} class="pixel" style="width:100%;height:100%"></canvas>
    {#each Array.from({ length: grid.rows * grid.columns }, (_, i) => i) as i (i)}
      {@const c = i % grid.columns}
      {@const r = Math.floor(i / grid.columns)}
      <div
        class="tex"
        class:cur={isCurrent(sheetCellPos(g, c, r))}
        style="left:{c * (texW + GAP) * zoom - 1}px;top:{r * (texH + GAP) * zoom - 1}px;width:{texW * zoom + 2}px;height:{texH * zoom + 2}px"
      ></div>
    {/each}
    {#each tiles as t (t.slot)}
      {@const id = g.sprites[t.slot]}
      <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
      <div
        class="slot"
        class:grid={showGrid}
        class:sel={id !== 0 && selectedSet.has(id)}
        class:drop={filling?.has(t.slot)}
        style="left:{t.x * zoom}px;top:{t.y * zoom}px;width:{size * zoom}px;height:{size * zoom}px"
        title="Frame {t.pos.frame + 1}, pattern {t.pos.x},{t.pos.y},{t.pos.z}, layer {t.pos.layer}: slot {t.slot} → sprite #{id}{id ? '' : ' (empty)'}"
        onclick={() => onpick(t.slot, t.pos)}
        ondblclick={() => onopen(t.pos)}
        ondragover={(e) => dragOver(e, t.slot)}
        ondragleave={() => (dropSlot = null)}
        ondrop={(e) => drop(e, t.slot)}
      ></div>
    {/each}
  </div>
</div>

<style>
  .sheet {
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    display: grid;
    place-items: center;
    border: 1px solid #111;
    box-shadow: inset 0 0 0 1px #4a4a4a;
  }
  .wrap {
    position: relative;
    margin: 12px;
  }
  .tex {
    position: absolute;
    border: 1px solid rgba(0, 0, 0, 0.55);
    pointer-events: none;
  }
  .tex.cur {
    border-color: var(--gold);
  }
  .slot {
    position: absolute;
    cursor: pointer;
  }
  .slot.grid {
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.12);
  }
  .slot:hover {
    background: rgba(255, 255, 255, 0.1);
  }
  .slot.sel {
    box-shadow: inset 0 0 0 1px var(--blue);
  }
  .slot.drop {
    background: rgba(80, 140, 255, 0.35);
    box-shadow: inset 0 0 0 1px #8fb6ff;
  }
</style>
