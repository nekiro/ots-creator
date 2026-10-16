<script lang="ts">
  import { onMount } from "svelte";
  import { app } from "../../lib/state.svelte";
  import { readJson, saveJson } from "../../lib/storage";
  import { commands } from "../../lib/commands";
  import { MapService } from "../../lib/map/api";
  import { mapCommands } from "../../lib/map/commands";
  import { loadMapData, map } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import MiniWindow from "../../lib/ui/MiniWindow.svelte";
  import PromptDialog from "../../lib/ui/PromptDialog.svelte";
  import Splitter from "../../lib/ui/Splitter.svelte";
  import CommandPalette from "./CommandPalette.svelte";
  import FindPanel from "./FindPanel.svelte";
  import Inspector from "./Inspector.svelte";
  import Library from "./Library.svelte";
  import MapCanvas from "./MapCanvas.svelte";
  import MapToolbar from "./MapToolbar.svelte";
  import LoadingMap from "./LoadingMap.svelte";
  import Minimap from "./Minimap.svelte";
  import World from "./World.svelte";
  import MapDialogs from "./dialogs/MapDialogs.svelte";
  import "./map.css";

  let { hoverText = $bindable("") }: { hoverText?: string } = $props();

  // The client may have changed while the object editor was shown.
  let lastProject: unknown = null;
  $effect(() => {
    const p = app.project?.open ? (app.project.info.datPath || app.project.info.version.name) : null;
    if (p !== lastProject) {
      lastProject = p;
      void loadMapData(true);
    }
  });

  onMount(() => {
    void loadMapData();
  });

  // Widths of the side panels; the splitters trade them with the map.
  const SIDES_KEY = "map:sides";
  // The panels' forms are laid out for these widths and break when narrower,
  // so they are also the minimum: the splitters only widen the panels.
  const SIDE_MIN = { left: 290, right: 270 };
  const CENTER_MIN = 400;
  let layout = $state<HTMLElement>();
  let sides = $state(loadSides());

  function loadSides(): { left: number; right: number } {
    const stored = (readJson(SIDES_KEY) ?? {}) as { left?: unknown; right?: unknown };
    return { left: Math.max(SIDE_MIN.left, Number(stored.left) || 0), right: Math.max(SIDE_MIN.right, Number(stored.right) || 0) };
  }

  function resizeSide(side: "left" | "right", dx: number) {
    const other = side === "left" ? (map.maps.length ? sides.right : 0) : sides.left;
    const max = Math.max(SIDE_MIN[side], (layout?.clientWidth ?? 1600) - other - CENTER_MIN - 20);
    const next = side === "left" ? sides[side] + dx : sides[side] - dx;
    sides[side] = Math.round(Math.max(SIDE_MIN[side], Math.min(max, next)));
  }

  function saveSides() {
    saveJson(SIDES_KEY, sides);
  }

  // Painting houses, zones or areas belongs to the world; show it there.
  $effect(() => {
    const k = map.brush?.kind;
    if (map.tool === "brush" && (k === "house" || k === "houseExit" || k === "zone" || k === "flag" || k === "waypoint")) map.side = "world";
  });
</script>

