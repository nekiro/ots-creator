<script lang="ts">
  // Find items on the map, walk through the results, and replace or remove
  // what was found. Stays open while you work on the map.
  import { errorMessage } from "../../lib/api";
  import { toast } from "../../lib/state.svelte";
  import { MapService } from "../../lib/map/api";
  import { goTo, mapCommands } from "../../lib/map/commands";
  import { edit, map } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import MiniWindow from "../../lib/ui/MiniWindow.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import ItemIcon from "./ItemIcon.svelte";
  import ItemPicker from "./ItemPicker.svelte";

  const KINDS = [
    { value: "", label: "Any item" },
    { value: "unique", label: "With a unique id" },
    { value: "action", label: "With an action id" },
    { value: "container", label: "Containers" },
    { value: "writeable", label: "With text" },
    { value: "teleport", label: "Teleports" },
    { value: "door", label: "Doors" },
    { value: "duplicate", label: "Same item twice on a tile" },
  ];

  let ids = $state<number[]>([]);
  let pick = $state(0);
  let name = $state("");
  let kind = $state("");
  let actionId = $state("");
  let uniqueId = $state("");
  let inSelection = $state(false);
  let containers = $state(true);
  let replaceOpen = $state(false);
  let target = $state(0);
  let busy = $state(false);
  let current = $state(-1);

  // Applies what the panel was opened with (Ctrl+F, "Find on the map"…).
  $effect(() => {
    const p = map.findPreset;
    if (!p) return;
    map.findPreset = null;
    if (p.ids?.length) {
      ids = [...p.ids];
      name = "";
    }
    inSelection = !!p.inSelection && !!map.active?.selection;
    if (p.replace) replaceOpen = true;
    if (p.ids?.length && !p.replace) void find();
  });

  const results = $derived(map.results);
  const canFind = $derived(ids.length > 0 || !!name.trim() || !!kind || !!Number(actionId) || !!Number(uniqueId));

  function addId(id: number) {
    if (id && !ids.includes(id)) ids = [...ids, id];
    pick = 0;
  }

  async function find() {
    if (map.activeId === null || !canFind) return;
    busy = true;
    try {
      const r = await MapService.Find(map.activeId, {
        ids,
        name: ids.length ? "" : name.trim(),
        kind,
        actionId: Number(actionId) || 0,
        uniqueId: Number(uniqueId) || 0,
        inSelection,
        containers,
        limit: 5000,
      });
      const title = ids.length ? (ids.length === 1 ? (map.items?.name(ids[0]) ?? "Found") : `${ids.length} items`) : name ? `“${name}”` : (KINDS.find((k) => k.value === kind)?.label ?? "Found");
      map.results = { title, more: r.more, items: (r.results ?? []).filter((x) => !!x).map((x) => ({ pos: x.pos, label: `${x.name} (${x.id})`, detail: x.detail })) };
      current = -1;
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      busy = false;
    }
  }

  function show(i: number) {
    const r = results?.items[i];
    if (!r) return;
    current = i;
    goTo(r.pos);
    mapCommands.inspect(r.pos);
  }

  async function replace(remove: boolean) {
    if (!ids.length || (!remove && !target)) return;
    const ch = await edit((id) => MapService.Replace(id, ids, ids.map(() => (remove ? 0 : target)), inSelection));
    if (!ch) return;
    toast(`${ch.count.toLocaleString()} item${ch.count === 1 ? "" : "s"} ${remove ? "removed" : "replaced"}. Ctrl+Z undoes it.`, "success");
    map.results = null;
    if (!remove) ids = [target];
  }

  function close() {
    map.findOpen = false;
    map.results = null;
  }
</script>

