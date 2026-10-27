<script lang="ts">
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
    zoom?: number;
    onzoom?: (delta: number) => void;
    resetKey?: string;
    stageSize?: [number, number];
  } = $props();

  let stageWidth = $state(0);
  let stageHeight = $state(0);
  let contentWidth = $state(0);
  let contentHeight = $state(0);
  const view = new PanView(
    () => ({ width: contentWidth * zoom, height: contentHeight * zoom, stageWidth, stageHeight }),
    (delta) => onzoom?.(delta),
  );

  let animate = $state(false);
  let glideTimer: ReturnType<typeof setTimeout> | undefined;
  function glide() {
    animate = true;
    clearTimeout(glideTimer);
    glideTimer = setTimeout(() => (animate = false), 220);
  }

  $effect(() => {
    stageSize = [stageWidth, stageHeight];
  });
  let lastZoom = 0;
  $effect(() => {
    const nextZoom = zoom;
    if (lastZoom && nextZoom !== lastZoom) {
      glide();
      view.rescale(nextZoom / lastZoom);
    }
    lastZoom = nextZoom;
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
<div class="pan-stage" class:ready={view.ready} class:panning={view.dragging} bind:clientWidth={stageWidth} bind:clientHeight={stageHeight} {...view.handlers}>
  <div
    class="content"
    class:animate={animate && !view.dragging}
    bind:offsetWidth={contentWidth}
    bind:offsetHeight={contentHeight}
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
