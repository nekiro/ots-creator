<script lang="ts">
  import { closeContextMenu, contextMenu, type MenuItem } from "../menu.svelte";
  import MenuEntries from "./MenuEntries.svelte";
  import { toCss } from "../pixelscale";

  let menu = $state<HTMLDivElement>();
  let pos = $state({ left: 0, top: 0 });

  $effect(() => {
    if (!contextMenu.items || !menu) return;
    const bounds = menu.getBoundingClientRect();
    pos = {
      left: Math.max(0, Math.min(contextMenu.x, toCss(window.innerWidth - bounds.width) - 2)),
      top: Math.max(0, Math.min(contextMenu.y, toCss(window.innerHeight - bounds.height) - 2)),
    };
  });

  function pick(item: MenuItem) {
    closeContextMenu();
    if (!item.disabled?.()) item.action();
  }

  function onmousedown(event: MouseEvent) {
    if (contextMenu.items && !menu?.contains(event.target as Node)) closeContextMenu();
  }
</script>

<svelte:window
  {onmousedown}
  onkeydown={(event) => event.key === "Escape" && closeContextMenu()}
  onblur={closeContextMenu}
  onresize={closeContextMenu}
/>

{#if contextMenu.items}
  <div bind:this={menu} class="t-menu ctx" role="menu" tabindex="-1" style="left:{pos.left}px;top:{pos.top}px" oncontextmenu={(event) => event.preventDefault()}>
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
