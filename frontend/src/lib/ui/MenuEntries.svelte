<script lang="ts">
  // Entries of an OTClient PopupMenu (menu bar drop-downs and context menus).
  import type { MenuEntry, MenuItem } from "../menu.svelte";

  let { items, onpick }: { items: MenuEntry[]; onpick: (item: MenuItem) => void } = $props();
  const checkable = $derived(items.some((it) => it !== "-" && it.checked));
</script>

{#each items as it}
  {#if it === "-"}
    <div class="t-menu-sep"></div>
  {:else}
    <button class="t-menu-entry" role="menuitem" disabled={it.disabled?.()} onclick={() => onpick(it)}>
      <span>{#if checkable}<span class="mark">{it.checked?.() ? "✓" : ""}</span>{/if}{it.label}</span>{#if it.keys}<kbd>{it.keys}</kbd>{/if}
    </button>
  {/if}
{/each}

<style>
  .mark {
    display: inline-block;
    width: 14px;
  }
</style>