<MiniWindow title="Find on the map" class="find">
  {#snippet actions()}
    <button class="t-icon-btn" title="Close (Esc)" onclick={close}><Icon name="close" /></button>
  {/snippet}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div class="body" role="search" onkeydown={(event) => event.key === "Escape" && (event.stopPropagation(), close())}>
    <div class="m-form">
      <ItemPicker bind:value={pick} placeholder={ids.length ? "Add another item…" : "Item name or id…"} onchange={addId} />
      {#if ids.length}
        <div class="chips">
          {#each ids as id}
            <span class="chip" title={String(id)}><ItemIcon {id} size={18} /><span class="cn">{map.items?.name(id)}</span><button class="x" title="Remove" onclick={() => (ids = ids.filter((x) => x !== id))}><Icon name="close" size={10} /></button></span>
          {/each}
        </div>
      {:else}
        <input class="t-input" placeholder="…or any item whose name contains" bind:value={name} onkeydown={(event) => event.key === "Enter" && find()} />
      {/if}
      <details>
        <summary class="t-label">More conditions{kind || actionId || uniqueId ? " (set)" : ""}</summary>
        <div class="m-form inner">
          <div class="m-field"><span class="m-lbl">Kind</span><Select grow value={kind} options={KINDS} onchange={(value) => (kind = value)} /></div>
          <div class="m-field">
            <span class="m-lbl">Action id</span><input class="t-input n" placeholder="any" bind:value={actionId} />
            <span class="t-label">Unique</span><input class="t-input n" placeholder="any" bind:value={uniqueId} />
          </div>
          <label class="row"><input class="t-check" type="checkbox" bind:checked={containers} />Look inside containers</label>
        </div>
      </details>
      <div class="row">
        <div class="m-seg scope">
          <button class:on={!inSelection} onclick={() => (inSelection = false)}>Whole map</button>
          <button class:on={inSelection} disabled={!map.active?.selection} title={map.active?.selection ? "" : "Select an area first"} onclick={() => (inSelection = true)}>Selection</button>
        </div>
        <button class="t-btn primary" disabled={busy || !canFind} onclick={find}>Find</button>
      </div>
    </div>

    {#if results}
      <div class="m-sect">
        {results.items.length.toLocaleString()}{results.more ? "+" : ""} found
        <span class="grow"></span>
        {#if results.items.length}
          <button class="t-icon-btn" title="Previous" onclick={() => show((current - 1 + results.items.length) % results.items.length)}><Icon name="arrowN" /></button>
          <button class="t-icon-btn" title="Next" onclick={() => show((current + 1) % results.items.length)}><Icon name="arrowS" /></button>
          <button class="t-icon-btn" title="Select all found tiles" onclick={() => edit((id) => MapService.SelectTiles(id, results.items.map((r) => r.pos), false))}><Icon name="check" /></button>
        {/if}
      </div>
      <div class="results t-panel">
        {#each results.items as r, i (i)}
          <button class="m-row res" class:on={i === current} onclick={() => show(i)}>
            <span class="rp">{r.pos.x}, {r.pos.y}, {r.pos.z}</span>
            <span class="m-name">{r.label}</span>
            {#if r.detail}<span class="m-meta">{r.detail}</span>{/if}
          </button>
        {:else}
          <div class="m-empty">Nothing found.</div>
        {/each}
      </div>
    {/if}

    <details class="replace" bind:open={replaceOpen}>
      <summary class="t-label">Replace or remove</summary>
      {#if ids.length}
        <div class="m-form inner">
          <ItemPicker bind:value={target} placeholder="Replace with…" />
          <div class="row">
            <button class="t-btn primary" disabled={!target} onclick={() => replace(false)}>Replace {ids.length > 1 ? "all" : ""}</button>
            <button class="t-btn danger" onclick={() => replace(true)}>Remove them</button>
            <span class="t-label small">{inSelection ? "in the selection" : "on the whole map"}</span>
          </div>
          <span class="t-label small">Attributes and container contents are kept.</span>
        </div>
      {:else}
        <p class="t-label small">Pick the items to replace above.</p>
      {/if}
    </details>
  </div>
</MiniWindow>

<style>
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 2px;
    overflow-y: auto;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 0 3px;
    border: 1px solid #555;
    background: rgba(0, 0, 0, 0.2);
    max-width: 100%;
  }
  .cn {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .x {
    background: none;
    border: 0;
    color: var(--text-dim);
    cursor: pointer;
    padding: 0;
    display: flex;
  }
  details summary {
    cursor: pointer;
    padding: 2px 0;
  }
  .inner {
    padding: 4px 0;
  }
  .n {
    width: 52px;
    flex: none !important;
  }
  .scope {
    flex: 1;
  }
  .results {
    flex: 1;
    min-height: 60px;
    max-height: 260px;
    overflow-y: auto;
    padding: 2px;
  }
  .rp {
    color: var(--gold);
    flex: none;
    min-width: 86px;
  }
  .small {
    font-size: 12px;
    margin: 0;
  }
  .replace .t-btn,
  .row .t-btn {
    min-width: 0;
    padding: 0 8px;
  }
</style>
