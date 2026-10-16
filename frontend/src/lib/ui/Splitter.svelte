<script lang="ts">
  // OTClient Splitter: invisible 4px handle that shows #ffffff44 while
  // hovered or dragged. Reports the pointer's movement across the handle:
  // vertical by default, horizontal for a `vertical` (column) splitter.
  import { toCss } from "../pixelscale";

  let { ondrag, onend, vertical = false }: { ondrag: (d: number) => void; onend?: () => void; vertical?: boolean } = $props();
  let active = $state(false);

  function onpointerdown(event: PointerEvent) {
    if (event.button !== 0) return;
    event.preventDefault();
    const el = event.currentTarget as HTMLElement;
    el.setPointerCapture(event.pointerId);
    active = true;
    let last = vertical ? event.clientX : event.clientY;
    el.onpointermove = (event) => {
      const p = vertical ? event.clientX : event.clientY;
      ondrag(toCss(p - last));
      last = p;
    };
    el.onpointerup = el.onpointercancel = () => {
      el.onpointermove = el.onpointerup = el.onpointercancel = null;
      active = false;
      onend?.();
    };
  }
</script>

<div class="splitter" class:active class:vertical role="separator" aria-orientation={vertical ? "vertical" : "horizontal"} title="Drag to resize" {onpointerdown}>
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
  .splitter.vertical {
    height: auto;
    width: 6px;
    padding: 0 1px;
    cursor: col-resize;
  }
  /* Grip: two short separator_horizontal ridges, always visible. */
  .grip {
    flex: none;
    width: 32px; /* one separator_horizontal tile, no seam */
    height: 4px;
    background:
      url("../../assets/ui/separator_horizontal.png") 0 0 / auto 2px repeat-x,
      url("../../assets/ui/separator_horizontal.png") 0 2px / auto 2px repeat-x;
    image-rendering: pixelated;
    pointer-events: none;
  }
  .vertical .grip {
    transform: rotate(90deg);
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
