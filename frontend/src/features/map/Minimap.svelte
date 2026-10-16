<script lang="ts">
  import { onMount } from "svelte";
  import { res } from "../../lib/map/api";
  import { goTo, spriteSize } from "../../lib/map/commands";
  import { worldToTile } from "../../lib/map/coords";
  import { clamp } from "../../lib/math";
  import { readPositive, writeStored } from "../../lib/storage";
  import { map, onMapChange } from "../../lib/map/state.svelte";
  import MiniWindow from "../../lib/ui/MiniWindow.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Splitter from "../../lib/ui/Splitter.svelte";

  /** Tiles shown across the minimap. */
  const SPANS = [64, 128, 256, 512];
  let span = $state(256);
  let rev = $state(0);
  let box = $state<HTMLDivElement>();
  let size = $state(200);

  // Height chosen with the splitter below the image; square (null) until dragged.
  const HEIGHT_KEY = "map:minimap-height";
  const HEIGHT_MIN = 80;
  let chosen = $state<number | null>(loadHeight());
  const height = $derived(chosen ?? size);
  /** Tiles shown down the minimap: the image keeps square tiles. */
  const spanY = $derived(Math.max(8, Math.round((span * height) / Math.max(1, size))));

  function loadHeight(): number | null {
    return readPositive(HEIGHT_KEY);
  }

  function resize(dy: number) {
    const max = Math.max(HEIGHT_MIN, window.innerHeight - 320);
    chosen = Math.round(clamp(height + dy, HEIGHT_MIN, max));
  }

  function saveHeight() {
    writeStored(HEIGHT_KEY, String(chosen ?? 0));
  }

  const center = $derived.by(() => {
    const cam = map.cam;
    return { ...worldToTile(cam.x, cam.y, cam.floor, spriteSize()), z: cam.floor };
  });
  // The image moves in steps so it is not fetched on every pixel of a pan.
  const step = $derived(Math.max(8, span / 8));
  const live = $derived({ x: Math.max(0, Math.round((center.x - span / 2) / step) * step), y: Math.max(0, Math.round((center.y - spanY / 2) / step) * step) });
  // While dragging, the image stays still: if it followed the camera, the same pointer
  // position would point at a new tile on every move and the camera would run away.
  let held = $state<{ x: number; y: number } | null>(null);
  const origin = $derived(held ?? live);
  const src = $derived(map.activeId !== null ? res.minimap(map.activeId, origin.x, origin.y, center.z, span, spanY, rev) : "");

  // The view rectangle of the map canvas.
  const view = $derived.by(() => {
    const cam = map.cam;
    const tileSize = spriteSize();
    const canvas = document.querySelector<HTMLCanvasElement>("canvas.map-gl");
    const w = canvas ? canvas.clientWidth / cam.zoom / tileSize : 40;
    const h = canvas ? canvas.clientHeight / cam.zoom / tileSize : 30;
    return { x: center.x - w / 2 - origin.x, y: center.y - h / 2 - origin.y, w, h };
  });

  let timer = 0;
  onMount(() =>
    onMapChange((id) => {
      if (id !== map.activeId) return;
      clearTimeout(timer);
      timer = window.setTimeout(() => rev++, 400);
    }),
  );

  function at(event: PointerEvent) {
    const rect = box!.getBoundingClientRect();
    let x = origin.x + Math.floor(((event.clientX - rect.left) / rect.width) * span);
    let y = origin.y + Math.floor(((event.clientY - rect.top) / rect.height) * spanY);
    // Stay where the map has tiles; outside it there is only black.
    const info = map.active;
    if (info?.hasBounds) {
      const [x0, y0, x1, y1] = info.bounds;
      x = clamp(x, x0, x1);
      y = clamp(y, y0, y1);
    }
    goTo({ x: clamp(x, 0, 65535), y: clamp(y, 0, 65535), z: center.z });
  }

  function onpointerdown(event: PointerEvent) {
    if (event.button !== 0) return;
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
    held = { ...live };
    at(event);
  }

  function release() {
    held = null;
  }
</script>

<MiniWindow title="Minimap" class="minimap">
  {#snippet actions()}
    <button class="t-icon-btn" title="Zoom out" disabled={span >= SPANS[SPANS.length - 1]} onclick={() => (span = SPANS[Math.min(SPANS.length - 1, SPANS.indexOf(span) + 1)])}><Icon name="zoomout" /></button>
    <button class="t-icon-btn" title="Zoom in" disabled={span <= SPANS[0]} onclick={() => (span = SPANS[Math.max(0, SPANS.indexOf(span) - 1)])}><Icon name="zoomin" /></button>
  {/snippet}
  <div class="wrap" bind:clientWidth={size}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="box" bind:this={box} style="height:{height}px" onpointerdown={onpointerdown} onpointermove={(event) => held && event.buttons === 1 && at(event)} onpointerup={release} onpointercancel={release} onlostpointercapture={release}>
      {#if src}<img class="pixel" {src} alt="" draggable="false" />{/if}
      <div
        class="view"
        style="left:{(view.x / span) * 100}%;top:{(view.y / spanY) * 100}%;width:{(view.w / span) * 100}%;height:{(view.h / spanY) * 100}%"
      ></div>
    </div>
  </div>
  <Splitter ondrag={resize} onend={saveHeight} />
  <div class="t-label foot">Floor {center.z} · {span}×{spanY} tiles</div>
</MiniWindow>

<style>
  .wrap {
    padding: 2px;
  }
  .box {
    position: relative;
    background: #000;
    overflow: hidden;
    cursor: crosshair;
    border: 1px solid #111;
  }
  img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .view {
    position: absolute;
    border: 1px solid #fff;
    box-shadow: 0 0 0 1px #000;
    pointer-events: none;
  }
  .foot {
    font: var(--fs-small) / 12px var(--font-small);
    padding: 2px;
  }
</style>
