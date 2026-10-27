<script lang="ts">
  import { MapService, type Suggestion } from "../../lib/map/data/api";
  import { brushContext, notePick, rerank, selectionOf, suggestionKey, suggestionTitle } from "../../lib/map/editor/suggestions";
  import { map, selectBrush } from "../../lib/map/state.svelte";
  import { readStored, writeStored } from "../../lib/state/storage";
  import ItemIcon from "./ItemIcon.svelte";

  // What fits the tile the cursor rests on, without a key: the "Here"
  // suggestions of the popup (W), refreshed when the cursor stops.
  const REST_MS = 300;
  const SHOWN = 8;
  const HIDDEN_KEY = "map:suggest-strip-hidden";

  let hidden = $state(readStored(HIDDEN_KEY) === "1");
  let items = $state.raw<Suggestion[]>([]);

  $effect(() => {
    const id = map.activeId;
    const at = map.hover;
    if (hidden || id === null || !at) return;
    const brush = brushContext();
    let stale = false;
    const timer = window.setTimeout(async () => {
      try {
        const result = await MapService.Suggest(id, at, brush);
        if (!stale) items = rerank(result.here ?? []).slice(0, SHOWN);
      } catch {
        // The popup shows errors; the strip keeps the last suggestions.
      }
    }, REST_MS);
    return () => {
      stale = true;
      clearTimeout(timer);
    };
  });

  function toggle() {
    hidden = !hidden;
    writeStored(HIDDEN_KEY, hidden ? "1" : "0");
  }

  function pick(suggestion: Suggestion) {
    notePick(items, suggestion);
    selectBrush(selectionOf(suggestion));
  }
</script>

<div class="strip">
  <button class="head" title={hidden ? "Show what fits the tile under the cursor" : "Hide the suggestions"} onclick={toggle}>
    <span>Fits here</span><span class="dim">{hidden ? "show" : "hide"}</span>
  </button>
  {#if !hidden}
    <div class="cells">
      {#each items as suggestion (suggestionKey(suggestion))}
        <button class="m-cell small" title={suggestionTitle(suggestion)} onclick={() => pick(suggestion)}>
          <span class="t-slot"><ItemIcon id={suggestion.item} /></span>
        </button>
      {:else}
        <span class="dim empty">Rest the cursor on the map</span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .strip {
    flex: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .head {
    display: flex;
    justify-content: space-between;
    padding: 0 4px;
    border: 0;
    background: none;
    color: var(--text-dim);
    font: var(--fs-small) / 12px var(--font-small);
    text-transform: uppercase;
    cursor: pointer;
  }
  .head:hover {
    color: var(--text);
  }
  .cells {
    display: flex;
    gap: 1px;
    min-height: 34px;
    overflow: hidden;
  }
  .cells .small {
    width: 30px;
    height: 34px;
  }
  .dim {
    color: var(--text-dim);
  }
  .empty {
    padding: 0 4px;
    font: var(--fs-small) / 34px var(--font-small);
  }
</style>
