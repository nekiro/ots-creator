<script lang="ts">
  import { mapCommands } from "../../lib/map/view/commands";
  import { creatureOutfit, map, selectBrush, type BrushSel } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import Slider from "../../lib/ui/Slider.svelte";
  import ItemIcon from "./ItemIcon.svelte";

  const b = $derived(map.brush);
  const isDoodad = $derived(b?.kind === "doodad");
  const isCreature = $derived(b?.kind === "creature");
  const sized = $derived(!!b && !["door", "houseExit", "waypoint", "creature"].includes(b.kind));
  /** Brushes without an item show a color. */
  const SWATCHES: Record<string, string> = { house: "#5a6fd8", houseExit: "#5a6fd8", flag: "#4c4", zone: "#50c8ff", spawn: "#f66", spawnNpc: "#6af", waypoint: "#7c7", optionalBorder: "#a87", eraser: "#ddd" };
  const FLAG_COLORS: Record<number, string> = { 1: "#4c4", 4: "#cc4", 8: "#c44", 16: "#c4c" };
  const swatch = $derived(b && !b.look && !b.item ? (b.kind === "flag" ? FLAG_COLORS[b.flag ?? 0] : SWATCHES[b.kind]) || "#888" : "");

  // Walls switch between the wall itself and the doors and windows that go
  // into it.
  const DOORS: [string, string, string][] = [
    ["normal", "Door", "A door; Alt places it open"],
    ["locked", "Locked", "A locked door (needs a key)"],
    ["quest", "Quest", "A quest door"],
    ["magic", "Magic", "A level door"],
    ["archway", "Arch", "An archway"],
    ["window", "Window", "A window"],
    ["hatch window", "Hatch", "A hatch window"],
  ];
  let lastWall = $state<BrushSel | null>(null);
  $effect(() => {
    if (b?.kind === "wall") lastWall = b;
  });
  const walling = $derived(!!b && (b.kind === "wall" || (b.kind === "door" && !!lastWall)));
  let density = $state(Math.round(map.thickness * 100));
  $effect(() => {
    map.thickness = density / 100;
  });

  type ViewToggle = { [Key in keyof typeof map.view]: (typeof map.view)[Key] extends boolean ? Key : never }[keyof typeof map.view];
  const QUICK: { key: ViewToggle; icon: string; title: string }[] = [
    { key: "grid", icon: "grid", title: "Grid (G)" },
    { key: "lowerFloors", icon: "sheet", title: "Show lower floors (L)" },
    { key: "ghostHigher", icon: "film", title: "Ghost the floor above (H)" },
    { key: "lights", icon: "sun", title: "Lights (I)" },
  ];
  // What is drawn over the map, shown or hidden with one click.
  const LAYERS: { key: ViewToggle; icon: string; title: string }[] = [
    { key: "creatures", icon: "person", title: "Creatures" },
    { key: "spawns", icon: "target", title: "Spawn areas" },
    { key: "houses", icon: "home", title: "Houses" },
    { key: "special", icon: "shield", title: "Protection zones, no PvP and other areas" },
    { key: "zones", icon: "dashed", title: "Zones" },
    { key: "markers", icon: "flag", title: "Temples, house entrances and waypoints" },
    { key: "labels", icon: "text", title: "Names of creatures, houses, temples and waypoints" },
    { key: "coords", icon: "hash", title: "Position of the tile under the mouse" },
  ];
</script>

