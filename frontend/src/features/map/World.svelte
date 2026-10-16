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
  const query = $derived(filter.trim().toLowerCase());
  const activeMap = $derived(map.active);
  const towns = $derived(activeMap ? allTowns() : []);
  const houses = $derived(activeMap ? allHouses() : []);
  const waypoints = $derived((activeMap?.waypoints ?? []).filter((waypoint): waypoint is Waypoint => !!waypoint && (!query || waypoint.name.toLowerCase().includes(query))));
  const zones = $derived((activeMap?.zones ?? []).filter((zone) => !query || zone.name.toLowerCase().includes(query)));
  const homeless = $derived(houses.filter((house) => !towns.some((town) => town.id === house.townId)));

  function townHouses(id: number): HouseInfo[] {
    return houses.filter((house) => house.townId === id && (!query || house.name.toLowerCase().includes(query)));
  }
  function townShown(id: number, name: string): boolean {
    return !query || name.toLowerCase().includes(query) || townHouses(id).length > 0;
  }

  const focus = $derived(map.focus);
  const on = (kind: string, id: number | string) => !!focus && focus.kind === kind && ("id" in focus ? focus.id === id : "name" in focus && focus.name === id);

  function townMenu(event: MouseEvent, town: Town) {
    const hasTemple = !!(town.temple.x || town.temple.y);
    openContextMenu(event, [
      { label: "Go to the temple", action: () => goTo(town.temple), disabled: () => !hasTemple },
      { label: "Set the temple…", action: () => pickPosition(`Click the temple of ${town.name}`, (position) => saveTown({ ...town, temple: position })) },
      "-",
      { label: open[town.id] ? "Hide houses" : "Show houses", action: () => (open[town.id] = !open[town.id]) },
      { label: "New house in this town", action: () => addHouse(town.id) },
      "-",
      { label: "Remove town", action: () => removeTown(town) },
    ]);
  }

  function pickHouse(house: HouseInfo) {
    mapCommands.focus({ kind: "house", id: house.id });
    paintHouse(house);
  }
</script>

{#snippet houseRow(house: HouseInfo, nested: boolean)}
  <button class="m-row" class:nested class:on={on("house", house.id)} title="Click to paint its tiles · double-click to go there" onclick={() => pickHouse(house)} ondblclick={() => goHouse(house)}>
    <span class="ico"><Icon name="home" size={12} /></span>
    <span class="m-name">{house.name}</span>
    <span class="m-meta">{house.size ? `${house.size} sqm` : "no tiles"}</span>
  </button>
{/snippet}

<div class="world">
  {#if !activeMap}
    <div class="m-empty">Open a map to see its towns, houses, waypoints and areas.</div>
  {:else}
    <SearchField bind:value={filter} />
    <div class="m-body t-panel m-scroll">
      <button class="m-row maprow" class:on={!focus || focus.kind === "map"} title="Map properties and statistics" onclick={() => mapCommands.focus({ kind: "map" })}>
        <span class="ico"><Icon name="sheet" size={12} /></span>
        <span class="m-name">{activeMap.name}</span>
        <span class="m-meta">{activeMap.tiles.toLocaleString()} tiles</span>
      </button>

      <div class="m-sect">Towns <span class="grow"></span><button class="t-icon-btn add" title="Add a town" onclick={addTown}><Icon name="plus" /></button></div>
      {#each towns as town (town.id)}
        {#if townShown(town.id, town.name)}
          {@const list = townHouses(town.id)}
          {@const expanded = !!query || open[town.id]}
          <div class="town">
            <button class="chev" title={expanded ? "Hide houses" : "Show houses"} onclick={() => (open[town.id] = !open[town.id])}>{expanded ? "▾" : "▸"}</button>
            <button class="m-row" class:on={on("town", town.id)} title="Double-click to go to the temple · right-click for more" oncontextmenu={(event) => townMenu(event, town)} onclick={() => mapCommands.focus({ kind: "town", id: town.id })} ondblclick={() => town.temple.x && goTo(town.temple)}>
              <span class="m-name">{town.name}</span>
              <span class="m-meta">{houses.filter((house) => house.townId === town.id).length} houses</span>
            </button>
          </div>
          {#if expanded}
            {#each list as house (house.id)}{@render houseRow(house, true)}{/each}
            <button class="m-row nested addrow" onclick={() => addHouse(town.id)}><span class="ico"><Icon name="plus" size={12} /></span><span class="m-name">New house in {town.name}</span></button>
          {/if}
        {/if}
      {:else}
        {#if !query}
          <div class="hint">Towns hold houses and the temple players start at. <button class="link" onclick={addTown}>Add a town</button></div>
        {/if}
      {/each}
      {#if homeless.length && (!query || homeless.some((house) => house.name.toLowerCase().includes(query)))}
        <div class="m-sect">Houses without a town</div>
        {#each homeless.filter((house) => !query || house.name.toLowerCase().includes(query)) as house (house.id)}{@render houseRow(house, false)}{/each}
      {/if}

      <div class="m-sect">Waypoints <span class="grow"></span><button class="t-icon-btn add" title="Add a waypoint" onclick={addWaypoint}><Icon name="plus" /></button></div>
      {#each waypoints as waypoint (waypoint.name)}
        <button class="m-row" class:on={on("waypoint", waypoint.name)} title="{waypoint.pos.x}, {waypoint.pos.y}, {waypoint.pos.z} · double-click to go there" onclick={() => mapCommands.focus({ kind: "waypoint", name: waypoint.name })} ondblclick={() => goTo(waypoint.pos)}>
          <span class="ico"><Icon name="flag" size={12} /></span>
          <span class="m-name">{waypoint.name}</span>
          <span class="m-meta">{waypoint.pos.x}, {waypoint.pos.y}, {waypoint.pos.z}</span>
        </button>
      {:else}
        {#if !query}<div class="hint">Named places for scripts and NPC travel.</div>{/if}
      {/each}

      {#if !query}
        <div class="m-sect">Areas</div>
        {#each AREAS as area}
          {@const brush = { kind: "flag", flag: area.flag, label: area.label }}
          <button class="m-row" class:on={isBrushActive(brush)} title="{area.title} · paint it on the map, Ctrl removes" onclick={() => selectBrush(brush)}>
            <span class="m-swatch" style="background:{area.color}"></span>
            <span class="m-name">{area.label}</span>
          </button>
        {/each}
      {/if}
      <div class="m-sect">Zones <span class="grow"></span><button class="t-icon-btn add" title="Add a zone" onclick={addZone}><Icon name="plus" /></button></div>
      {#each zones as zone (zone.id)}
        {@const brush = { kind: "zone", zone: zone.id, label: `Zone ${zone.name}` }}
        <button class="m-row" class:on={isBrushActive(brush) || on("zone", zone.id)} title="Click to paint it · Ctrl removes" onclick={() => (mapCommands.focus({ kind: "zone", id: zone.id }), selectBrush(brush))}>
          <span class="m-swatch zone"></span>
          <span class="m-name">{zone.name}</span>
          <span class="m-meta">#{zone.id}</span>
        </button>
      {:else}
        {#if !query}<div class="hint">Named tile groups for scripts.</div>{/if}
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
