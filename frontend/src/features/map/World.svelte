<script lang="ts">
  // What the map contains: towns with their houses, waypoints and areas.
  // A click shows the thing in the inspector, a double click goes there.
  import { TILE_FLAGS, type HouseInfo, type Town, type Waypoint } from "../../lib/map/data/api";
  import { goTo, mapCommands } from "../../lib/map/view/commands";
  import { openContextMenu } from "../../lib/menu.svelte";
  import { isBrushActive, map, selectBrush } from "../../lib/map/state.svelte";
  import { addHouse, addTown, addWaypoint, addZone, goHouse, houses as allHouses, paintHouse, pickPosition, removeTown, saveTown, towns as allTowns } from "../../lib/map/editor/world";
  import Icon from "../../lib/ui/Icon.svelte";
  import SearchField from "../../lib/ui/SearchField.svelte";

  const AREAS: { flag: number; label: string; color: string; title: string }[] = [
    { flag: TILE_FLAGS.pz, label: "Protection zone", color: "#4c4", title: "No fights; players heal and log out safely" },
    { flag: TILE_FLAGS.noPvp, label: "No PvP", color: "#cc4", title: "Players cannot attack each other" },
    { flag: TILE_FLAGS.pvpZone, label: "PvP arena", color: "#c4c", title: "Fights without penalties" },
    { flag: TILE_FLAGS.noLogout, label: "No logout", color: "#c44", title: "Players cannot log out here" },
  ];

  let filter = $state("");
  let open = $state<Record<number, boolean>>({});
  const f = $derived(filter.trim().toLowerCase());
  const m = $derived(map.active);
  const towns = $derived(m ? allTowns() : []);
  const houses = $derived(m ? allHouses() : []);
  const waypoints = $derived((m?.waypoints ?? []).filter((w): w is Waypoint => !!w && (!f || w.name.toLowerCase().includes(f))));
  const zones = $derived((m?.zones ?? []).filter((z) => !f || z.name.toLowerCase().includes(f)));
  const homeless = $derived(houses.filter((h) => !towns.some((t) => t.id === h.townId)));

  function townHouses(id: number): HouseInfo[] {
    return houses.filter((h) => h.townId === id && (!f || h.name.toLowerCase().includes(f)));
  }
  function townShown(id: number, name: string): boolean {
    return !f || name.toLowerCase().includes(f) || townHouses(id).length > 0;
  }

  const focus = $derived(map.focus);
  const on = (kind: string, id: number | string) => !!focus && focus.kind === kind && ("id" in focus ? focus.id === id : "name" in focus && focus.name === id);

  function townMenu(event: MouseEvent, t: Town) {
    const hasTemple = !!(t.temple.x || t.temple.y);
    openContextMenu(event, [
      { label: "Go to the temple", action: () => goTo(t.temple), disabled: () => !hasTemple },
      { label: "Set the temple…", action: () => pickPosition(`Click the temple of ${t.name}`, (p) => saveTown({ ...t, temple: p })) },
      "-",
      { label: open[t.id] ? "Hide houses" : "Show houses", action: () => (open[t.id] = !open[t.id]) },
      { label: "New house in this town", action: () => addHouse(t.id) },
      "-",
      { label: "Remove town", action: () => removeTown(t) },
    ]);
  }

  function pickHouse(h: HouseInfo) {
    mapCommands.focus({ kind: "house", id: h.id });
    paintHouse(h);
  }
</script>

