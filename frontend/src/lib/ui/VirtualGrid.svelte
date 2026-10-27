<script lang="ts" generics="T">
  import type { Snippet } from "svelte";

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
    scrollTo?: number | null;
    onkeydown?: (event: KeyboardEvent, columns: number) => void;
  } = $props();

  let scroller: HTMLDivElement;
  let width = $state(0);
  let height = $state(0);
  let scrollTop = $state(0);

  const columns = $derived(Math.max(1, Math.floor((width + gap) / (cellWidth + gap))));
  // Spread the columns over the full width so no empty strip is left on the right.
  const step = $derived(columns > 1 ? Math.max(cellWidth + gap, (width - cellWidth) / (columns - 1)) : 0);
  const rows = $derived(Math.ceil(count / columns));
  const rowHeight = $derived(cellHeight + gap);
  const first = $derived(Math.max(0, Math.floor(scrollTop / rowHeight) - overscan));
  const last = $derived(Math.min(rows - 1, Math.ceil((scrollTop + height) / rowHeight) + overscan));
  const visible = $derived.by(() => {
    const out: number[] = [];
    for (let row = first; row <= last; row++) {
      for (let column = 0; column < columns; column++) {
        const i = row * columns + column;
        if (i < count) out.push(i);
      }
    }
    return out;
  });

  $effect(() => {
    if (scrollTo == null || !scroller) return;
    const row = Math.floor(scrollTo / columns);
    const top = row * rowHeight;
    if (top < scroller.scrollTop) scroller.scrollTop = top;
    else if (top + rowHeight > scroller.scrollTop + scroller.clientHeight) scroller.scrollTop = top + rowHeight - scroller.clientHeight;
  });

  const DRAG_EDGE = 40;

  // Dragging near an edge scrolls, faster closer to it, so a drag can reach
  // cells outside the view.
  function ondragover(event: DragEvent) {
    const bounds = scroller.getBoundingClientRect();
    const fromTop = event.clientY - bounds.top;
    const fromBottom = bounds.bottom - event.clientY;
    if (fromTop < DRAG_EDGE) scroller.scrollTop -= Math.ceil((DRAG_EDGE - fromTop) / 2);
    else if (fromBottom < DRAG_EDGE) scroller.scrollTop += Math.ceil((DRAG_EDGE - fromBottom) / 2);
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="vgrid"
  bind:this={scroller}
  bind:clientWidth={width}
  bind:clientHeight={height}
  onscroll={() => (scrollTop = scroller.scrollTop)}
  {ondragover}
  tabindex="0"
  onkeydown={(event) => onkeydown?.(event, columns)}
>
  <div class="spacer" style="height:{rows * rowHeight}px">
    {#each visible as i (i)}
      <div class="cell" style="transform:translate({Math.round((i % columns) * step)}px,{Math.floor(i / columns) * rowHeight}px);width:{cellWidth}px;height:{cellHeight}px">
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
