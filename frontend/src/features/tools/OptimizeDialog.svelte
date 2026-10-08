<script lang="ts">
  import { SpriteService, type OptimizeResult } from "../../lib/api";
  import { app, run } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";

  let duplicates = $state(true);
  let empty = $state(true);
  let unused = $state(true);
  let result = $state<OptimizeResult | null>(null);

  async function optimize() {
    const res = await run("Optimizing sprites", () => SpriteService.Optimize({ duplicates, empty, unused }));
    if (res) {
      result = res;
      app.selectedSprites = [];
    }
  }
</script>

<Dialog title="Optimize Sprites" width={420} onclose={() => (app.dialog = null)}>
  <div class="col">
    <p>Removes sprites and renumbers the rest without gaps. Objects are updated to the new ids. Undo reverts it.</p>
    <label class="row"><input class="t-check" type="checkbox" bind:checked={duplicates} />Merge duplicate sprites</label>
    <label class="row"><input class="t-check" type="checkbox" bind:checked={empty} />Remove empty sprites</label>
    <label class="row"><input class="t-check" type="checkbox" bind:checked={unused} />Remove sprites no object uses</label>
    {#if result}
      <div class="result t-panel">
        <span>Sprites</span><strong>{result.before.toLocaleString()} → {result.after.toLocaleString()}</strong>
        <span>Duplicates merged</span><strong>{result.duplicates.toLocaleString()}</strong>
        <span>Empty removed</span><strong>{result.empty.toLocaleString()}</strong>
        <span>Unused removed</span><strong>{result.unused.toLocaleString()}</strong>
        <span>Objects updated</span><strong>{result.things.toLocaleString()}</strong>
      </div>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={(!duplicates && !empty && !unused) || !!app.busy} onclick={optimize}>Optimize</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>{result ? "Close" : "Cancel"}</button>
  {/snippet}
</Dialog>

<style>
  p {
    margin: 0;
  }
  .result {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 2px 12px;
    padding: 6px 8px;
  }
  strong {
    color: var(--text-bright);
    text-align: right;
  }
</style>
