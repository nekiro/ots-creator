<script lang="ts">
  // What to draw with: brushes, items and creatures. The search field filters
  // the open tab; the tabs show how many of theirs match.
  import { untrack } from "svelte";
  import { errorMessage } from "../../lib/api";
  import { ask, askText } from "../../lib/confirm.svelte";
  import { openContextMenu } from "../../lib/menu.svelte";
  import { toast } from "../../lib/state/app.svelte";
  import { readJson, saveJson } from "../../lib/state/storage";
  import { MapService, SpawnFormat, type CreatureType, type PaletteBrush } from "../../lib/map/data/api";
  import { mapCommands } from "../../lib/map/view/commands";
  import { creatureOutfit, isBrushActive, map, reloadPalette, selectBrush, type BrushSel } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import SearchField from "../../lib/ui/SearchField.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import BrushPreview from "./BrushPreview.svelte";
  import ItemIcon from "./ItemIcon.svelte";

  type View = "terrain" | "decor" | "items" | "creatures";
  const VIEWS: { id: View; label: string; title: string }[] = [
    { id: "terrain", label: "Terrain", title: "Grounds, walls, carpets and tables: they join their neighbors" },
    { id: "decor", label: "Decor", title: "Doodads: trees, stones, furniture, scattered by the brush" },
    { id: "items", label: "Items", title: "Every item of the client, placed as is" },
    { id: "creatures", label: "Creatures", title: "Monsters and NPCs" },
  ];
  type ItemKind = "all" | "ground" | "edge" | "container" | "pickup" | "other";
  const ITEM_KINDS: { id: ItemKind; label: string; title: string }[] = [
    { id: "all", label: "All", title: "All items" },
    { id: "ground", label: "Ground", title: "Ground tiles" },
    { id: "edge", label: "On top", title: "Walls, borders and other items that stay on top" },
    { id: "container", label: "Boxes", title: "Containers" },
    { id: "pickup", label: "Loot", title: "Items players can pick up" },
    { id: "other", label: "Other", title: "Everything else: decoration, furniture" },
  ];
  const TERRAIN_GROUPS: { kinds: string[]; label: string }[] = [
    { kinds: ["ground"], label: "Grounds" },
    { kinds: ["wall", "decoration"], label: "Walls" },
    { kinds: ["carpet"], label: "Carpets" },
    { kinds: ["table"], label: "Tables" },
  ];

  const PREFS_KEY = "map:library";
  const prefs = loadPrefs();
  let view = $state<View>(prefs.view);
  let collection = $state<string>(prefs.collection);
  let itemKind = $state<ItemKind>("all");
  let who = $state<"all" | "monster" | "npc">("all");
  let query = $state("");

  function loadPrefs(): { view: View; collection: string } {
    const stored = (readJson(PREFS_KEY) ?? {}) as { view?: View; collection?: unknown };
    return { view: VIEWS.some((view) => view.id === stored.view) ? stored.view! : "terrain", collection: typeof stored.collection === "string" ? stored.collection : "" };
  }
  $effect(() => saveJson(PREFS_KEY, { view, collection }));

  const palette = $derived(map.palette);
  /** The item table and palette are on their way (none when the client has no map data). */
  const loadingData = $derived((map.data === null || map.data.ready) && (!map.items || !map.palette));
  const sets = $derived(palette?.tilesets?.filter((tileset) => !!tileset) ?? []);
  const set = $derived(sets.find((tileset) => tileset.name === collection) ?? null);
  const collections = $derived([{ value: "", label: "All collections" }, ...sets.map((tileset) => ({ value: tileset.name, label: tileset.name }))]);
  const allBrushes = $derived((palette?.brushes ?? []).filter((brush): brush is PaletteBrush => !!brush));
  const creatures = $derived((palette?.creatures ?? []).filter((creature): creature is CreatureType => !!creature));

  // ----- search -----
  const search = $derived(query.trim().toLowerCase());
  const named = <T extends { name: string }>(list: T[]) => (search ? list.filter((entry) => entry.name.toLowerCase().includes(search)) : list);

  // ----- terrain and decor -----
  const terrainAll = $derived(set ? ((set.terrain ?? []) as PaletteBrush[]) : allBrushes.filter((brush) => brush.kind !== "doodad"));
  const decorAll = $derived(set ? ((set.doodads ?? []) as PaletteBrush[]) : allBrushes.filter((brush) => brush.kind === "doodad"));
  const terrain = $derived(named(terrainAll));
  const decor = $derived(named(decorAll));

  // ----- items -----
  const collectionItems = $derived.by<number[]>(() => {
    if (set) return [...new Set([...(set.items ?? []), ...(set.raw ?? [])])];
    return map.items?.ids ?? [];
  });
  // Names, ids and id ranges ("100-200") match, as in the item search.
  const baseItems = $derived.by<number[]>(() => {
    if (!search || !map.items) return collectionItems;
    const hits = map.items.search(search, Infinity);
    if (!set) return hits;
    const hit = new Set(hits);
    return collectionItems.filter((id) => hit.has(id));
  });
  const items = $derived.by<number[]>(() => {
    const itemIndex = map.items;
    if (!itemIndex || itemKind === "all") return baseItems;
    return baseItems.filter((id) => {
      const ground = itemIndex.is(id, "ground");
      const edge = !ground && itemIndex.top[id] > 0;
      switch (itemKind) {
        case "ground":
          return ground;
        case "edge":
          return edge;
        case "container":
          return itemIndex.is(id, "container");
        case "pickup":
          return itemIndex.is(id, "pickupable") && !itemIndex.is(id, "container");
        default:
          return !ground && !edge && !itemIndex.is(id, "pickupable") && !itemIndex.is(id, "container");
      }
    });
  });
  /** A kind tab that lists the item (an item can be in more than one). */
  function kindOf(id: number): Exclude<ItemKind, "all"> {
    const itemIndex = map.items;
    if (!itemIndex) return "other";
    if (itemIndex.is(id, "ground")) return "ground";
    if (itemIndex.top[id] > 0) return "edge";
    if (itemIndex.is(id, "container")) return "container";
    if (itemIndex.is(id, "pickupable")) return "pickup";
    return "other";
  }

  // An item picked from the map: open the items tab on its kind, drop the
  // filters that hide it and scroll to it while it stays the brush.
  let revealed = $state<number | null>(null);
  $effect(() => {
    const request = map.revealItem;
    if (!request) return;
    untrack(() => {
      map.revealItem = null;
      view = "items";
      itemKind = kindOf(request.id);
      if (!items.includes(request.id)) query = "";
      if (!items.includes(request.id)) collection = "";
      revealed = request.id;
    });
  });
  const revealIndex = $derived.by(() => {
    if (revealed === null || map.brush?.kind !== "raw" || map.brush.item !== revealed) return null;
    const index = items.indexOf(revealed);
    return index >= 0 ? index : null;
  });

  // ----- creatures -----
  const matchedCreatures = $derived(named(creatures));
  const shownCreatures = $derived(matchedCreatures.filter((creature) => who === "all" || (who === "npc") === !!creature.npc));
  const separate = $derived(map.active?.props.spawnFormat === SpawnFormat.SpawnsSeparate);

  // Matches per tab while searching.
  const counts = $derived<Record<View, number>>({ terrain: terrain.length, decor: decor.length, items: baseItems.length, creatures: matchedCreatures.length });

  function brushSel(brush: PaletteBrush): BrushSel {
    const kind = brush.kind === "decoration" ? "wall" : brush.kind;
    return { kind, name: brush.name, look: brush.look, label: brush.name };
  }
  function itemSel(id: number): BrushSel {
    return { kind: "raw", item: id, look: id, label: map.items?.name(id) ?? `item ${id}` };
  }
  function creatureSel(creature: CreatureType): BrushSel {
    return { kind: "creature", creature: creature.name, npc: !!creature.npc, label: creature.name };
  }

  function editBrush(brush: PaletteBrush) {
    map.editBrush = { name: brush.name, type: brush.kind };
    map.dialog = "brush";
  }

  async function removeBrush(brush: PaletteBrush) {
    if (!(await ask({ title: "Remove brush", message: `Remove the brush "${brush.name}"? Maps keep their items.`, ok: "Remove" }))) return;
    try {
      await MapService.RemoveBrush(brush.name);
      await reloadPalette();
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  function brushMenu(event: MouseEvent, brush: PaletteBrush) {
    openContextMenu(event, [
      { label: "Edit brush…", action: () => editBrush(brush) },
      { label: "Remove brush", action: () => removeBrush(brush) },
    ]);
  }

  function itemMenu(event: MouseEvent, id: number) {
    openContextMenu(event, [
      { label: "Find on the map", action: () => mapCommands.find({ ids: [id] }) },
      { label: "Replace on the map…", action: () => mapCommands.find({ ids: [id], replace: true }) },
      "-",
      { label: "Add to a collection…", action: () => addToCollection(id) },
      { label: "Copy id", action: () => navigator.clipboard.writeText(String(id)) },
    ]);
  }

  async function addToCollection(id: number) {
    const name = await askText({ title: "Add to a collection", label: "Collection", value: set && !set.auto ? set.name : "My items", ok: "Add" });
    if (!name) return;
    try {
      await MapService.AddToTileset(name, [id]);
      await reloadPalette();
      toast(`Added to ${name}.`, "success");
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  function newBrush() {
    map.editBrush = { name: "", type: view === "decor" ? "doodad" : "ground" };
    map.dialog = "brush";
  }

  const ITEM_CELL = 36;
</script>

{#snippet brushRow(brush: PaletteBrush)}
  {@const choice = brushSel(brush)}
  <button class="m-row" class:on={isBrushActive(choice)} title="{brush.name} · double-click to edit" onclick={() => selectBrush(choice)} ondblclick={() => editBrush(brush)} oncontextmenu={(event) => brushMenu(event, brush)}>
    <span class="t-slot"><ItemIcon id={brush.look} size={24} /></span>
    <span class="m-name">{brush.name}</span>
  </button>
{/snippet}

<!-- Groups of tiles (stamps): shown whole in a grid of cards, an icon tells too little. -->
{#snippet stampCards(list: PaletteBrush[])}
  {#if list.length}
    <div class="m-sect">Groups<span class="grow"></span>{list.length}</div>
    <div class="stamps">
      {#each list as brush (brush.name)}
        {@const choice = brushSel(brush)}
        <button class="stamp" class:on={isBrushActive(choice)} title="{brush.name} · {brush.preview?.w}×{brush.preview?.h} tiles · double-click to edit" onclick={() => selectBrush(choice)} ondblclick={() => editBrush(brush)} oncontextmenu={(event) => brushMenu(event, brush)}>
          {#if brush.preview}<BrushPreview preview={brush.preview} />{/if}
          <span class="m-name">{brush.name}</span>
        </button>
      {/each}
    </div>
  {/if}
{/snippet}

{#snippet itemCell(id: number)}
  {@const choice = itemSel(id)}
  <button class="m-cell" class:on={isBrushActive(choice)} title="{map.items?.name(id)} · {id}" onclick={() => selectBrush(choice)} oncontextmenu={(event) => itemMenu(event, id)}>
    <span class="t-slot"><ItemIcon {id} /></span>
  </button>
{/snippet}

{#snippet creatureRow(creature: CreatureType)}
  {@const choice = creatureSel(creature)}
  <button class="m-row" class:on={isBrushActive(choice)} onclick={() => selectBrush(choice)}>
    <span class="t-slot"><ItemIcon outfit={creature.outfit.lookItem ? 0 : creature.outfit.lookType} id={creature.outfit.lookItem} size={24} /></span>
    <span class="m-name">{creature.name}</span>
    {#if creature.npc}<span class="m-meta">NPC</span>{/if}
  </button>
{/snippet}

{#snippet noMatch()}
  <div class="m-empty">
    <span>Nothing here matches “{query}”.</span>
    {#if set}<button class="t-btn" onclick={() => (collection = "")}>Search all collections</button>{/if}
  </div>
{/snippet}

{#snippet noBrushes()}
  <div class="m-empty">
    <span>No brushes yet. Brushes place borders, connect walls and scatter decoration for you.</span>
    <button class="t-btn primary" onclick={() => (map.dialog = "data")}>Import brushes…</button>
    <span class="small">or <button class="link" onclick={newBrush}>make one</button>; every item is in Items.</span>
  </div>
{/snippet}

<div class="lib">
  <SearchField bind:value={query} placeholder="Search by name or id" />

  <div class="t-tabs views" role="tablist">
    {#each VIEWS as tab}
      <button class="t-tab" role="tab" class:on={view === tab.id} aria-selected={view === tab.id} title={tab.title} onclick={() => (view = tab.id)}>{tab.label}{#if search}<span class="count" class:none={!counts[tab.id]}>{counts[tab.id]}</span>{/if}</button>
    {/each}
  </div>

  {#if view !== "creatures" && sets.length}
    <div class="bar" title="Collections are the tilesets of your brush data; the tabs below filter items by kind">
      <span class="t-label dim">Collection</span>
      <Select grow value={collection} options={collections} maxItems={16} onchange={(value) => (collection = value)} />
      {#if view !== "items"}<button class="t-icon-btn" title="New brush" onclick={newBrush}><Icon name="plus" /></button>{/if}
    </div>
  {/if}

  {#if loadingData}
    <div class="m-body t-panel">
      <div class="m-empty" role="status"><span class="busy"><span class="t-spinner"></span>Loading items…</span></div>
    </div>
  {:else if view === "terrain"}
    <div class="m-body t-panel m-scroll">
      {#each TERRAIN_GROUPS as group}
        {@const list = terrain.filter((brush) => group.kinds.includes(brush.kind))}
        {#if list.length}
          <div class="m-sect">{group.label}</div>
          {#each list as brush (brush.name)}{@render brushRow(brush)}{/each}
        {/if}
      {/each}
      {#if !terrain.length}
        {#if search}{@render noMatch()}{:else if allBrushes.length}<div class="m-empty">No terrain in this collection.</div>{:else}{@render noBrushes()}{/if}
      {:else if !search}
        {@const optionalBorder = { kind: "optionalBorder", label: "Optional border" }}
        <div class="m-sect">Tools</div>
        <button class="m-row" class:on={isBrushActive(optionalBorder)} title="Adds the optional border of a ground (like the gravel around mountains) where you paint" onclick={() => selectBrush(optionalBorder)}>
          <span class="m-swatch opt"></span><span class="m-name">Optional border</span>
        </button>
      {/if}
    </div>
  {:else if view === "decor"}
    <div class="m-body t-panel m-scroll">
      {#each decor.filter((brush) => !brush.preview) as brush (brush.name)}{@render brushRow(brush)}{/each}
      {@render stampCards(decor.filter((brush) => !!brush.preview))}
      {#if !decor.length}
        {#if search}{@render noMatch()}{:else if allBrushes.length}<div class="m-empty">No decoration in this collection.</div>{:else}{@render noBrushes()}{/if}
      {/if}
    </div>
  {:else if view === "items"}
    <div class="t-tabs kinds" role="tablist">
      {#each ITEM_KINDS as kindTab}
        <button class="t-tab" role="tab" class:on={itemKind === kindTab.id} aria-selected={itemKind === kindTab.id} title={kindTab.title} onclick={() => (itemKind = kindTab.id)}>{kindTab.label}</button>
      {/each}
    </div>
    <div class="m-body t-panel grid-host">
      {#if items.length}
        <VirtualGrid count={items.length} cellWidth={ITEM_CELL} cellHeight={ITEM_CELL} gap={2} scrollTo={revealIndex}>
          {#snippet cell(i)}{@render itemCell(items[i])}{/snippet}
        </VirtualGrid>
      {:else if search}
        {@render noMatch()}
      {:else}
        <div class="m-empty">No items of this kind here.</div>
      {/if}
    </div>
  {:else}
    <div class="t-tabs kinds" role="tablist">
      <button class="t-tab" role="tab" class:on={who === "all"} aria-selected={who === "all"} onclick={() => (who = "all")}>All</button>
      <button class="t-tab" role="tab" class:on={who === "monster"} aria-selected={who === "monster"} onclick={() => (who = "monster")}>Monsters</button>
      <button class="t-tab" role="tab" class:on={who === "npc"} aria-selected={who === "npc"} onclick={() => (who = "npc")}>NPCs</button>
    </div>
    <div class="m-body t-panel m-scroll">
      {#if !search}
        <div class="m-sect">Spawn areas</div>
        <button class="m-row" class:on={isBrushActive({ kind: "spawn", label: "" })} title="Drag on the map: the brush size is the radius" onclick={() => selectBrush({ kind: "spawn", label: "Spawn area" })}>
          <span class="m-swatch spawn"></span><span class="m-name">{separate ? "Monster spawn area" : "Spawn area"}</span>
        </button>
        {#if separate}
          <button class="m-row" class:on={isBrushActive({ kind: "spawnNpc", label: "" })} onclick={() => selectBrush({ kind: "spawnNpc", label: "NPC spawn area" })}>
            <span class="m-swatch npc"></span><span class="m-name">NPC spawn area</span>
          </button>
        {/if}
      {/if}
      {#if shownCreatures.length}
        <div class="m-sect">{who === "npc" ? "NPCs" : who === "monster" ? "Monsters" : "Creatures"}<span class="grow"></span>{shownCreatures.length}</div>
        {#each shownCreatures as creature (creature.name)}{@render creatureRow(creature)}{/each}
      {:else if search}
        {@render noMatch()}
      {:else}
        <div class="m-empty">
          <span>No creatures yet. They are read from the monster and NPC files of your server.</span>
          <button class="t-btn" onclick={() => (map.dialog = "data")}>Choose the server folder…</button>
        </div>
      {/if}
    </div>
  {/if}

  {#if map.recentBrushes.length}
    <div class="recent" title="Recently used: keys 1-9">
      {#each map.recentBrushes.slice(0, 9) as brush, i}
        <button class="m-cell small" class:on={isBrushActive(brush)} title="{i + 1}: {brush.label}" onclick={() => selectBrush(brush)}>
          <span class="t-slot"><ItemIcon id={brush.look ?? brush.item ?? 0} outfit={brush.kind === "creature" ? creatureOutfit(brush.creature) : 0} /></span>
          <span class="key">{i + 1}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .lib {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .stamps {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
    gap: 4px;
    padding: 2px;
  }
  .stamp {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-end;
    gap: 3px;
    min-width: 0;
    padding: 4px;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid transparent;
    color: var(--text);
    font: inherit;
    cursor: pointer;
  }
  .stamp:hover {
    background: rgba(255, 255, 255, 0.06);
  }
  .stamp.on {
    background: var(--select);
    border-color: #b6b6b6;
    color: #fff;
  }
  .stamp .m-name {
    flex: none;
    max-width: 100%;
    font-size: 12px;
    text-align: center;
  }
  .count {
    margin-left: 4px;
    color: var(--gold);
    font: var(--fs-small) var(--font-small);
  }
  .count.none {
    color: var(--text-dim);
  }
  .bar {
    display: flex;
    gap: 4px;
    align-items: center;
  }
  .kinds > button,
  .views > button {
    font-size: 12px;
    padding: 0 2px;
  }
  .dim {
    color: var(--text-dim);
    font-size: 12px;
  }
  .grid-host {
    display: flex;
  }
  .recent {
    display: flex;
    gap: 1px;
    flex: none;
    overflow: hidden;
  }
  .recent .small {
    width: 30px;
    height: 34px;
  }
  .key {
    position: absolute;
    right: 1px;
    bottom: 0;
    font: var(--fs-small) / 10px var(--font-small);
    color: var(--text-dim);
    text-shadow: 1px 1px 0 #000;
  }
  .small {
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
  .opt {
    background: #a87;
  }
  .spawn {
    background: rgba(255, 90, 90, 0.35);
    border-color: #f66;
  }
  .npc {
    background: rgba(90, 160, 255, 0.35);
    border-color: #6af;
  }
  .busy {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
