<script lang="ts" generics="T">
  import type { Snippet } from "svelte";

  // Renders only the visible cells of a large uniform grid.
  let {
    count,
    cellWidth,
    cellHeight,
    gap = 2,
    overscan = 2,
    cell,
    scrollTo = null,
    onkeydown,
  }: {
    count: number;
    cellWidth: number;
    cellHeight: number;
    gap?: number;
    overscan?: number;
    cell: Snippet<[number]>;
    /** Index to keep visible; changes scroll the grid. */
    scrollTo?: number | null;
    onkeydown?: (e: KeyboardEvent, columns: number) => void;
  } = $props();

  let el: HTMLDivElement;
  let width = $state(0);
  let height = $state(0);
  let scrollTop = $state(0);

  const columns = $derived(Math.max(1, Math.floor((width + gap) / (cellWidth + gap))));
  // Spread the columns over the full width so no empty strip is left on the right.
  const step = $derived(columns > 1 ? Math.max(cellWidth + gap, (width - cellWidth) / (columns - 1)) : 0);
  const rows = $derived(Math.ceil(count / columns));
  const rowH = $derived(cellHeight + gap);
  const first = $derived(Math.max(0, Math.floor(scrollTop / rowH) - overscan));
  const last = $derived(Math.min(rows - 1, Math.ceil((scrollTop + height) / rowH) + overscan));
  const visible = $derived.by(() => {
    const out: number[] = [];
    for (let r = first; r <= last; r++) {
      for (let c = 0; c < columns; c++) {
        const i = r * columns + c;
        if (i < count) out.push(i);
      }
    }
    return out;
  });

  $effect(() => {
    if (scrollTo == null || !el) return;
    const row = Math.floor(scrollTo / columns);
    const top = row * rowH;
    if (top < el.scrollTop) el.scrollTop = top;
    else if (top + rowH > el.scrollTop + el.clientHeight) el.scrollTop = top + rowH - el.clientHeight;
  });
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="vgrid"
  bind:this={el}
  bind:clientWidth={width}
  bind:clientHeight={height}
  onscroll={() => (scrollTop = el.scrollTop)}
  tabindex="0"
  onkeydown={(e) => onkeydown?.(e, columns)}
>
  <div class="spacer" style="height:{rows * rowH}px">
    {#each visible as i (i)}
      <div class="cell" style="transform:translate({Math.round((i % columns) * step)}px,{Math.floor(i / columns) * rowH}px);width:{cellWidth}px;height:{cellHeight}px">
        {@render cell(i)}
      </div>
    {/each}
  </div>
</div>

<style>
  .vgrid {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    /* Reserve the scrollbar so the measured width (and column count) does
       not change when it appears. */
    scrollbar-gutter: stable;
    outline: none;
    contain: strict;
  }
  .spacer {
    position: relative;
  }
  .cell {
    position: absolute;
    top: 0;
    left: 0;
  }
</style>
