<script lang="ts">
  import { onMount } from "svelte";
  import { res } from "../../lib/map/data/api";
  import { goTo, spriteSize } from "../../lib/map/view/commands";
  import { worldToTile } from "../../lib/map/view/coords";
  import { clamp } from "../../lib/util/math";
  import { readPositive, writeStored } from "../../lib/state/storage";
  import { map, onMapChange } from "../../lib/map/state.svelte";
  import MiniWindow from "../../lib/ui/MiniWindow.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Splitter from "../../lib/ui/Splitter.svelte";

  /** Tiles shown across the minimap. */
  const SPANS = [64, 128, 256, 512];
  let span = $state(256);
  let rev = $state(0);
  let box = $state<HTMLDivElement>();
  let layer = $state<HTMLDivElement>();
  let size = $state(200);
  // Inside of the box (no border): the image fills its width.
  let innerWidth = $state(0);
  // Shown height inside the box: below the chosen one when the column has no room for it.
  let shown = $state(0);

  // Height chosen with the splitter below the image; square (null) until dragged.
  const HEIGHT_KEY = "map:minimap-height";
  const HEIGHT_MIN = 80;
  let chosen = $state<number | null>(loadHeight());
  const height = $derived(chosen ?? size);
  const maxHeight = () => Math.max(HEIGHT_MIN, window.innerHeight - 320);
  /** Tiles down a box this tall: the image keeps square tiles. */
  const tilesDown = (pixels: number) => Math.max(8, Math.round((span * pixels) / Math.max(1, innerWidth || size)));
  // While the splitter is dragged, one image as tall as the box can get is fetched,
  // its top kept where it was: the box shows more of it with every pixel of the drag.
  let drag = $state<{ from: number; moved: number; spanY: number; top: number } | null>(null);
  const spanY = $derived(drag?.spanY ?? tilesDown(shown || height));

  function loadHeight(): number | null {
    return readPositive(HEIGHT_KEY);
  }

  function resize(dy: number) {
    // The splitter reports each move apart; adding them up from the height at the
    // start keeps the drag under the pointer whatever the box's border and layout do.
    if (!drag) drag = { from: box?.offsetHeight || height, moved: 0, spanY: tilesDown(maxHeight()), top: origin.y };
    drag.moved += dy;
    chosen = Math.round(clamp(drag.from + drag.moved, HEIGHT_MIN, maxHeight()));
  }

  function saveHeight() {
    drag = null;
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
  const origin = $derived(held ?? (drag ? { x: live.x, y: drag.top } : live));
  const src = $derived(map.activeId !== null ? res.minimap(map.activeId, origin.x, origin.y, center.z, span, spanY, rev) : "");

  // The image on screen: a new one replaces it only once loaded, so the box never
  // goes black while the next is fetched. The view rectangle and clicks follow it.
  // The loaded image itself is drawn (on a canvas), not its URL again: that could fetch it twice.
  let image = $state<{ picture: HTMLImageElement; x: number; y: number; span: number; spanY: number } | null>(null);
  $effect(() => {
    if (!src) {
      image = null;
      return;
    }
    const loader = new Image();
    const next = { picture: loader, x: origin.x, y: origin.y, span, spanY };
    loader.onload = () => {
      image = next;
    };
    loader.src = src;
    return () => {
      loader.onload = null;
    };
  });

  // The view rectangle of the map canvas.
  const view = $derived.by(() => {
    if (!image) return null;
    const cam = map.cam;
    const tileSize = spriteSize();
    const canvas = document.querySelector<HTMLCanvasElement>("canvas.map-gl");
    const width = canvas ? canvas.clientWidth / cam.zoom / tileSize : 40;
    const height = canvas ? canvas.clientHeight / cam.zoom / tileSize : 30;
    return { x: center.x - width / 2 - image.x, y: center.y - height / 2 - image.y, w: width, h: height };
  });

  let timer = 0;
  onMount(() =>
    onMapChange((id) => {
      if (id !== map.activeId) return;
      clearTimeout(timer);
      timer = window.setTimeout(() => rev++, 400);
    }),
  );

  function paint(canvas: HTMLCanvasElement, picture: HTMLImageElement) {
    canvas.width = picture.naturalWidth;
    canvas.height = picture.naturalHeight;
    canvas.getContext("2d")?.drawImage(picture, 0, 0);
  }

  function at(event: PointerEvent) {
    if (!image || !layer) return;
    const rect = layer.getBoundingClientRect();
    let x = image.x + Math.floor(((event.clientX - rect.left) / rect.width) * image.span);
    let y = image.y + Math.floor(((event.clientY - rect.top) / rect.height) * image.spanY);
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
    <div class="box" bind:this={box} style="height:{height}px" bind:clientWidth={innerWidth} bind:clientHeight={shown} onpointerdown={onpointerdown} onpointermove={(event) => held && event.buttons === 1 && at(event)} onpointerup={release} onpointercancel={release} onlostpointercapture={release}>
      {#if image && view}
        <!-- Square tiles: the image is as tall as its tiles across the box's width; the box crops it. -->
        <div class="layer" bind:this={layer} style:height="{((innerWidth || size) * image.spanY) / image.span}px">
          {#key image.picture}<canvas class="pixel" use:paint={image.picture}></canvas>{/key}
          <div
            class="view"
            style="left:{(view.x / image.span) * 100}%;top:{(view.y / image.spanY) * 100}%;width:{(view.w / image.span) * 100}%;height:{(view.h / image.spanY) * 100}%"
          ></div>
        </div>
      {/if}
    </div>
  </div>
  <Splitter ondrag={resize} onend={saveHeight} />
  <div class="t-label foot">Floor {center.z} · {span}×{tilesDown(shown || height)} tiles</div>
</MiniWindow>

<style>
  /* Both shrink so the image stays inside the window when the column is short. */
  .wrap {
    flex: 0 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
    padding: 2px;
  }
  .box {
    flex: 0 1 auto;
    min-height: 40px;
    position: relative;
    background: #000;
    overflow: hidden;
    cursor: crosshair;
    border: 1px solid #111;
  }
  .layer {
    position: absolute;
    left: 0;
    top: 0;
    width: 100%;
  }
  canvas {
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
