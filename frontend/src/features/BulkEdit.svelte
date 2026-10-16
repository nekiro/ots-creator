<script lang="ts">
  // Edits flags of several selected objects at once. Every flag starts as
  // "keep"; clicking cycles keep → on → off.
  import { CATEGORY_NAMES, ThingService, type PropsPatch } from "../lib/api";
  import { buildPatch, FLAG_GROUPS, flagMatches, flagVisible, type Field } from "../lib/flags";
  import { app, run, toast } from "../lib/state/app.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import { plural } from "../lib/util/format";
  import FlagFields from "./FlagFields.svelte";

  let states = $state<Record<string, boolean | undefined>>({});
  let values = $state<Record<string, number | string>>({});
  let filter = $state("");

  const changes = $derived(Object.values(states).filter((value) => value !== undefined).length);
  const count = $derived(app.selection.length);

  // A new selection or category starts clean.
  $effect(() => {
    void app.category;
    void count;
    states = {};
  });

  function cycle(key: string) {
    const current = states[key];
    states[key] = current === undefined ? true : current ? false : undefined;
  }

  function options(field: Field) {
    return (field.options ?? []).map(([value, label]) => ({ value, label }));
  }

  // Fields start at 0 (or empty text) until they are set.
  function fieldValue(field: Field) {
    if (field.kind === "text") return values[field.key] ?? "";
    if (field.kind === "select") return values[field.key] ?? 0;
    return Number(values[field.key] ?? 0);
  }

  async function apply() {
    const patch = buildPatch(states, values) as PropsPatch;
    const ids = [...app.selection];
    const updated = await run("Applying", () => ThingService.Patch(app.category, ids, patch));
    if (updated === undefined) return;
    toast(`Updated ${updated} of ${ids.length} ${CATEGORY_NAMES[app.category]}(s).`, "success");
    states = {};
  }
</script>

<MiniWindow title="Edit {count} {CATEGORY_NAMES[app.category]}s" class="props">
  {#snippet actions()}
    {#if changes}<span class="dirty">{plural(changes, "change")}</span>{/if}
  {/snippet}
  <div class="scroll">
    <p class="t-label hint">Click a flag to set it on, again to set it off, a third time to keep each object's value.</p>
    <input class="t-input filter" placeholder="Filter flags…" bind:value={filter} />
    {#each FLAG_GROUPS as flagGroup}
      {@const flags = flagGroup.flags.filter((flag) => flag.key !== "hasBones" && flagVisible(flag.key, app.category, false) && flagMatches(filter, flag.label, flag.key))}
      {#if flags.length}
        <div class="group">
          <div class="ghead">{flagGroup.title}</div>
          {#each flags as flag}
            {@const setTo = states[flag.key]}
            {@const supported = app.supported.has(flag.key)}
            <div class="flag" class:unsupported={!supported}>
              <button class="row tri" onclick={() => cycle(flag.key)} title={supported ? (flag.hint ?? "") : `Not stored by ${app.project?.info.formatLabel} clients`}>
                <span class="t-check box" class:on={setTo === true} class:keep={setTo === undefined}></span>
                <span class:set={setTo !== undefined}>{flag.label}</span>
                <span class="grow"></span>
                {#if setTo === true}<span class="tag on">set on</span>{:else if setTo === false}<span class="tag off">set off</span>{/if}
              </button>
              {#if setTo && flag.fields}
                <FlagFields fields={flag.fields} value={fieldValue} onset={(field, value) => (values[field.key] = value)} {options} />
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    {/each}
  </div>
  <div class="t-sep"></div>
  <div class="row apply">
    <button class="t-btn" disabled={!changes} onclick={() => (states = {})}>Reset</button>
    <button class="t-btn primary" disabled={!changes || !!app.busy} onclick={apply}>Apply to {count}</button>
  </div>
</MiniWindow>

<style>
  .dirty {
    color: var(--gold);
    font-weight: normal;
  }
  .scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 2px 6px 6px 4px;
  }
  .hint {
    margin: 2px 0 6px;
    font: var(--fs-small) / 10px var(--font-small);
  }
  .filter {
    width: 100%;
    margin-bottom: 6px;
  }
  .ghead {
    font-weight: bold;
    padding: 4px 0 2px;
  }
  .flag {
    padding: 1px 0 1px 10px;
  }
  .flag.unsupported {
    opacity: 0.45;
  }
  .tri {
    width: 100%;
    background: none;
    border: 0;
    padding: 1px 0;
    font: inherit;
    color: var(--text);
    cursor: pointer;
    text-align: left;
  }
  .box {
    display: inline-block;
  }
  .box.on {
    background-image: url("../assets/ui/checkbox-checked.png");
  }
  .box.keep {
    opacity: 0.4;
  }
  .set {
    color: var(--text-bright);
  }
  .tag {
    font: var(--fs-small) / 10px var(--font-small);
  }
  .tag.on {
    color: var(--green);
  }
  .tag.off {
    color: #ff9c9c;
  }

  .apply {
    justify-content: flex-end;
    padding: 0 4px 2px;
  }

</style>
