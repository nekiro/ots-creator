<script lang="ts">
  // Every texture of a frame group at once (ObjectBuilder sheet layout:
  // columns pattern Z × X × layers, rows frames × pattern Y). Each tile is a
  // drop target; dropping several sprites fills the following slots.
  import { fitZoom as zoomToFit } from "../lib/render/zoom";
  import type { FrameGroup } from "../lib/api";
  import { compose, type PixelSource } from "../lib/render/compose";
  import { sheetCellPos, sheetGrid, spriteIndex, tileOffset, type TexturePos } from "../lib/render/layout";
  import { spriteCache } from "../lib/render/sprites";
  import { dragsMany, droppedSprites } from "../lib/dragsprites";
  import type { OutfitColors } from "../lib/render/outfit";
  import { PanView } from "../lib/pan.svelte";

  /** The pixel grid shows from this zoom up (same as the single view). */
  const PIXEL_GRID_ZOOM = 4;

  let {
    frameGroup,
    size,
    loaded,
    selected,
    showGrid,
    pixelGrid = false,
    colors = null,
    resetKey = "",
    current,
    onassign,
    onpick,
    onopen,
    manualZoom = null,
    fitZoom = $bindable(1),
    onwheelzoom,
    get = (id) => spriteCache.get(id)?.pixels,
    readonly = false,
  }: {
    frameGroup: FrameGroup;
    size: number;
    /** Bumps when sprites arrive. */
    loaded: number;
    selected: number[];
    showGrid: boolean;
    /** Lines between single pixels (from PIXEL_GRID_ZOOM up). */
    pixelGrid?: boolean;
    /** Outfit colors painted on layer 0 textures; mask layers stay as they are. */
    colors?: OutfitColors | null;
    /** A change centers the sheet again (another object or frame group). */
    resetKey?: string;
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
    /** Pixels of the sprites; the open client's by default. */
    get?: PixelSource;
    /** No sprites can be dropped. */
    readonly?: boolean;
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let stage = $state<HTMLDivElement>();
  let stageSize = $state<[number, number]>([0, 0]);
  let dropSlot = $state<number | null>(null);
  let dropCount = $state(1);

  const GAP = 4; // pixels between textures (unscaled)
  const grid = $derived(sheetGrid(frameGroup));
  const textureWidth = $derived(frameGroup.width * size);
  const textureHeight = $derived(frameGroup.height * size);
  const sheetWidth = $derived(grid.columns * (textureWidth + GAP) - GAP);
  const sheetHeight = $derived(grid.rows * (textureHeight + GAP) - GAP);
  const STAGE_MARGIN = 24;
  // Like the preview: no huge zoom on big screens.
  const MAX_FIT_ZOOM = 4;
  const fit = $derived(stageSize[0] ? zoomToFit(sheetWidth, sheetHeight, stageSize[0], stageSize[1], MAX_FIT_ZOOM, STAGE_MARGIN, 1) : 1);
  $effect(() => {
    fitZoom = fit;
  });
  const zoom = $derived(manualZoom ?? fit);

  // A canvas, not a scrolled box: the sheet is centered and panned
  // (space + drag, middle button drag, wheel; ctrl+wheel zooms).
  const view = new PanView(
    () => ({ width: sheetWidth * zoom, height: sheetHeight * zoom, stageWidth: stageSize[0], stageHeight: stageSize[1] }),
    (delta) => onwheelzoom?.(delta),
  );
  // Zooming keeps the centered pixel; fitting the window or another object centers.
  let lastZoom = 0;
  $effect(() => {
    const current = zoom;
    if (lastZoom && current !== lastZoom) view.rescale(current / lastZoom);
    lastZoom = current;
  });
  $effect(() => {
    if (manualZoom === null) view.reset();
  });
  let lastKey = "";
  $effect(() => {
    if (resetKey !== lastKey) view.reset();
    lastKey = resetKey;
  });

  // Every slot with its position on the (unscaled) sheet.
  const tiles = $derived.by(() => {
    const out: { slot: number; x: number; y: number; pos: TexturePos }[] = [];
    for (let row = 0; row < grid.rows; row++)
      for (let column = 0; column < grid.columns; column++) {
        const pos = sheetCellPos(frameGroup, column, row);
        for (let tileY = 0; tileY < frameGroup.height; tileY++)
          for (let tileX = 0; tileX < frameGroup.width; tileX++) {
            const [offsetX, offsetY] = tileOffset(frameGroup, tileX, tileY, size);
            out.push({ slot: spriteIndex(frameGroup, tileX, tileY, pos), x: column * (textureWidth + GAP) + offsetX, y: row * (textureHeight + GAP) + offsetY, pos });
          }
      }
    return out;
  });
  // Slots a drop of dropCount sprites at dropSlot would fill.
  const filling = $derived(dropSlot === null ? null : new Set(Array.from({ length: dropCount }, (_, i) => dropSlot! + i)));
  const selectedSet = $derived(new Set(selected));
  const isCurrent = (pos: TexturePos) =>
    pos.layer === current.layer && pos.x === current.x && pos.y === current.y && pos.z === current.z && pos.frame === current.frame;

  $effect(() => {
    if (!stage) return;
    const resizeObserver = new ResizeObserver(() => (stageSize = [stage!.clientWidth, stage!.clientHeight]));
    resizeObserver.observe(stage);
    return () => resizeObserver.disconnect();
  });

  // Draw the whole sheet at 1:1; CSS scales it with pixelated rendering.
  $effect(() => {
    void loaded;
    if (!canvas) return;
    canvas.width = Math.max(1, sheetWidth);
    canvas.height = Math.max(1, sheetHeight);
    const ctx = canvas.getContext("2d")!;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    for (let row = 0; row < grid.rows; row++)
      for (let column = 0; column < grid.columns; column++) {
        const image = compose(frameGroup, frameGroup.sprites, size, { pos: sheetCellPos(frameGroup, column, row), colors }, get);
        ctx.putImageData(image, column * (textureWidth + GAP), row * (textureHeight + GAP));
      }
  });

  function dragOver(event: DragEvent, slot: number) {
    if (readonly) return;
    event.preventDefault();
    dropSlot = slot;
    // The ids are only readable on drop; a multi drag is the selection.
    dropCount = dragsMany(event) ? Math.max(1, selected.length) : 1;
  }

  function drop(event: DragEvent, slot: number) {
    event.preventDefault();
    dropSlot = null;
    const ids = droppedSprites(event);
    if (ids.length) onassign(slot, ids);
  }
</script>

<svelte:window onkeydown={view.key} onkeyup={view.key} onblur={view.blur} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="sheet checker" class:pan-ready={view.ready} class:panning={view.dragging} bind:this={stage} {...view.handlers}>
  <div class="wrap" style="width:{sheetWidth * zoom}px;height:{sheetHeight * zoom}px;transform:{view.transform}">
    <canvas bind:this={canvas} class="pixel" style="width:100%;height:100%"></canvas>
    {#if pixelGrid && zoom >= PIXEL_GRID_ZOOM}
      <div class="pixel-grid" style="background-size:{zoom}px {zoom}px"></div>
    {/if}
    {#each Array.from({ length: grid.rows * grid.columns }, (_, i) => i) as i (i)}
      {@const column = i % grid.columns}
      {@const row = Math.floor(i / grid.columns)}
      <div
        class="tex"
        class:cur={isCurrent(sheetCellPos(frameGroup, column, row))}
        style="left:{column * (textureWidth + GAP) * zoom - 1}px;top:{row * (textureHeight + GAP) * zoom - 1}px;width:{textureWidth * zoom + 2}px;height:{textureHeight * zoom + 2}px"
      ></div>
    {/each}
    {#each tiles as tile (tile.slot)}
      {@const id = frameGroup.sprites[tile.slot]}
      <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
      <div
        class="slot"
        class:grid={showGrid}
        class:sel={id !== 0 && selectedSet.has(id)}
        class:drop={filling?.has(tile.slot)}
        style="left:{tile.x * zoom}px;top:{tile.y * zoom}px;width:{size * zoom}px;height:{size * zoom}px"
        title="Frame {tile.pos.frame + 1}, pattern {tile.pos.x},{tile.pos.y},{tile.pos.z}, layer {tile.pos.layer}: slot {tile.slot} → sprite #{id}{id ? '' : ' (empty)'}"
        onclick={() => onpick(tile.slot, tile.pos)}
        ondblclick={() => onopen(tile.pos)}
        ondragover={(event) => dragOver(event, tile.slot)}
        ondragleave={() => (dropSlot = null)}
        ondrop={(event) => drop(event, tile.slot)}
      ></div>
    {/each}
  </div>
</div>

<style>
  .sheet {
    position: relative;
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    display: grid;
    place-items: center;
    border: 1px solid #111;
    box-shadow: inset 0 0 0 1px #4a4a4a;
  }
  .wrap {
    position: absolute;
    left: 50%;
    top: 50%;
  }
  .pixel-grid {
    position: absolute;
    inset: 0;
    pointer-events: none;
    background-image:
      linear-gradient(to right, rgba(128, 128, 128, 0.35) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(128, 128, 128, 0.35) 1px, transparent 1px);
  }
  .sheet.pan-ready {
    cursor: grab;
  }
  .sheet.panning {
    cursor: grabbing;
  }
  .sheet.pan-ready .slot,
  .sheet.panning .slot {
    pointer-events: none;
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
