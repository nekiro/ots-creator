<script lang="ts">
  import { Format, SpriteService, type OptimizeResult } from "../../lib/api";
  import { formatSize } from "../../lib/util/format";
  import { app, run } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import ProgressBar from "../../lib/ui/ProgressBar.svelte";

  let duplicates = $state(true);
  let empty = $state(true);
  let unused = $state(true);
  // The dialog shows the options, then a loader while the backend works, then the result.
  let stage = $state<"options" | "running" | "done">("options");
  let result = $state<OptimizeResult | null>(null);

  const percent = $derived(app.progress && app.progress.total > 0 ? (app.progress.done / app.progress.total) * 100 : 0);

  async function optimize() {
    stage = "running";
    const optimized = await run("Optimizing sprites", () => SpriteService.Optimize({ duplicates, empty, unused }));
    if (optimized) {
      result = optimized;
      app.selectedSprites = [];
      stage = "done";
    } else {
      stage = "options";
    }
  }

  function close() {
    if (stage !== "running") app.dialog = null;
  }
</script>

<Dialog title={stage === "done" ? "Sprites Optimized" : "Optimize Sprites"} width={420} onclose={close}>
  {#if stage === "options"}
    <div class="col">
      <p>Removes sprites and renumbers the rest without gaps. Objects are updated to the new ids. Undo reverts it.</p>
      <label class="row"><input class="t-check" type="checkbox" bind:checked={duplicates} />Merge duplicate sprites</label>
      <label class="row"><input class="t-check" type="checkbox" bind:checked={empty} />Remove empty sprites</label>
      <label class="row"><input class="t-check" type="checkbox" bind:checked={unused} />Remove sprites no object uses</label>
    </div>
  {:else if stage === "running"}
    <div class="col loading" role="status">
      <div class="row"><span class="t-spinner"></span>Optimizing sprites…</div>
      {#if app.progress}<ProgressBar {percent} label="{Math.floor(percent)}%" />{/if}
    </div>
  {:else if result}
    <div class="result t-panel">
      <span>Sprites</span><strong>{result.before.toLocaleString()} → {result.after.toLocaleString()}</strong>
      <span>Duplicates merged</span><strong>{result.duplicates.toLocaleString()}</strong>
      <span>Empty removed</span><strong>{result.empty.toLocaleString()}</strong>
      <span>Unused removed</span><strong>{result.unused.toLocaleString()}</strong>
      <span>Objects updated</span><strong>{result.things.toLocaleString()}</strong>
      {#if app.project?.info.format === Format.FormatAssets}
        <span class="note t-label">Sheets with no used sprite are removed from the folder when you compile.</span>
      {:else}
        <span>Size saved</span><strong>{formatSize(result.bytes)}</strong>
      {/if}
    </div>
  {/if}
  {#snippet footer()}
    <span class="grow"></span>
    {#if stage === "done"}
      <button class="t-btn primary" onclick={close}>Close</button>
    {:else}
      <button class="t-btn primary" disabled={(!duplicates && !empty && !unused) || !!app.busy} onclick={optimize}>Optimize</button>
      <button class="t-btn" disabled={stage === "running"} onclick={close}>Cancel</button>
    {/if}
  {/snippet}
</Dialog>

<style>
  p {
    margin: 0;
  }
  .loading {
    padding: 12px 0;
    color: var(--text-bright);
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
  .note {
    grid-column: 1 / -1;
  }
</style>
