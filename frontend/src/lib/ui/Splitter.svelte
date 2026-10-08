<script lang="ts">
  // OTClient Splitter: invisible 4px handle that shows #ffffff44 while
  // hovered or dragged. Reports the pointer's vertical movement.
  import { toCss } from "../pixelscale";

  let { ondrag, onend }: { ondrag: (dy: number) => void; onend?: () => void } = $props();
  let active = $state(false);

  function onpointerdown(e: PointerEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    const el = e.currentTarget as HTMLElement;
    el.setPointerCapture(e.pointerId);
    active = true;
    let last = e.clientY;
    el.onpointermove = (ev) => {
      ondrag(toCss(ev.clientY - last));
      last = ev.clientY;
    };
    el.onpointerup = el.onpointercancel = () => {
      el.onpointermove = el.onpointerup = el.onpointercancel = null;
      active = false;
      onend?.();
    };
  }
</script>

<div class="splitter" class:active role="separator" aria-orientation="horizontal" title="Drag to resize" {onpointerdown}>
  <span class="grip"></span>
</div>

<style>
  .splitter {
    flex: none;
    /* 6px grab area matching the layout gap, 4px visible bar */
    height: 6px;
    padding: 1px 0;
    background-clip: content-box;
    cursor: row-resize;
    touch-action: none;
    display: flex;
    justify-content: center;
    align-items: center;
  }
  /* Grip: two short separator_horizontal ridges, always visible. */
  .grip {
    width: 32px; /* one separator_horizontal tile, no seam */
    height: 4px;
    background:
      url("../../assets/ui/separator_horizontal.png") 0 0 / auto 2px repeat-x,
      url("../../assets/ui/separator_horizontal.png") 0 2px / auto 2px repeat-x;
    image-rendering: pixelated;
    pointer-events: none;
  }
  .splitter:hover .grip,
  .splitter.active .grip {
    filter: brightness(1.6);
  }
  .splitter:hover,
  .splitter.active {
    background: #ffffff44;
  }
</style>
