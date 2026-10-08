<script lang="ts">
  import { closeContextMenu, contextMenu, type MenuItem } from "../menu.svelte";
  import MenuEntries from "./MenuEntries.svelte";
  import { toCss } from "../pixelscale";

  let el = $state<HTMLDivElement>();
  let pos = $state({ left: 0, top: 0 });

  // Keep the menu inside the window.
  $effect(() => {
    if (!contextMenu.items || !el) return;
    const r = el.getBoundingClientRect();
    pos = {
      left: Math.max(0, Math.min(contextMenu.x, toCss(window.innerWidth - r.width) - 2)),
      top: Math.max(0, Math.min(contextMenu.y, toCss(window.innerHeight - r.height) - 2)),
    };
  });

  function pick(item: MenuItem) {
    closeContextMenu();
    if (!item.disabled?.()) item.action();
  }

  function onmousedown(e: MouseEvent) {
    if (contextMenu.items && !el?.contains(e.target as Node)) closeContextMenu();
  }
</script>

<svelte:window
  {onmousedown}
  onkeydown={(e) => e.key === "Escape" && closeContextMenu()}
  onblur={closeContextMenu}
  onresize={closeContextMenu}
/>

{#if contextMenu.items}
  <div bind:this={el} class="t-menu ctx" role="menu" tabindex="-1" style="left:{pos.left}px;top:{pos.top}px" oncontextmenu={(e) => e.preventDefault()}>
    <MenuEntries items={contextMenu.items} onpick={pick} />
  </div>
{/if}

<style>
  .ctx {
    position: fixed;
    z-index: 300;
    min-width: 180px;
    outline: none;
  }
</style>
