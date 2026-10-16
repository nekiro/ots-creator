<script lang="ts">
  // Shows whatever is in focus: a tile, a town, a house, a waypoint, a zone,
  // or the map itself when nothing is.
  import { fmtPos } from "../../lib/map/data/api";
  import { mapCommands } from "../../lib/map/view/commands";
  import { map } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import MiniWindow from "../../lib/ui/MiniWindow.svelte";
  import HouseView from "./inspector/HouseView.svelte";
  import MapView from "./inspector/MapView.svelte";
  import TownView from "./inspector/TownView.svelte";
  import WaypointView from "./inspector/WaypointView.svelte";
  import ZoneView from "./inspector/ZoneView.svelte";
  import TileView from "./TileView.svelte";

  const m = $derived(map.active);
  const f = $derived(map.focus);
  const town = $derived(f?.kind === "town" ? (m?.towns?.find((t) => t?.id === f.id) ?? null) : null);
  const house = $derived(f?.kind === "house" ? (m?.houses?.find((h) => h?.id === f.id) ?? null) : null);
  const waypoint = $derived(f?.kind === "waypoint" ? (m?.waypoints?.find((w) => w?.name === f.name) ?? null) : null);
  const zone = $derived(f?.kind === "zone" ? (m?.zones?.find((z) => z.id === f.id) ?? null) : null);
  /** A waypoint named but not placed yet. */
  const pendingWaypoint = $derived(f?.kind === "waypoint" && !waypoint ? { name: f.name, pos: { x: 0, y: 0, z: 0 } } : null);

  const view = $derived.by(() => {
    if (f?.kind === "tile" && map.inspect) return "tile";
    if (town) return "town";
    if (house) return "house";
    if (waypoint || pendingWaypoint) return "waypoint";
    if (zone) return "zone";
    return "map";
  });
  const title = $derived(
    view === "tile"
      ? `Tile ${fmtPos(map.inspect)}`
      : view === "town"
        ? `Town · ${town!.name}`
        : view === "house"
          ? `House · ${house!.name}`
          : view === "waypoint"
            ? `Waypoint · ${(waypoint ?? pendingWaypoint)!.name}`
            : view === "zone"
              ? `Zone · ${zone!.name}`
              : "Map",
  );
</script>

<MiniWindow {title} class="inspector">
  {#snippet actions()}
    {#if view !== "map"}
      <button class="t-icon-btn" title="Back to the map (Esc)" onclick={() => (map.inspect && view === "tile" ? mapCommands.inspect(null) : mapCommands.focus(null))}><Icon name="close" /></button>
    {/if}
  {/snippet}
  <div class="body">
    {#if !m}
      <div class="m-empty">No map open.</div>
    {:else if view === "tile"}
      <TileView />
    {:else if view === "town" && town}
      <TownView {town} />
    {:else if view === "house" && house}
      <HouseView {house} />
    {:else if view === "waypoint"}
      <WaypointView waypoint={(waypoint ?? pendingWaypoint)!} pending={!waypoint} />
    {:else if view === "zone" && zone}
      <ZoneView {zone} />
    {:else}
      <MapView info={m} />
    {/if}
  </div>
</MiniWindow>

<style>
  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    padding: 2px;
  }
</style>
