<script lang="ts">
  import { hsiToRgb, PALETTE_SIZE } from "../lib/render/outfit";

  let { label, value, onchange }: { label: string; value: number; onchange: (v: number) => void } = $props();
  let open = $state(false);

  const hex = (c: number) => "#" + hsiToRgb(c).toString(16).padStart(6, "0");
  // The client shows the palette as 7 rows of 19 colors.
  const cells = Array.from({ length: PALETTE_SIZE }, (_, i) => i);
</script>

<div class="cp">
  <button class="swatch t-frame-down" title="{label}: {value}" style="background:{hex(value)}" onclick={() => (open = !open)}></button>
  <span class="t-label">{label}</span>
  {#if open}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="pop t-frame-up" onmouseleave={() => (open = false)}>
      {#each cells as c}
        <button
          class="cell"
          class:on={c === value}
          style="background:{hex(c)}"
          title={String(c)}
          onclick={() => {
            onchange(c);
            open = false;
          }}
        ></button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .cp {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .swatch {
    width: 18px;
    height: 18px;
    padding: 0;
    cursor: pointer;
  }
  .pop {
    position: absolute;
    bottom: 22px;
    left: 0;
    z-index: 30;
    display: grid;
    grid-template-columns: repeat(19, 12px);
    gap: 1px;
    padding: 4px;
    background-color: #262626;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.6);
  }
  .cell {
    width: 12px;
    height: 12px;
    padding: 0;
    border: 1px solid #000;
    cursor: pointer;
  }
  .cell.on,
  .cell:hover {
    outline: 1px solid #fff;
    z-index: 1;
  }
</style>
