<script lang="ts">
  // A canvas-like view for images in dialogs, like the main preview: the
  // content is centered, there are no scrollbars, and it is moved with
  // space + drag, the middle button or the wheel (ctrl+wheel zooms).
  // The content is drawn at 1x and scaled by `zoom`; zoom changes and
  // centering animate.
  import type { Snippet } from "svelte";
  import { PanView } from "../pan.svelte";

  let {
    children,
    zoom = 1,
    onzoom,
    resetKey = "",
    stageSize = $bindable<[number, number]>([0, 0]),
  }: {
    children: Snippet;
    /** Scale of the content (pixelated). */
    zoom?: number;
    /** Ctrl+wheel: +1 or -1. */
    onzoom?: (delta: number) => void;
    /** A change centers the content again (another file or object). */
    resetKey?: string;
    /** Out: the size of the stage, for fit-to-window zooms. */
    stageSize?: [number, number];
  } = $props();

  let sw = $state(0);
  let sh = $state(0);
  let cw = $state(0);
  let ch = $state(0);
  const view = new PanView(
    () => ({ w: cw * zoom, h: ch * zoom, sw, sh }),
    (d) => onzoom?.(d),
  );

  // Zoom and centering glide; panning by hand follows the mouse directly.
  let animate = $state(false);
  let animTimer: ReturnType<typeof setTimeout> | undefined;
  function glide() {
    animate = true;
    clearTimeout(animTimer);
    animTimer = setTimeout(() => (animate = false), 220);
  }

  $effect(() => {
    stageSize = [sw, sh];
  });
  // Zooming keeps the centered pixel in place.
  let lastZoom = 0;
  $effect(() => {
    const z = zoom;
    if (lastZoom && z !== lastZoom) {
      glide();
      view.rescale(z / lastZoom);
    }
    lastZoom = z;
  });
  let lastKey: string | undefined;
  $effect(() => {
    if (lastKey !== undefined && resetKey !== lastKey) {
      glide();
      view.reset();
    }
    lastKey = resetKey;
  });
</script>

<svelte:window onkeydown={view.key} onkeyup={view.key} onblur={view.blur} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="pan-stage" class:ready={view.ready} class:panning={view.dragging} bind:clientWidth={sw} bind:clientHeight={sh} {...view.handlers}>
  <div
    class="content"
    class:animate={animate && !view.dragging}
    bind:offsetWidth={cw}
    bind:offsetHeight={ch}
    style="transform:{view.transform} scale({zoom})"
  >
    {@render children()}
  </div>
</div>

<style>
  .pan-stage {
    position: absolute;
    inset: 0;
    overflow: hidden;
  }
  .content {
    position: absolute;
    left: 50%;
    top: 50%;
    display: flex;
    width: max-content; /* never squeezed by the stage edge */
  }
  .content.animate {
    transition: transform 0.2s ease-out;
  }
  .ready {
    cursor: grab;
  }
  .panning {
    cursor: grabbing;
  }
  .ready .content,
  .panning .content {
    pointer-events: none;
  }
</style>
