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

  function set(v: number) {
    v = clamp(Math.round(v), min, max);
    if (v !== value) value = v;
  }

  let trackEl: HTMLDivElement;
  function drag(e: PointerEvent) {
    if (disabled || e.button !== 0) return;
    e.preventDefault();
    const el = e.currentTarget as HTMLElement;
    el.setPointerCapture(e.pointerId);
    const move = (ev: PointerEvent) => set(valueAt(toCss(ev.clientX - trackEl.getBoundingClientRect().left), track, min, max));
    move(e);
    el.onpointermove = move;
    el.onpointerup = el.onpointercancel = () => (el.onpointermove = el.onpointerup = el.onpointercancel = null);
  }

  function onwheel(e: WheelEvent) {
    if (disabled) return;
    e.preventDefault();
    set(value + (e.deltaY < 0 ? 1 : -1));
  }

  function onkeydown(e: KeyboardEvent) {
    const d: Record<string, number> = { ArrowLeft: -1, ArrowDown: -1, ArrowRight: 1, ArrowUp: 1 };
    if (e.key in d) set(value + d[e.key]);
    else if (e.key === "Home") set(min);
    else if (e.key === "End") set(max);
    else return;
    e.preventDefault();
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
    background: url("../../assets/ui/sb-left.png");
    cursor: pointer;
  }
  .dec:active:not(:disabled) {
    background-image: url("../../assets/ui/sb-left-pressed.png");
  }
  .inc {
    background-image: url("../../assets/ui/sb-right.png");
  }
  .inc:active:not(:disabled) {
    background-image: url("../../assets/ui/sb-right-pressed.png");
  }
  .dec:disabled,
  .inc:disabled {
    cursor: default;
  }
  /* HorizontalScrollBar: scrollbar.png clip 0 38 24 12, image-border 1 */
  .track {
    position: relative;
    flex: 1;
    border-style: solid;
    border-width: 1px;
    border-image: url("../../assets/ui/sb-track-h.png") 1 fill / 1px repeat;
    margin: 0;
    cursor: pointer;
  }
  /* HorizontalScrollBarSlider: clip 12 26 12 12, borders 3 6 3 5 */
  .thumb {
    position: absolute;
    top: -1px;
    height: 12px;
    margin-left: -1px;
    border-style: solid;
    border-width: 3px 6px 3px 5px;
    border-image: url("../../assets/ui/sb-thumb-h.png") 3 6 3 5 fill / 3px 6px 3px 5px repeat;
    box-sizing: border-box;
  }
  .disabled {
    opacity: 0.5;
  }
  .disabled .track {
    cursor: default;
  }
</style>