{#snippet houseRow(h: HouseInfo, nested: boolean)}
  <button class="m-row" class:nested class:on={on("house", h.id)} title="Click to paint its tiles · double-click to go there" onclick={() => pickHouse(h)} ondblclick={() => goHouse(h)}>
    <span class="ico"><Icon name="home" size={12} /></span>
    <span class="m-name">{h.name}</span>
    <span class="m-meta">{h.size ? `${h.size} sqm` : "no tiles"}</span>
  </button>
{/snippet}

<div class="world">
  {#if !m}
    <div class="m-empty">Open a map to see its towns, houses, waypoints and areas.</div>
  {:else}
    <SearchField bind:value={filter} />
    <div class="m-body t-panel m-scroll">
      <button class="m-row maprow" class:on={!focus || focus.kind === "map"} title="Map properties and statistics" onclick={() => mapCommands.focus({ kind: "map" })}>
        <span class="ico"><Icon name="sheet" size={12} /></span>
        <span class="m-name">{m.name}</span>
        <span class="m-meta">{m.tiles.toLocaleString()} tiles</span>
      </button>

      <div class="m-sect">Towns <span class="grow"></span><button class="t-icon-btn add" title="Add a town" onclick={addTown}><Icon name="plus" /></button></div>
      {#each towns as t (t.id)}
        {#if townShown(t.id, t.name)}
          {@const list = townHouses(t.id)}
          {@const expanded = !!f || open[t.id]}
          <div class="town">
            <button class="chev" title={expanded ? "Hide houses" : "Show houses"} onclick={() => (open[t.id] = !open[t.id])}>{expanded ? "▾" : "▸"}</button>
            <button class="m-row" class:on={on("town", t.id)} title="Double-click to go to the temple · right-click for more" oncontextmenu={(event) => townMenu(event, t)} onclick={() => mapCommands.focus({ kind: "town", id: t.id })} ondblclick={() => t.temple.x && goTo(t.temple)}>
              <span class="m-name">{t.name}</span>
              <span class="m-meta">{houses.filter((h) => h.townId === t.id).length} houses</span>
            </button>
          </div>
          {#if expanded}
            {#each list as h (h.id)}{@render houseRow(h, true)}{/each}
            <button class="m-row nested addrow" onclick={() => addHouse(t.id)}><span class="ico"><Icon name="plus" size={12} /></span><span class="m-name">New house in {t.name}</span></button>
          {/if}
        {/if}
      {:else}
        {#if !f}
          <div class="hint">Towns hold houses and the temple players start at. <button class="link" onclick={addTown}>Add a town</button></div>
        {/if}
      {/each}
      {#if homeless.length && (!f || homeless.some((h) => h.name.toLowerCase().includes(f)))}
        <div class="m-sect">Houses without a town</div>
        {#each homeless.filter((h) => !f || h.name.toLowerCase().includes(f)) as h (h.id)}{@render houseRow(h, false)}{/each}
      {/if}

      <div class="m-sect">Waypoints <span class="grow"></span><button class="t-icon-btn add" title="Add a waypoint" onclick={addWaypoint}><Icon name="plus" /></button></div>
      {#each waypoints as w (w.name)}
        <button class="m-row" class:on={on("waypoint", w.name)} title="{w.pos.x}, {w.pos.y}, {w.pos.z} · double-click to go there" onclick={() => mapCommands.focus({ kind: "waypoint", name: w.name })} ondblclick={() => goTo(w.pos)}>
          <span class="ico"><Icon name="flag" size={12} /></span>
          <span class="m-name">{w.name}</span>
          <span class="m-meta">{w.pos.x}, {w.pos.y}, {w.pos.z}</span>
        </button>
      {:else}
        {#if !f}<div class="hint">Named places for scripts and NPC travel.</div>{/if}
      {/each}

      {#if !f}
        <div class="m-sect">Areas</div>
        {#each AREAS as a}
          {@const sel = { kind: "flag", flag: a.flag, label: a.label }}
          <button class="m-row" class:on={isBrushActive(sel)} title="{a.title} · paint it on the map, Ctrl removes" onclick={() => selectBrush(sel)}>
            <span class="m-swatch" style="background:{a.color}"></span>
            <span class="m-name">{a.label}</span>
          </button>
        {/each}
      {/if}
      <div class="m-sect">Zones <span class="grow"></span><button class="t-icon-btn add" title="Add a zone" onclick={addZone}><Icon name="plus" /></button></div>
      {#each zones as z (z.id)}
        {@const sel = { kind: "zone", zone: z.id, label: `Zone ${z.name}` }}
        <button class="m-row" class:on={isBrushActive(sel) || on("zone", z.id)} title="Click to paint it · Ctrl removes" onclick={() => (mapCommands.focus({ kind: "zone", id: z.id }), selectBrush(sel))}>
          <span class="m-swatch zone"></span>
          <span class="m-name">{z.name}</span>
          <span class="m-meta">#{z.id}</span>
        </button>
      {:else}
        {#if !f}<div class="hint">Named tile groups for scripts.</div>{/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .world {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .maprow {
    margin-bottom: 2px;
  }
  .ico {
    width: 26px;
    display: grid;
    place-items: center;
    color: var(--text-dim);
    flex: none;
  }
  .town {
    display: flex;
    align-items: center;
  }
  .chev {
    width: 16px;
    flex: none;
    background: none;
    border: 0;
    color: var(--text-dim);
    cursor: pointer;
    padding: 0;
    font-size: 11px;
  }
  .nested {
    padding-left: 16px;
  }
  .addrow {
    color: var(--text-dim);
  }
  .add {
    transform: scale(0.75);
    margin: -4px -2px;
  }
  .hint {
    padding: 2px 6px 4px;
    color: var(--text-dim);
    font-size: 12px;
  }
  .link {
    background: none;
    border: 0;
    padding: 0;
    color: var(--gold);
    font: inherit;
    cursor: pointer;
    text-decoration: underline;
  }
  .zone {
    background: rgba(80, 200, 255, 0.4);
  }
</style>
