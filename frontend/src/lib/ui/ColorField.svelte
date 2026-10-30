<script lang="ts">
  // An 8-bit client color (light, minimap): the number, a swatch of it and,
  // on a click of the swatch, the 6x6x6 palette to pick from. Rows are the
  // red levels; each row holds the six green levels of six blue levels.
  import { tick } from "svelte";
  import { minimapCss } from "../map/data/items";
  import { toCss } from "../pixelscale";
  import NumberField from "./NumberField.svelte";
  import { portal } from "./portal";

  let {
    value,
    onchange,
    disabled = false,
  }: {
    value: number;
    onchange: (next: number) => void;
    disabled?: boolean;
  } = $props();

  const COLORS = Array.from({ length: 216 }, (_, color) => color);
  let open = $state(false);
  let swatch: HTMLButtonElement;
  let palette = $state<HTMLDivElement>();
  let position = $state({ left: 0, top: 0 });
  let hovered = $state(-1);

  async function show() {
    if (disabled) return;
    const bounds = swatch.getBoundingClientRect();
    position = { left: toCss(bounds.left), top: toCss(bounds.bottom) + 2 };
    hovered = -1;
    open = true;
    await tick();
    if (!palette) return;
    const right = toCss(window.innerWidth) - 4;
    const bottom = toCss(window.innerHeight) - 4;
    if (position.left + palette.offsetWidth > right) position.left = Math.max(4, right - palette.offsetWidth);
    if (position.top + palette.offsetHeight > bottom) position.top = Math.max(4, toCss(bounds.top) - palette.offsetHeight - 2);
  }

  function pick(color: number) {
    open = false;
    if (color !== value) onchange(color);
  }
</script>

<svelte:window
  onmousedown={(event) => open && !palette?.contains(event.target as Node) && !swatch.contains(event.target as Node) && (open = false)}
  onkeydown={(event) => {
    if (open && event.key === "Escape") {
      event.stopPropagation();
      open = false;
    }
  }}
  onresize={() => (open = false)}
  onblur={() => (open = false)}
/>

<span class="color-field">
  <NumberField {value} max={255} {disabled} {onchange} />
  <button
    bind:this={swatch}
    type="button"
    class="swatch"
    style="background:{minimapCss(value)}"
    title="Pick a color"
    aria-label="Pick a color"
    aria-expanded={open}
    {disabled}
    onclick={() => (open ? (open = false) : show())}
  ></button>
</span>

{#if open}
  <div bind:this={palette} use:portal class="palette t-popup" style="left:{position.left}px;top:{position.top}px">
    <div class="grid" role="listbox" tabindex="-1" aria-label="Colors" onmouseleave={() => (hovered = -1)}>
      {#each COLORS as color}
        <button
          type="button"
          class="cell"
          class:sel={color === value}
          style="background:{minimapCss(color)}"
          role="option"
          aria-selected={color === value}
          aria-label={String(color)}
          onmouseenter={() => (hovered = color)}
          onclick={() => pick(color)}
        ></button>
      {/each}
    </div>
    <div class="info">
      {#if hovered >= 0}
        <span class="preview" style="background:{minimapCss(hovered)}"></span>{hovered}
      {:else}
        <span class="preview" style="background:{minimapCss(value)}"></span>{value}
      {/if}
    </div>
  </div>
{/if}

<style>
  .color-field {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .swatch {
    width: 20px;
    height: 20px;
    padding: 0;
    border: 1px solid #000;
    box-shadow: inset 0 0 0 1px #ffffff40;
    cursor: pointer;
  }
  .swatch:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .palette {
    position: fixed;
    z-index: 200;
    padding: 4px;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(36, 10px);
    grid-auto-rows: 10px;
    gap: 1px;
  }
  .cell {
    padding: 0;
    border: 0;
    cursor: pointer;
  }
  .cell:hover,
  .cell.sel {
    outline: 1px solid #f4f4f4;
    z-index: 1;
  }
  .info {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
    height: 16px;
    color: #c0c0c0;
  }
  .preview {
    width: 24px;
    height: 14px;
    border: 1px solid #000;
  }
</style>
