<script lang="ts">
  // Edits flags of several selected objects at once. Every flag starts as
  // "keep"; clicking cycles keep → on → off.
  import { CATEGORY_NAMES, ThingService, type PropsPatch } from "../lib/api";
  import { buildPatch, FLAG_GROUPS, flagVisible, type Field } from "../lib/flags";
  import { app, run, toast } from "../lib/state.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import NumberField from "../lib/ui/NumberField.svelte";
  import Select from "../lib/ui/Select.svelte";

  let states = $state<Record<string, boolean | undefined>>({});
  let values = $state<Record<string, number | string>>({});
  let filter = $state("");

  const changes = $derived(Object.values(states).filter((v) => v !== undefined).length);
  const count = $derived(app.selection.length);

  // A new selection or category starts clean.
  $effect(() => {
    void app.category;
    void count;
    states = {};
  });

  function cycle(key: string) {
    const cur = states[key];
    states[key] = cur === undefined ? true : cur ? false : undefined;
  }

  function matches(label: string, key: string) {
    const q = filter.trim().toLowerCase();
    return !q || label.toLowerCase().includes(q) || key.toLowerCase().includes(q);
  }

  function options(fd: Field) {
    return (fd.options ?? []).map(([value, label]) => ({ value, label }));
  }

  async function apply() {
    const patch = buildPatch(states, values) as PropsPatch;
    const ids = [...app.selection];
    const n = await run("Applying", () => ThingService.Patch(app.category, ids, patch));
    if (n === undefined) return;
    toast(`Updated ${n} of ${ids.length} ${CATEGORY_NAMES[app.category]}(s).`, "success");
    states = {};
  }
</script>

<MiniWindow title="Edit {count} {CATEGORY_NAMES[app.category]}s" class="props">
  {#snippet actions()}
    {#if changes}<span class="dirty">{changes} change{changes > 1 ? "s" : ""}</span>{/if}
  {/snippet}
  <div class="scroll">
    <p class="t-label hint">Click a flag to set it on, again to set it off, a third time to keep each object's value.</p>
    <input class="t-input filter" placeholder="Filter flags…" bind:value={filter} />
    {#each FLAG_GROUPS as grp}
      {@const flags = grp.flags.filter((f) => f.key !== "hasBones" && flagVisible(f.key, app.category, false) && matches(f.label, f.key))}
      {#if flags.length}
        <div class="group">
          <div class="ghead">{grp.title}</div>
          {#each flags as f}
            {@const st = states[f.key]}
            {@const supported = app.supported.has(f.key)}
            <div class="flag" class:unsupported={!supported}>
              <button class="row tri" onclick={() => cycle(f.key)} title={supported ? (f.hint ?? "") : `Not stored by ${app.project?.info.formatLabel} clients`}>
                <span class="t-check box" class:on={st === true} class:keep={st === undefined}></span>
                <span class:set={st !== undefined}>{f.label}</span>
                <span class="grow"></span>
                {#if st === true}<span class="tag on">set on</span>{:else if st === false}<span class="tag off">set off</span>{/if}
              </button>
              {#if st && f.fields}
                <div class="fields">
                  {#each f.fields as fd}
                    <label class="row">
                      <span class="t-label">{fd.label}</span>
                      {#if fd.kind === "select"}
                        <Select grow value={values[fd.key] ?? 0} options={options(fd)} onchange={(v) => (values[fd.key] = v)} />
                      {:else if fd.kind === "text"}
                        <input class="t-input grow" value={values[fd.key] ?? ""} onchange={(e) => (values[fd.key] = e.currentTarget.value)} />
                      {:else}
                        <NumberField value={Number(values[fd.key] ?? 0)} min={fd.kind === "i16" ? -32768 : 0} max={fd.kind === "i16" ? 32767 : 65535} onchange={(v) => (values[fd.key] = v)} />
                      {/if}
                    </label>
                  {/each}
                </div>
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
    background-image: url("../assets/ui/check-on.png");
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
  .fields {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin: 3px 0 4px 18px;
    padding-left: 6px;
    border-left: 1px solid #4a4a4a;
  }
  .fields .t-label {
    min-width: 54px;
  }
  .apply {
    justify-content: flex-end;
    padding: 0 4px 2px;
  }
</style>
