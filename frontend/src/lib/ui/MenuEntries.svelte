<script lang="ts">
  // Entries of an OTClient PopupMenu (menu bar drop-downs and context menus).
  import type { MenuEntry, MenuItem } from "../menu.svelte";

  let { items, onpick }: { items: MenuEntry[]; onpick: (item: MenuItem) => void } = $props();
  const checkable = $derived(items.some((entry) => entry !== "-" && entry.checked));
</script>

{#each items as entry}
  {#if entry === "-"}
    <div class="t-menu-sep"></div>
  {:else}
    <button class="t-menu-entry" role="menuitem" disabled={entry.disabled?.()} onclick={() => onpick(entry)}>
      <span>{#if checkable}<span class="mark">{entry.checked?.() ? "✓" : ""}</span>{/if}{entry.label}</span>{#if entry.keys}<kbd>{entry.keys}</kbd>{/if}
    </button>
  {/if}
{/each}

<style>
  .mark {
    display: inline-block;
    width: 14px;
  }
</style>
