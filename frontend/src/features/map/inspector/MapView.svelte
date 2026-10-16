<script lang="ts">
  // The map itself: properties (saved as they change) and what is on it.
  import { errorMessage } from "../../../lib/api";
  import { MapService, SpawnFormat, type MapInfo, type Properties, type Statistics } from "../../../lib/map/api";
  import { edit, map } from "../../../lib/map/state.svelte";
  import Icon from "../../../lib/ui/Icon.svelte";
  import NumberField from "../../../lib/ui/NumberField.svelte";
  import Select from "../../../lib/ui/Select.svelte";

  let { info: m }: { info: MapInfo } = $props();
  const p = $derived(m.props);

  function set(patch: Partial<Properties>) {
    const next = { ...p, ...patch };
    void edit((id) => MapService.SetProperties(id, next));
  }

  let stats = $state<Statistics | null>(null);
  let statsRev = $state(-1);
  let error = $state("");
  let counting = $state(false);
  async function count() {
    if (counting) return;
    counting = true;
    try {
      stats = await MapService.Stats(m.id);
      statsRev = m.rev;
      error = "";
    } catch (err) {
      error = errorMessage(err);
    } finally {
      counting = false;
    }
  }
  // Counts when shown and again a moment after edits; big maps only on
  // request since counting them takes a while.
  $effect(() => {
    const rev = m.rev;
    if (statsRev === rev) return;
    if (statsRev >= 0 && m.tiles > 400_000) return;
    const t = setTimeout(() => void count(), statsRev < 0 ? 0 : 700);
    return () => clearTimeout(t);
  });

  const separate = $derived(p.spawnFormat === SpawnFormat.SpawnsSeparate);
  const rows = $derived(
    stats
      ? [
          ["Tiles", stats.tiles],
          ["Items", stats.items + stats.containerItems],
          ["Walkable", stats.walkable],
          ["Protection zone", stats.pz],
          ["Houses", stats.houses],
          ["House tiles", stats.houseTiles],
          ["Spawns", stats.spawns],
          ["Monsters", stats.monsters],
          ["NPCs", stats.npcs],
          ["Teleports", stats.teleports],
          ["Action ids", stats.actionIds],
          ["Unique ids", stats.uniqueIds],
        ]
      : [],
  );
</script>

<div class="m-form">
  <label class="col"><span class="t-label">Description</span><textarea class="t-input text" value={p.description} placeholder="Shown to players by some servers" onchange={(event) => set({ description: event.currentTarget.value })}></textarea></label>
  <div class="m-field">
    <span class="m-lbl">Spawns</span>
    <Select grow value={p.spawnFormat} options={[{ value: SpawnFormat.SpawnsSingle, label: "One file (monsters and NPCs)" }, { value: SpawnFormat.SpawnsSeparate, label: "Separate monster and NPC files" }]} onchange={(value) => set({ spawnFormat: value })} />
  </div>
  <div class="m-field">
    <span class="m-lbl">Size</span>
    <NumberField value={p.width} min={1} max={65535} width={64} onchange={(value) => set({ width: value })} /><span>×</span><NumberField value={p.height} min={1} max={65535} width={64} onchange={(value) => set({ height: value })} />
  </div>

  <details class="files">
    <summary class="t-label">Files saved with the map</summary>
    <div class="m-form inner">
      <div class="m-field"><span class="m-lbl">Houses</span><input class="t-input" value={p.houseFile} placeholder="{m.name.replace(/\.otbm$/i, '')}-house.xml" onchange={(event) => set({ houseFile: event.currentTarget.value })} /></div>
      <div class="m-field"><span class="m-lbl">{separate ? "Monsters" : "Spawns"}</span><input class="t-input" value={p.spawnFile} placeholder="{m.name.replace(/\.otbm$/i, '')}-{separate ? 'monster' : 'spawn'}.xml" onchange={(event) => set({ spawnFile: event.currentTarget.value })} /></div>
      {#if separate}<div class="m-field"><span class="m-lbl">NPCs</span><input class="t-input" value={p.npcFile} placeholder="{m.name.replace(/\.otbm$/i, '')}-npc.xml" onchange={(event) => set({ npcFile: event.currentTarget.value })} /></div>{/if}
      <div class="m-field"><span class="m-lbl">Zones</span><input class="t-input" value={p.zoneFile} placeholder="{m.name.replace(/\.otbm$/i, '')}-zones.xml" onchange={(event) => set({ zoneFile: event.currentTarget.value })} /></div>
      <div class="t-label path">{m.path || "Not saved yet"}</div>
    </div>
  </details>

  <div class="m-sect">
    On the map <span class="grow"></span>
    <button class="t-icon-btn" title={statsRev === m.rev ? "Up to date" : "Count again"} disabled={counting} onclick={count}><Icon name="refresh" /></button>
  </div>
  {#if error}
    <p class="warn">{error}</p>
  {:else if stats}
    <div class="stats" class:stale={statsRev !== m.rev}>
      {#each rows as [k, v]}
        <span class="k">{k}</span><span class="v">{Number(v).toLocaleString()}</span>
      {/each}
    </div>
  {:else}
    <p class="t-label">Counting…</p>
  {/if}

  <div class="acts">
    <button class="t-btn" onclick={() => (map.dialog = "tools")}><Icon name="tools" /> Map tools…</button>
    <button class="t-btn" onclick={() => (map.dialog = "import")}>Import a map…</button>
  </div>
</div>

<style>
  .col {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .text {
    height: 44px;
    resize: vertical;
    font-weight: normal;
    padding: 3px;
  }
  .warn {
    margin: 0;
    color: var(--gold);
    font-size: 12px;
  }
  .files summary {
    cursor: pointer;
    padding: 2px 0;
  }
  .inner {
    padding: 4px 0 2px;
  }
  .path {
    font-size: 11px;
    word-break: break-all;
  }
  .stats {
    display: grid;
    grid-template-columns: 1fr auto 1fr auto;
    gap: 1px 8px;
    font-size: 12px;
  }
  .stats.stale .v {
    color: var(--text-dim);
  }
  .k {
    color: var(--text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .v {
    text-align: right;
    color: var(--text-bright);
  }
  .acts {
    display: flex;
    gap: 3px;
    flex-wrap: wrap;
    margin-top: 4px;
  }
  .acts .t-btn {
    min-width: 0;
    padding: 0 8px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
</style>
