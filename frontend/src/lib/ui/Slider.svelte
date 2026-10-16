<script lang="ts">
  // OTClient HorizontalScrollBar used as a value slider: arrow buttons,
  // scrollbar track and slider thumb. Drag, click, wheel and arrow keys.
  import { toCss } from "../pixelscale";
  import { clamp, thumbOffset, thumbWidth, valueAt } from "./slider";

  let {
    value = $bindable(),
    min = 0,
    max,
    width = 110,
    disabled = false,
    title = "",
  }: { value: number; min?: number; max: number; width?: number; disabled?: boolean; title?: string } = $props();

  const track = $derived(width - 24);
  const thumb = $derived(thumbWidth(track, min, max));
  const offset = $derived(thumbOffset(value, track, min, max));

  function set(next: number) {
    next = clamp(Math.round(next), min, max);
    if (next !== value) value = next;
  }

  let trackEl: HTMLDivElement;
  function drag(event: PointerEvent) {
    if (disabled || event.button !== 0) return;
    event.preventDefault();
    const handle = event.currentTarget as HTMLElement;
    handle.setPointerCapture(event.pointerId);
    const move = (event: PointerEvent) => set(valueAt(toCss(event.clientX - trackEl.getBoundingClientRect().left), track, min, max));
    move(event);
    handle.onpointermove = move;
    handle.onpointerup = handle.onpointercancel = () => (handle.onpointermove = handle.onpointerup = handle.onpointercancel = null);
  }

  function onwheel(event: WheelEvent) {
    if (disabled) return;
    event.preventDefault();
    set(value + (event.deltaY < 0 ? 1 : -1));
  }

  function onkeydown(event: KeyboardEvent) {
    const steps: Record<string, number> = { ArrowLeft: -1, ArrowDown: -1, ArrowRight: 1, ArrowUp: 1 };
    if (event.key in steps) set(value + steps[event.key]);
    else if (event.key === "Home") set(min);
    else if (event.key === "End") set(max);
    else return;
    event.preventDefault();
  }
</script>

<div
  class="slider"
  class:disabled
  style="width:{width}px"
  role="slider"
  tabindex={disabled ? -1 : 0}
  aria-valuemin={min}
  aria-valuemax={max}
  aria-valuenow={value}
  {title}
  {onwheel}
  {onkeydown}
>
  <button type="button" class="dec" tabindex="-1" aria-label="Decrease" disabled={disabled || value <= min} onclick={() => set(value - 1)}></button>
  <div class="track" role="presentation" bind:this={trackEl} onpointerdown={drag}>
    <div class="thumb" style="left:{offset}px;width:{thumb}px"></div>
  </div>
  <button type="button" class="inc" tabindex="-1" aria-label="Increase" disabled={disabled || value >= max} onclick={() => set(value + 1)}></button>
</div>

<style>
  .slider {
    display: inline-flex;
    height: 12px;
    flex: none;
    outline: none;
    image-rendering: pixelated;
  }
  .slider:focus-visible {
    outline: 1px dotted #c0c0c0;
    outline-offset: 1px;
  }
  .dec,
  .inc {
    width: 12px;
    height: 12px;
    padding: 0;
    border: 0;
    flex: none;
    background: url("../../assets/ui/slider-buttonleft-up.png");
    cursor: pointer;
  }
  .dec:active:not(:disabled) {
    background-image: url("../../assets/ui/slider-buttonleft-down.png");
  }
  .inc {
    background-image: url("../../assets/ui/slider-buttonright-up.png");
  }
  .inc:active:not(:disabled) {
    background-image: url("../../assets/ui/slider-buttonright-down.png");
  }
  .dec:disabled,
  .inc:disabled {
    cursor: default;
  }
  /* slider-texture: 1px bevel top and bottom, tiled along. */
  .track {
    position: relative;
    flex: 1;
    border-style: solid;
    border-width: 1px 0;
    border-image: url("../../assets/ui/slider-texture.png") 1 0 fill / 1px 0 repeat;
    margin: 0;
    cursor: pointer;
  }
  /* The horizontal scrollbar handle: 6px rounded ends, the middle stretches. */
  .thumb {
    position: absolute;
    top: -1px;
    height: 12px;
    border-style: solid;
    border-width: 0 6px;
    border-image: url("../../assets/ui/scrollbar-handle-horizontal.png") 0 6 fill / 0 6px stretch;
    box-sizing: border-box;
  }
  .disabled {
    opacity: 0.5;
  }
  .disabled .track {
    cursor: default;
  }
</style>