<main class="maplayout" class:nomap={app.open && !map.maps.length} bind:this={layout} style="--left-w:{sides.left}px;--right-w:{sides.right}px">
  {#if !app.open}
    <div class="welcome">
      <section class="t-window card">
        <div class="t-window-title">Map editor</div>
        <p>The map editor draws maps with the sprites of a client. Open the client of your server first.</p>
        <div class="row">
          <button class="t-btn primary" onclick={() => commands.browseOpen()}>Open client…</button>
          <button class="t-btn" onclick={() => mapCommands.setMode("objects")}>Back to objects</button>
        </div>
      </section>
    </div>
  {:else}
    <div class="left">
      <MiniWindow title={map.side === "library" ? "Draw with" : "On this map"} class="side">
        {#snippet actions()}
          <button class="t-icon-btn" title="Items and brushes: items.otb, items.xml, brush import" onclick={() => (map.dialog = "data")}><Icon name="tools" /></button>
        {/snippet}
        <div class="t-tabs sidetabs" role="tablist">
          <button class="t-tab" role="tab" class:on={map.side === "library"} aria-selected={map.side === "library"} title="Brushes, items and creatures to draw with" onclick={() => (map.side = "library")}>Library</button>
          <button class="t-tab" role="tab" class:on={map.side === "world"} aria-selected={map.side === "world"} title="Towns, houses, waypoints and areas of the map" onclick={() => (map.side = "world")}>World</button>
        </div>
        {#if map.side === "library"}<Library />{:else}<World />{/if}
      </MiniWindow>
    </div>
    <Splitter vertical ondrag={(dx) => resizeSide("left", dx)} onend={saveSides} />
    <div class="center">
      <!-- With maps open the canvas goes edge to edge: the tabs name the map, no window frame. -->
      <section class="mapwin" class:t-window={!map.maps.length} class:open={map.maps.length > 0}>
        {#if !map.maps.length}<div class="t-window-title">Map</div>{/if}
        {#if map.maps.length}
          <div class="head t-flat">
            <div class="t-tabs tabs" role="tablist">
              {#each map.maps as m (m.id)}
                <div class="t-tab tab" class:on={m.id === map.activeId} role="tab" aria-selected={m.id === map.activeId}>
                  <button class="tabname" title={m.path || "Not saved yet"} onclick={() => (map.activeId = m.id)}>{m.name}{m.changed ? " ●" : ""}</button>
                  <button class="x" title="Close map (Ctrl+W)" onclick={() => mapCommands.close(m.id)}>×</button>
                </div>
              {/each}
              <button class="t-icon-btn add" title="New map (Ctrl+N)" onclick={mapCommands.newMap}><Icon name="plus" /></button>
              <button class="t-icon-btn add" title="Open map (Ctrl+O)" onclick={() => mapCommands.open()}><Icon name="open" /></button>
            </div>
            <MapToolbar />
          </div>
          <MapCanvas bind:hoverText />
        {:else}
          <div class="start">
            <div class="startcard">
              <div class="row">
                <button class="t-btn primary" onclick={mapCommands.newMap}>New map</button>
                <button class="t-btn" onclick={() => mapCommands.open()}>Open map…</button>
              </div>
              {#if map.recent.length}
                <div class="t-label rtitle">Recent maps</div>
                <div class="t-panel rlist">
                  {#each map.recent as r (r.path)}
                    <div class="ritem">
                      <button class="ropen" title={r.path} onclick={() => mapCommands.open(r.path)}>{r.path}</button>
                      <button class="rdel" title="Remove from list" onclick={() => MapService.RemoveRecent(r.path).then(() => loadMapData())}>×</button>
                    </div>
                  {/each}
                </div>
              {/if}
              <div class="data t-panel">
                {#if map.data?.ready}
                  <div><span class="t-label">Items</span> {map.data.items.toLocaleString()} · {map.data.mode === "otb" ? `items.otb ${map.data.otbVersion}` : "client ids (no items.otb)"}</div>
                  <div><span class="t-label">Brushes</span> {map.data.brushes} · <span class="t-label">tilesets</span> {map.data.tilesets} · <span class="t-label">creatures</span> {map.data.creatures}</div>
                  {#if map.data.brushes === 0}
                    <div class="t-label hint">Without brushes you can still place any item (Library › Items). Import brushes for automatic borders, walls and decoration.</div>
                  {/if}
                {:else}
                  <div class="t-label">Loading the item data…</div>
                {/if}
                <button class="t-btn" onclick={() => (map.dialog = "data")}>Items and brushes…</button>
              </div>
              <p class="t-label keys"><kbd>Ctrl+N</kbd> new · <kbd>Ctrl+O</kbd> open · <kbd>Ctrl+K</kbd> go to anything · <kbd>F1</kbd> shortcuts</p>
            </div>
          </div>
        {/if}
        {#if map.busy}<div class="busy">{map.busy}…</div>{/if}
      </section>
    </div>
    {#if map.maps.length}
    <Splitter vertical ondrag={(dx) => resizeSide("right", dx)} onend={saveSides} />
    <div class="right">
      <Minimap />
      {#if map.findOpen}<FindPanel />{/if}
      <Inspector />
    </div>
    {/if}
  {/if}
</main>

<MapDialogs />
<PromptDialog />
{#if map.loading}<LoadingMap />{/if}
{#if map.command !== null}<CommandPalette />{/if}

<style>
  .maplayout {
    flex: 1;
    min-height: 0;
    display: grid;
    /* The splitter columns are the 6px gaps between the panels. */
    grid-template-columns: var(--left-w) auto minmax(400px, 1fr) auto var(--right-w);
    padding: 4px;
  }
  .maplayout.nomap {
    grid-template-columns: var(--left-w) auto minmax(400px, 1fr);
  }
  .left,
  .right,
  .center {
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .left :global(.side) {
    flex: 1;
  }
  .sidetabs {
    margin-bottom: 4px;
  }
  .right :global(.inspector) {
    flex: 1;
    min-height: 200px;
  }
  .right :global(.find) {
    flex: 0 1 auto;
    max-height: 55%;
    min-height: 160px;
  }
  .mapwin {
    flex: 1;
    min-height: 0;
    padding: 4px 6px 6px;
  }
  .mapwin.open {
    position: relative;
    display: flex;
    flex-direction: column;
    padding: 0;
    border: 1px solid #111;
    background: #000;
  }
  .head {
    flex: none;
    padding: 2px 2px 0;
  }
  .tabs {
    align-items: flex-end;
    overflow: hidden;
  }
  /* A .t-tab holding the name and its close button; sized by its name. */
  .tab {
    flex: 0 1 auto;
    display: flex;
    align-items: center;
    height: 20px;
    padding: 0 0 0 6px;
    max-width: 200px;
    min-width: 0;
  }
  .tabname {
    padding: 0;
    background: none;
    border: 0;
    color: inherit;
    font: inherit;
    text-shadow: inherit;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .x {
    background: none;
    border: 0;
    color: var(--text-dim);
    cursor: pointer;
    font: inherit;
    padding: 0 3px;
  }
  .x:hover {
    color: #ff9c9c;
  }
  .add {
    transform: scale(0.85);
  }
  .start {
    flex: 1;
    display: grid;
    place-items: center;
  }
  .startcard {
    width: 440px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    align-items: stretch;
  }
  .startcard > .row {
    justify-content: center;
  }
  .rtitle {
    margin-bottom: -6px;
  }
  .rlist {
    padding: 2px;
  }
  .ritem {
    display: flex;
  }
  .ritem:hover {
    background: rgba(255, 255, 255, 0.06);
  }
  .ropen {
    flex: 1;
    min-width: 0;
    background: none;
    border: 0;
    color: var(--text);
    font: inherit;
    text-align: left;
    padding: 3px 6px;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl;
  }
  .rdel {
    background: none;
    border: 0;
    color: var(--text-dim);
    cursor: pointer;
    visibility: hidden;
  }
  .ritem:hover .rdel {
    visibility: visible;
  }
  .data {
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    align-items: flex-start;
  }
  .hint {
    font-size: 12px;
  }
  .keys {
    text-align: center;
    margin: 0;
  }
  .busy {
    position: absolute;
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    background: rgba(0, 0, 0, 0.8);
    border: 1px solid #555;
    padding: 8px 16px;
    color: var(--gold);
  }
  .welcome {
    grid-column: 1 / -1;
    display: grid;
    place-items: center;
  }
  .card {
    width: 420px;
    padding: 16px 20px 14px;
    align-items: center;
    text-align: center;
    gap: 10px;
  }
  .card p {
    margin: 0;
  }
</style>
