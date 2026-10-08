<script lang="ts">
  import { CATEGORIES, CATEGORY_LABELS, ThingService } from "../../lib/api";
  import { app, run, toast } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";

  const DEFAULTS: Record<number, number> = { 1: 500, 2: 300, 3: 100, 4: 75 };

  let cats = $state<Record<number, boolean>>({ [app.category]: true });
  let min = $state(DEFAULTS[app.category] ?? 100);
  let max = $state(DEFAULTS[app.category] ?? 100);

  const chosen = $derived(CATEGORIES.filter((c) => cats[c]));

  async function apply() {
    const n = await run("Setting durations", () => ThingService.SetDurations(chosen, min, Math.max(min, max)));
    if (n === undefined) return;
    toast(`Updated ${n} animated object(s).`, "success");
    app.dialog = null;
  }
</script>

<Dialog title="Frame Durations" width={400} onclose={() => (app.dialog = null)}>
  <div class="col">
    <p>Sets the duration of every frame of every animated object in the chosen categories.</p>
    <div class="row cats">
      {#each CATEGORIES as c}
        <label class="row"><input class="t-check" type="checkbox" bind:checked={cats[c]} />{CATEGORY_LABELS[c]}</label>
      {/each}
    </div>
    <div class="row">
      <NumberField label="Minimum" value={min} min={0} max={1000000} width={60} onchange={(v) => ((min = v), (max = Math.max(max, v)))} />
      <NumberField label="Maximum" value={max} min={min} max={1000000} width={60} onchange={(v) => (max = v)} />
      <span class="t-label">ms</span>
    </div>
    <div class="row">
      {#each CATEGORIES as c}
        <button class="t-btn" onclick={() => (min = max = DEFAULTS[c])} title="Default for {CATEGORY_LABELS[c].toLowerCase()}">{CATEGORY_LABELS[c]} {DEFAULTS[c]}</button>
      {/each}
    </div>
    {#if !app.project?.info.features.improvedAnimations}
      <p class="note">This client has no per-frame durations; values are used for preview and OBD only.</p>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!chosen.length || !!app.busy} onclick={apply}>Apply</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  p {
    margin: 0;
  }
  .cats {
    gap: 12px;
  }
  .note {
    color: var(--gold);
  }
</style>
