<script lang="ts">
  import { CATEGORIES, CATEGORY_LABELS, ThingService } from "../../lib/api";
  import { app, run, toast } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";

  const DEFAULTS: Record<number, number> = { 1: 500, 2: 300, 3: 100, 4: 75 };

  let checked = $state<Record<number, boolean>>({ [app.category]: true });
  let min = $state(DEFAULTS[app.category] ?? 100);
  let max = $state(DEFAULTS[app.category] ?? 100);

  const chosen = $derived(CATEGORIES.filter((category) => checked[category]));

  async function apply() {
    const updated = await run("Setting durations", () => ThingService.SetDurations(chosen, min, Math.max(min, max)));
    if (updated === undefined) return;
    toast(`Updated ${updated} animated object(s).`, "success");
    app.dialog = null;
  }
</script>

<Dialog title="Frame Durations" width={400} onclose={() => (app.dialog = null)}>
  <div class="col">
    <p>Sets the duration of every frame of every animated object in the chosen categories.</p>
    <div class="row cats">
      {#each CATEGORIES as category}
        <label class="row"><input class="t-check" type="checkbox" bind:checked={checked[category]} />{CATEGORY_LABELS[category]}</label>
      {/each}
    </div>
    <div class="row">
      <NumberField label="Minimum" value={min} min={0} max={1000000} width={60} onchange={(value) => ((min = value), (max = Math.max(max, value)))} />
      <NumberField label="Maximum" value={max} min={min} max={1000000} width={60} onchange={(value) => (max = value)} />
      <span class="t-label">ms</span>
    </div>
    <div class="row">
      {#each CATEGORIES as category}
        <button class="t-btn" onclick={() => (min = max = DEFAULTS[category])} title="Default for {CATEGORY_LABELS[category].toLowerCase()}">{CATEGORY_LABELS[category]} {DEFAULTS[category]}</button>
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