<div class="toolbar">
  <div class="grp">
    <button class="t-icon-btn" class:on={map.tool === "select" && !map.pasting} title="Select and move (S) · drag a box, Ctrl+click adds, drag the selection to move it" onclick={() => ((map.tool = "select"), (map.pasting = false))}>
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 2l9 6-4 1 2 5-2 1-2-5-3 3z" /></svg>
    </button>
    <button class="t-icon-btn" class:on={map.tool === "brush" && b?.kind !== "eraser"} title="Draw with the brush (B) · Ctrl erases, Shift+drag paints a rectangle" disabled={!b} onclick={() => (map.tool = "brush")}><Icon name="pencil" /></button>
    <button class="t-icon-btn" class:on={map.tool === "brush" && b?.kind === "eraser"} title="Eraser (E) · Ctrl also removes grounds" onclick={mapCommands.eraser}><Icon name="eraser" /></button>
    <button class="t-icon-btn" class:on={map.fill} title="Fill with the ground brush (F)" disabled={b?.kind !== "ground"} onclick={() => (map.fill = !map.fill)}>
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 8l5-5 5 5-5 5z M13 10c0 2 1 3 1 4a1 1 0 0 1-2 0c0-1 1-2 1-4" /></svg>
    </button>
  </div>
  <span class="t-vsep"></span>
  <div class="grp brush" title={b ? b.label : "No brush"}>
    {#if b}
      <span class="t-slot sm">
        {#if swatch}<span class="sw" style="background:{swatch}"></span>{:else}<ItemIcon id={b.look ?? b.item ?? 0} size={26} outfit={isCreature ? creatureOutfit(b.creature) : 0} />{/if}
      </span>
      <span class="label">{b.label}</span>
    {:else}
      <span class="t-label label">Pick a brush</span>
    {/if}
  </div>
  {#if sized}
    <span class="t-vsep"></span>
    <div class="grp" title="Brush size ([ ], Alt+wheel)">
      <span class="t-label">Size</span>
      <Slider bind:value={map.size} min={0} max={15} width={90} />
      <span class="val">{map.size * 2 + 1}</span>
      <button class="t-icon-btn" class:on={map.shape === "circle"} title="Round brush (Q)" onclick={() => (map.shape = map.shape === "square" ? "circle" : "square")}>
        {#if map.shape === "circle"}
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="5.5" /></svg>
        {:else}
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="2.5" y="2.5" width="11" height="11" /></svg>
        {/if}
      </button>
    </div>
  {/if}
  {#if walling && lastWall}
    {@const wall = lastWall}
    <span class="t-vsep"></span>
    <div class="m-seg doors" title="Doors and windows go into an existing wall">
      <button class:on={b?.kind === "wall"} title="The wall itself" onclick={() => selectBrush(wall)}>Wall</button>
      {#each DOORS as [type, label, title]}
        <button class:on={b?.kind === "door" && b.doorType === type} {title} onclick={() => selectBrush({ kind: "door", doorType: type, look: wall.look, label: `${label} in ${wall.label}` })}>{label}</button>
      {/each}
    </div>
  {/if}
  {#if isDoodad}
    <span class="t-vsep"></span>
    <div class="grp">
      <span class="t-label">Variation</span>
      <input class="t-input n" type="number" min="0" bind:value={map.variation} />
      <span class="t-label" title="Density of objects of a big brush (0 uses the brush's own)">Density</span>
      <Slider bind:value={density} min={0} max={100} width={70} />
      <button class="t-btn sm" title="New random placement" onclick={() => map.reroll++}>Reroll</button>
    </div>
  {/if}
  {#if isCreature || b?.kind === "spawn"}
    <span class="t-vsep"></span>
    <div class="grp">
      <span class="t-label">Spawn time</span>
      <input class="t-input n" type="number" min="0" placeholder={String(map.settings?.defaultSpawnTime ?? 60)} value={map.spawnTime || ""} onchange={(event) => (map.spawnTime = Math.max(0, Math.trunc(+event.currentTarget.value) || 0))} />
      {#if isCreature}
        <Select value={map.direction} width={76} options={[{ value: 0, label: "North" }, { value: 1, label: "East" }, { value: 2, label: "South" }, { value: 3, label: "West" }]} onchange={(value) => (map.direction = value)} />
      {/if}
    </div>
  {/if}
  <span class="grow"></span>
  <div class="grp">
    {#each LAYERS as l}
      <button class="t-icon-btn" class:on={map.view[l.key]} title="{map.view[l.key] ? 'Hide' : 'Show'}: {l.title}" onclick={() => (map.view[l.key] = !map.view[l.key])}><Icon name={l.icon} /></button>
    {/each}
  </div>
  <span class="t-vsep"></span>
  <div class="grp">
    {#each QUICK as q}
      <button class="t-icon-btn" class:on={map.view[q.key]} title={q.title} onclick={() => (map.view[q.key] = !map.view[q.key])}><Icon name={q.icon} /></button>
    {/each}
    {#if map.view.lights}
      <span class="t-label" title="Ambient light: how bright the map is away from lights">Ambient</span>
      <Slider bind:value={map.view.ambientLight} min={0} max={100} width={70} />
      <span class="val pct">{map.view.ambientLight}%</span>
    {/if}
    <button class="t-btn sm" class:on={!!map.settings?.automagic} title="Automatic borders and walls (A)" onclick={mapCommands.toggleAutomagic}>Auto</button>
    <button class="t-icon-btn" title="Play from here (P): walk the map as the client shows it" onclick={() => mapCommands.play()}><Icon name="play" /></button>
  </div>

</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 4px;
    flex: none;
    flex-wrap: nowrap;
    overflow: hidden;
  }
  .grp {
    display: flex;
    align-items: center;
    gap: 3px;
    flex: none;
  }
  .t-vsep {
    align-self: center;
    height: 18px;
  }
  .brush {
    min-width: 96px;
    flex: 0 1 auto;
    max-width: 200px;
  }
  .sw {
    width: 18px;
    height: 18px;
    border: 1px solid #000;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-bright);
  }
  .sm {
    transform: scale(0.85);
  }
  .t-btn.sm {
    transform: none;
    min-width: 0;
    padding: 0 6px;
  }
  .val {
    min-width: 18px;
    color: var(--text-bright);
  }
  .n {
    width: 46px;
  }
  .pct {
    min-width: 30px;
  }
  .doors {
    flex: none;
  }
  .doors > button {
    flex: none;
    font-size: 12px;
    padding: 0 5px;
    text-overflow: clip;
  }
</style>
