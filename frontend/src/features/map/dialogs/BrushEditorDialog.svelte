<script lang="ts">
  import { errorMessage } from "../../../lib/api";
  import { toast } from "../../../lib/state/app.svelte";
  import { MapService, type Border, type Brush, type BorderRule, type Chance } from "../../../lib/map/data/api";
  import { loadMapData, map, reloadPalette, selectBrush } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";
  import Icon from "../../../lib/ui/Icon.svelte";
  import NumberField from "../../../lib/ui/NumberField.svelte";
  import Select from "../../../lib/ui/Select.svelte";
  import ItemIcon from "../ItemIcon.svelte";
  import ItemPicker from "../ItemPicker.svelte";
  import TagPicker from "../../../lib/ui/TagPicker.svelte";
  import { isTyping } from "../../../lib/ui/typing";
  import BrushPreview from "../BrushPreview.svelte";
  import { frameOf, previewOf, type Frame } from "../../../lib/map/editor/brushpreview";
  import { MAIN_WALLS, MORE_WALLS, chances, collectionsContaining, ensureBrushShape, missingForSave, normalizeBrushForSave, stampOf } from "../../../lib/map/editor/brushEdit";

  // A new brush starts by choosing what it paints; the form then shows only
  // what that kind needs, in order, with the rarely changed parts folded.
  const TYPES = [
    { value: "ground", label: "Ground", desc: "Grass, sand, floors. Borders join it to the grounds around it." },
    { value: "wall", label: "Wall", desc: "Walls that connect by themselves, with doors and windows." },
    { value: "doodad", label: "Doodad", desc: "Trees, stones, furniture: scattered or placed as a group." },
    { value: "carpet", label: "Carpet", desc: "Fills an area with matching edges and corners." },
    { value: "table", label: "Table", desc: "Pieces that join into long tables." },
    { value: "decoration", label: "Wall decoration", desc: "Things hung on walls, following the wall shape." },
  ];
  // Visual layout of border and carpet edges: [edge, column, row].
  const EDGE_GRID: [string, number, number][] = [
    ["cnw", 0, 0], ["n", 1, 0], ["cne", 2, 0],
    ["w", 0, 1], ["e", 2, 1],
    ["csw", 0, 2], ["s", 1, 2], ["cse", 2, 2],
    ["dnw", 4, 0], ["dne", 5, 0],
    ["dsw", 4, 1], ["dse", 5, 1],
  ];
  const EDGE_LABEL: Record<string, string> = {
    n: "North", e: "East", s: "South", w: "West", cnw: "Corner NW", cne: "Corner NE", csw: "Corner SW", cse: "Corner SE",
    dnw: "Diagonal NW", dne: "Diagonal NE", dsw: "Diagonal SW", dse: "Diagonal SE", c: "Center",
  };
  const DOOR_TYPES = ["normal", "locked", "quest", "magic", "archway", "window", "hatch window"];
  const TABLE_PARTS = ["north", "south", "east", "west", "horizontal", "vertical", "alone"];

  const target = map.editBrush ?? { name: "", type: "ground" };
  const fromSelection = target.type === "fromSelection";
  const isNew = !target.name;

  let step = $state<"type" | "edit">(isNew && !fromSelection ? "type" : "edit");
  let brush = $state<Brush>({ name: "", type: target.type === "fromSelection" ? "doodad" : target.type } as Brush);
  let borders = $state<Border[]>([]);
  /** Collections the brush is in; edited brushes start with their current ones. */
  let collections = $state<string[]>(isNew ? [] : collectionsContaining(map.palette?.tilesets, target.name));
  let loading = $state(!isNew);
  let saving = $state(false);
  let moreWalls = $state(false);
  /** Border being edited and its selected edge. */
  let editBorder = $state<Border | null>(null);
  let edge = $state("n");

  $effect(() => {
    void MapService.Borders().then((loaded) => (borders = (loaded ?? []).filter((border): border is Border => !!border)));
    if (!isNew) {
      void MapService.Brush(target.name).then(
        (loaded) => {
          brush = JSON.parse(JSON.stringify(loaded));
          loading = false;
        },
        (err) => {
          toast(errorMessage(err), "error");
          map.dialog = null;
        },
      );
    }
  });

  const brushNames = $derived((map.palette?.brushes ?? []).filter((other) => !!other && other.name !== brush.name).map((other) => other.name));
  const tilesets = $derived((map.palette?.tilesets ?? []).filter((tileset) => !!tileset && !tileset.auto).map((tileset) => tileset.name));
  const borderOptions = $derived([{ value: 0, label: "None" }, ...borders.map((border) => ({ value: border.id, label: `Border ${border.id}${border.group ? ` (group ${border.group})` : ""}` }))]);

  function newBorder(): Border {
    return { id: 0, edges: {}, group: 0 } as Border;
  }

  async function saveBorder(border: Border): Promise<number> {
    const id = await MapService.SaveBorder($state.snapshot(border) as Border);
    borders = (await MapService.Borders())?.filter((saved): saved is Border => !!saved) ?? borders;
    return id;
  }

  const typeLabel = $derived(TYPES.find((type) => type.value === brush.type)?.label ?? brush.type);
  /** A brush made from a selection: one group of tiles and nothing else. */
  const stamp = $derived(stampOf(brush));

  // Editing a stamp: chosen tiles in preview coordinates. The frame is taken
  // once, so the preview keeps its place when edge tiles go (placement
  // offsets never change).
  type TilePoint = { x: number; y: number };
  let picked = $state<TilePoint[]>([]);
  let frame = $state<Frame | null>(null);
  $effect(() => {
    if (stamp && !frame) frame = frameOf(stamp.tiles);
  });
  const tileAt = (point: TilePoint) =>
    (stamp?.tiles ?? []).find((tile) => !!tile && !tile.z && frame && tile.x === frame.x + point.x && tile.y === frame.y + point.y) ?? null;
  type StampTile = NonNullable<ReturnType<typeof tileAt>>;
  function selectTiles(tiles: TilePoint[], add: boolean) {
    picked = add ? [...picked, ...tiles.filter((tile) => !picked.some((point) => point.x === tile.x && point.y === tile.y))] : tiles;
  }
  function removeItem(tile: StampTile, itemIndex: number) {
    tile.items = (tile.items ?? []).filter((_, i) => i !== itemIndex);
    if (!tile.items.length && stamp) stamp.tiles = (stamp.tiles ?? []).filter((other) => other !== tile);
  }
  /** Clears the chosen tiles: the map keeps what is there. */
  function clearPicked() {
    if (!stamp || !picked.length) return;
    const gone = new Set(picked.map(tileAt).filter((tile) => !!tile));
    stamp.tiles = (stamp.tiles ?? []).filter((tile) => !gone.has(tile));
    picked = [];
  }
  function onkeydown(event: KeyboardEvent) {
    if (!stamp || !picked.length || (event.key !== "Delete" && event.key !== "Backspace")) return;
    if (isTyping(event)) return;
    event.preventDefault();
    clearPicked();
  }

  function chooseType(type: string) {
    brush.type = type;
    step = "edit";
  }

  /** What still stops saving, told in the footer. */
  const missing = $derived(missingForSave(brush, fromSelection));

  function save() {
    if (fromSelection) void saveDoodadFromSelection();
    else void saveBrush();
  }

  async function saveDoodadFromSelection() {
    const name = brush.name.trim();
    if (!name || map.activeId === null) return;
    saving = true;
    try {
      await MapService.DoodadFromSelection(map.activeId, name, $state.snapshot(collections));
      await reloadPalette();
      await loadMapData(true);
      selectBrush({ kind: "doodad", name, label: name });
      toast(`Doodad "${brush.name}" created. Click the map to place it.`, "success");
      map.dialog = null;
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      saving = false;
    }
  }

  async function saveBrush() {
    saving = true;
    try {
      if (editBorder) {
        await saveBorder(editBorder);
        editBorder = null;
      }
      const saved = normalizeBrushForSave($state.snapshot(brush) as Brush);
      await MapService.SaveBrush(isNew ? "" : target.name, saved, $state.snapshot(collections));
      await reloadPalette();
      await loadMapData(true);
      toast(`Saved "${saved.name}".`, "success");
      map.dialog = null;
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      saving = false;
    }
  }

  // ----- ground -----
  function addRule() {
    brush.borders = [...(brush.borders ?? []), { align: "outer", to: "all", border: 0 } as BorderRule];
  }

  function createBorderFor(ruleIndex: number) {
    editBorder = { ...newBorder() };
    pendingRule = ruleIndex;
  }
  let pendingRule = $state<number | null>(null);

  async function finishBorder() {
    if (!editBorder) return;
    const id = await saveBorder(editBorder);
    if (pendingRule !== null && brush.borders?.[pendingRule]) brush.borders[pendingRule].border = id;
    if (pendingRule === -1) brush.optional = { align: "outer", border: id } as BorderRule;
    editBorder = null;
    pendingRule = null;
  }

  function borderById(id: number | undefined): Border | undefined {
    return borders.find((border) => border.id === id);
  }

  // Walls and doodads edit nested objects: make sure they exist.
  $effect(() => {
    if (loading) return;
    ensureBrushShape(brush);
  });

  // ----- carpet and table -----
  function part(align: string): Chance[] {
    return chances(brush.parts?.[align]);
  }
  function setPart(align: string, id: number) {
    brush.parts ??= {};
    brush.parts[align] = id ? [{ id, chance: 10 }] : [];
  }
</script>

{#snippet chanceList(list: Chance[], set: (list: Chance[]) => void, addLabel = "Add item")}
  {#each list as chance, i}
    <div class="row">
      <ItemPicker value={chance.id} width={250} onchange={(id) => set(list.map((entry, k) => (k === i ? { ...entry, id } : entry)))} />
      <span class="t-label">chance</span>
      <NumberField value={chance.chance} min={0} max={100000} width={50} onchange={(value) => set(list.map((entry, k) => (k === i ? { ...entry, chance: value } : entry)))} />
      <button class="t-icon-btn danger" title="Remove" onclick={() => set(list.filter((_, k) => k !== i))}><Icon name="close" /></button>
    </div>
  {/each}
  <button class="t-btn small" class:primary={!list.length} onclick={() => set([...list, { id: 0, chance: 10 }])}>{addLabel}</button>
{/snippet}

{#snippet head(step: number, title: string, hint = "")}
  <div class="sh"><span class="st">{step}. {title}</span>{#if hint}<span class="t-label hint">{hint}</span>{/if}</div>
{/snippet}

{#snippet edgeGrid(get: (edgeName: string) => number, selected: string, onselect: (edgeName: string) => void, withCenter = false)}
  <div class="edges">
    {#each EDGE_GRID as [edgeName, column, row]}
      <button class="slot t-slot" class:on={selected === edgeName} style="grid-column:{column + 1};grid-row:{row + 1}" title={EDGE_LABEL[edgeName]} onclick={() => onselect(edgeName)}>
        <ItemIcon id={get(edgeName)} />
        <span class="el">{edgeName}</span>
      </button>
    {/each}
    {#if withCenter}
      <button class="slot t-slot" class:on={selected === "c"} style="grid-column:2;grid-row:2" title="Center" onclick={() => onselect("c")}>
        <ItemIcon id={get("c")} /><span class="el">c</span>
      </button>
    {/if}
  </div>
{/snippet}

<svelte:window {onkeydown} />

<Dialog title={fromSelection ? "Doodad from selection" : isNew ? (step === "type" ? "New brush: what does it paint?" : `New ${typeLabel.toLowerCase()} brush`) : `Brush: ${target.name}`} width={fromSelection ? 400 : 640} onclose={() => (map.dialog = null)}>
  {#if loading}
    <p class="t-label">Loading…</p>
  {:else if step === "type"}
    <div class="types">
      {#each TYPES as type}
        <button class="type" onclick={() => chooseType(type.value)}>
          <span class="tname">{type.label}</span>
          <span class="tdesc">{type.desc}</span>
        </button>
      {/each}
    </div>
  {:else}
    <div class="col form">
      {@render head(1, "Name")}
      <div class="row">
        <!-- svelte-ignore a11y_autofocus -->
        <input class="t-input grow" bind:value={brush.name} placeholder={brush.type === "ground" ? "e.g. grass" : brush.type === "wall" ? "e.g. stone wall" : "e.g. pine trees"} autofocus />
        {#if isNew && !fromSelection}
          <span class="kind t-label">{typeLabel} · <button class="link" onclick={() => (step = "type")}>change</button></span>
        {/if}
      </div>

      {#if fromSelection}
        <p class="t-label hint">The selected items become one doodad, placed with its top-left tile at the cursor.</p>
        {#if isNew}
          <div class="row"><span class="t-label lbl">Collections</span><div class="grow"><TagPicker plain allowCustom value={collections} options={tilesets.map((tileset) => ({ tag: tileset }))} placeholder="none · type to add or create" onchange={(chosen) => (collections = chosen)} /></div></div>
        {/if}
      {:else}
        {#if brush.type === "ground"}
          {@render head(2, "Ground items", "One is picked at random for each tile; chance weighs the pick.")}
          {@render chanceList(chances(brush.items), (list) => (brush.items = list))}

          {@render head(3, "Borders", "Optional: the edges drawn where this ground meets others.")}
          {#each brush.borders ?? [] as rule, i}
            {#if rule}
              {@const ruleBorder = borderById(rule.border)}
              <div class="rule">
                <Select value={rule.align} width={100} options={[{ value: "outer", label: "Outside" }, { value: "inner", label: "Inside" }]} onchange={(value) => (rule.align = value)} />
                <span class="t-label">next to</span>
                <Select value={rule.to || "all"} width={130} maxItems={14} options={[{ value: "all", label: "Everything" }, { value: "none", label: "Empty tiles" }, ...brushNames.map((name) => ({ value: name, label: name }))]} onchange={(value) => (rule.to = value)} />
                <Select value={rule.border ?? 0} width={150} maxItems={14} options={borderOptions} onchange={(value) => (rule.border = value)} />
                <span class="mini">{#each ["n", "cnw", "dnw"] as edgeName}<ItemIcon id={ruleBorder?.edges?.[edgeName] ?? 0} size={20} />{/each}</span>
                <button class="t-icon-btn" title="Edit this border" disabled={!ruleBorder} onclick={() => ((editBorder = ruleBorder ? JSON.parse(JSON.stringify(ruleBorder)) : null), (pendingRule = null))}><Icon name="pencil" /></button>
                <button class="t-icon-btn" title="New border for this rule" onclick={() => createBorderFor(i)}><Icon name="plus" /></button>
                <button class="t-icon-btn danger" title="Remove rule" onclick={() => (brush.borders = (brush.borders ?? []).filter((_, k) => k !== i))}><Icon name="close" /></button>
              </div>
            {/if}
          {/each}
          <div class="row">
            <button class="t-btn small" onclick={addRule}>Add border</button>
            {#if (brush.borders ?? []).length}<span class="t-label hint">Outside: drawn on the neighbor tiles. Inside: drawn on this ground.</span>{/if}
          </div>
        {:else if brush.type === "wall" || brush.type === "decoration"}
          {@render head(2, "Pieces", "Walls connect by themselves. Horizontal and vertical are needed; corner and pole make them look right.")}
          {#each (moreWalls ? [...MAIN_WALLS, ...MORE_WALLS] : MAIN_WALLS).filter((wallAlign) => !!brush.walls?.[wallAlign]) as align}
            {@const wall = brush.walls![align]!}
            <details class="wall" open={MAIN_WALLS.includes(align) && !wall.items?.length}>
              <summary
                ><span class="cap">{align}</span> <span class="t-label">{chances(wall.items).length} items · {(wall.doors ?? []).length} doors</span>
                <span class="mini">{#each chances(wall.items).slice(0, 4) as chance}<ItemIcon id={chance.id} size={20} />{/each}</span></summary
              >
              {@render chanceList(chances(wall.items), (list) => (wall.items = list))}
              <div class="t-label">Doors and windows in this piece</div>
              {#each wall.doors ?? [] as door, i}
                {#if door}
                  <div class="row">
                    <ItemPicker value={door.id} width={220} onchange={(id) => (door.id = id)} />
                    <Select value={door.type} width={110} options={DOOR_TYPES.map((type) => ({ value: type, label: type }))} onchange={(value) => (door.type = value)} />
                    <label class="row"><input class="t-check" type="checkbox" bind:checked={door.open} />open</label>
                    <button class="t-icon-btn danger" title="Remove" onclick={() => (wall.doors = (wall.doors ?? []).filter((_, k) => k !== i))}><Icon name="close" /></button>
                  </div>
                {/if}
              {/each}
              <button class="t-btn small" onclick={() => (wall.doors = [...(wall.doors ?? []), { id: 0, type: "normal", open: false }])}>Add door</button>
            </details>
          {/each}
          <button class="link" onclick={() => (moreWalls = !moreWalls)}>{moreWalls ? "Fewer shapes" : "More shapes: ends, T pieces, diagonals…"}</button>
        {:else if brush.type === "carpet"}
          {@render head(2, "Pieces", "Click a position, then pick its item. The center fills the inside.")}
          <div class="row top">
            {@render edgeGrid((edgeName) => part(edgeName)[0]?.id ?? 0, edge, (edgeName) => (edge = edgeName), true)}
            <div class="col">
              <span class="t-label">{EDGE_LABEL[edge]}</span>
              <ItemPicker value={part(edge)[0]?.id ?? 0} width={240} onchange={(id) => setPart(edge, id)} />
            </div>
          </div>
        {:else if brush.type === "table"}
          {@render head(2, "Pieces", "Alone is a single table; the others join their neighbors.")}
          {#each ["alone", ...TABLE_PARTS.filter((tablePart) => tablePart !== "alone")] as tablePart}
            <div class="row"><span class="t-label lbl cap">{tablePart}</span><ItemPicker value={part(tablePart)[0]?.id ?? 0} width={280} onchange={(id) => setPart(tablePart, id)} /></div>
          {/each}
        {:else if brush.type === "doodad" && stamp}
          {@const stampPreview = previewOf(stamp.tiles, frame)}
          {@const pickedTile = picked.length === 1 ? tileAt(picked[0]) : null}
          {@render head(2, "Group", `${(stamp.tiles ?? []).length} tiles, placed whole with the top-left tile at the cursor.`)}
          <div class="row top">
            {#if stampPreview}<div class="stamp"><BrushPreview preview={stampPreview} max={24} selected={picked} onselect={selectTiles} /></div>{/if}
            <div class="col tilebox">
              {#if !picked.length}
                <span class="t-label hint">Click a tile to see its items, or drag a box over several. Ctrl adds to the choice; Delete clears the chosen tiles.</span>
              {:else if picked.length > 1}
                <span class="t-label">{picked.length} tiles chosen</span>
                <button class="t-btn small danger" onclick={clearPicked}>Clear {picked.length} tiles</button>
                <span class="t-label hint">Or press Delete. Cleared tiles keep what the map has there.</span>
              {:else}
                <span class="t-label">Tile {picked[0].x + 1}, {picked[0].y + 1}</span>
                {#if pickedTile && (pickedTile.items ?? []).some((item) => !!item)}
                  <div class="t-panel stack">
                    {#each [...(pickedTile.items ?? [])].reverse() as item, reverseIndex}
                      {#if item}
                        {@const itemIndex = (pickedTile.items ?? []).length - 1 - reverseIndex}
                        <div class="m-row">
                          <span class="t-slot"><ItemIcon id={item.id} size={24} /></span>
                          <span class="m-name" title="Item {item.id}">{map.items?.name(item.id) ?? `item ${item.id}`}</span>
                          <button class="t-icon-btn danger" title="Remove from this tile" onclick={() => removeItem(pickedTile, itemIndex)}><Icon name="close" /></button>
                        </div>
                      {/if}
                    {/each}
                  </div>
                  <button class="t-btn small danger" onclick={clearPicked}>Clear tile</button>
                {:else}
                  <span class="t-label hint">Empty: the map keeps what is there.</span>
                {/if}
              {/if}
            </div>
          </div>
          <span class="t-label hint">Tiles with a ground replace the map tile; the others are added on top of it.</span>
        {:else if brush.type === "doodad"}
          {@render head(2, "Items", "One is picked at random each time you place the brush.")}
          {#each brush.alternatives ?? [] as alternative, alternativeIndex}
            {#if alternative}
              {#if (brush.alternatives ?? []).length > 1}<div class="t-label">Variation {alternativeIndex + 1}</div>{/if}
              {#each alternative.singles ?? [] as single, singleIndex}
                {#if single}
                  <div class="row">
                    <ItemPicker value={single.item.id} width={250} onchange={(id) => (single.item.id = id)} />
                    <span class="t-label">chance</span><NumberField value={single.chance} min={0} max={100000} width={50} onchange={(value) => (single.chance = value)} />
                    <button class="t-icon-btn danger" title="Remove" onclick={() => (alternative.singles = (alternative.singles ?? []).filter((_, k) => k !== singleIndex))}><Icon name="close" /></button>
                  </div>
                {/if}
              {/each}
              {#each alternative.composites ?? [] as composite, compositeIndex}
                {#if composite}
                  {@const compositePreview = previewOf(composite.tiles)}
                  <div class="row comp">
                    {#if compositePreview}<span class="cpv"><BrushPreview preview={compositePreview} max={6} /></span>{/if}
                    <span class="t-label">group of {(composite.tiles ?? []).length} tiles · chance</span>
                    <NumberField value={composite.chance} min={0} max={100000} width={50} onchange={(value) => (composite.chance = value)} />
                    <button class="t-icon-btn danger" title="Remove group" onclick={() => (alternative.composites = (alternative.composites ?? []).filter((_, k) => k !== compositeIndex))}><Icon name="close" /></button>
                  </div>
                {/if}
              {/each}
              <div class="row">
                <button class="t-btn small" class:primary={!(alternative.singles ?? []).length && !(alternative.composites ?? []).length} onclick={() => (alternative.singles = [...(alternative.singles ?? []), { chance: 10, item: { id: 0 } }])}>Add item</button>
                {#if (brush.alternatives ?? []).length > 1}<button class="t-btn small danger" onclick={() => (brush.alternatives = (brush.alternatives ?? []).filter((_, k) => k !== alternativeIndex))}>Remove variation</button>{/if}
              </div>
            {/if}
          {/each}
          <span class="t-label hint">Groups of several tiles are made from a selection: select them on the map, then Make doodad brush.</span>
        {/if}

        {#if editBorder}
          {@const edited = editBorder}
          <div class="borderedit t-panel">
            <div class="row">
              <span class="h">{edited.id ? `Border ${edited.id}` : "New border"}</span><span class="t-label hint">Click a position, then pick its item.</span><span class="grow"></span>
              <span class="t-label">Group</span><NumberField value={edited.group ?? 0} min={0} max={65535} width={46} onchange={(value) => (edited.group = value)} />
            </div>
            <div class="row top">
              {@render edgeGrid((edgeName) => edited.edges?.[edgeName] ?? 0, edge, (edgeName) => (edge = edgeName))}
              <div class="col">
                <span class="t-label">{EDGE_LABEL[edge]}</span>
                <ItemPicker value={edited.edges?.[edge] ?? 0} width={240} onchange={(id) => (edited.edges = { ...(edited.edges ?? {}), [edge]: id })} />
                <span class="t-label hint">Empty diagonals are drawn from their two sides.</span>
              </div>
            </div>
            <div class="row">
              <button class="t-btn primary small" onclick={finishBorder}>Save border</button>
              <button class="t-btn small" onclick={() => ((editBorder = null), (pendingRule = null))}>Cancel</button>
            </div>
          </div>
        {/if}

        <!-- Rarely changed: folded so the steps above stay the focus. -->
        <details class="adv">
          <summary>More options</summary>
          <div class="col advbody">
            {#if !stamp}
              <div class="row"><span class="t-label lbl">Library icon</span><ItemPicker value={brush.look ?? 0} width={260} placeholder="The first item" onchange={(id) => (brush.look = id)} /></div>
            {/if}
            <div class="row"><span class="t-label lbl">Collections</span><div class="grow"><TagPicker plain allowCustom value={collections} options={tilesets.map((tileset) => ({ tag: tileset }))} placeholder="none · type to add or create" onchange={(chosen) => (collections = chosen)} /></div></div>
            {#if brush.type === "ground"}
              <div class="row">
                <span class="t-label lbl">Z-order</span><NumberField value={brush.zOrder ?? 0} min={-100000} max={100000} width={60} onchange={(value) => (brush.zOrder = value)} />
                <span class="t-label hint">Borders of a higher z-order lie over lower ones.</span>
              </div>
              <label class="row"><input class="t-check" type="checkbox" checked={!brush.noRandomize} onchange={(event) => (brush.noRandomize = !event.currentTarget.checked)} />Mix the ground items randomly</label>
              <div class="row">
                <span class="t-label lbl">Optional border</span>
                <Select value={brush.optional?.border ?? 0} width={150} options={borderOptions} onchange={(value) => (brush.optional = value ? ({ align: "outer", border: value } as BorderRule) : null)} />
                <button class="t-icon-btn" title="New optional border" onclick={() => ((editBorder = newBorder()), (pendingRule = -1))}><Icon name="plus" /></button>
              </div>
              <div class="row">
                <span class="t-label lbl">Friends</span>
                <input
                  class="t-input grow"
                  value={(brush.friends ?? []).join(", ")}
                  placeholder="brushes that get no border against this one"
                  onchange={(event) =>
                    (brush.friends = event.currentTarget.value
                      .split(",")
                      .map((name) => name.trim())
                      .filter(Boolean))}
                />
              </div>
            {:else if brush.type === "doodad" && brush.doodad}
              {@const options = brush.doodad}
              <div class="opts">
                <label class="row"><input class="t-check" type="checkbox" bind:checked={options.draggable} />Draw by dragging</label>
                <label class="row"><input class="t-check" type="checkbox" bind:checked={options.onBlocking} />Also on blocking tiles</label>
                <label class="row"><input class="t-check" type="checkbox" bind:checked={options.onDuplicate} />Allow twice on a tile</label>
                <label class="row"><input class="t-check" type="checkbox" bind:checked={options.oneSize} />One size (ignores brush size)</label>
                <label class="row"><input class="t-check" type="checkbox" bind:checked={options.redoBorders} />Redo borders (ground doodads)</label>
              </div>
              <div class="row">
                <span class="t-label lbl">Density</span>
                <NumberField value={options.thickness ?? 0} min={0} max={10000} width={50} onchange={(value) => (options.thickness = value)} /><span>/</span>
                <NumberField value={options.thicknessCeiling ?? 100} min={1} max={10000} width={50} onchange={(value) => (options.thicknessCeiling = value)} />
                <span class="t-label hint">objects per tile of a big brush</span>
              </div>
              <button class="t-btn small" onclick={() => (brush.alternatives = [...(brush.alternatives ?? []), { singles: [], composites: [] }])}>Add variation</button>
            {/if}
          </div>
        </details>
      {/if}
    </div>
  {/if}
  {#snippet footer()}
    {#if step === "edit"}
      {#if missing}<span class="t-label need">{missing}</span>{/if}
      <span class="grow"></span>
      <button class="t-btn primary" disabled={saving || !!missing} onclick={save}>{fromSelection ? "Create" : isNew ? "Create brush" : "Save brush"}</button>
    {/if}
    <button class="t-btn" onclick={() => (map.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .form {
    gap: 6px;
    max-height: calc(100vh - 160px);
    overflow-y: auto;
    padding-right: 4px;
  }
  /* Step heading: gold like other section titles, hint on the same line. */
  .sh {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
  }
  .sh:first-child {
    margin-top: 0;
  }
  .st {
    flex: none;
    color: var(--gold);
  }
  .kind {
    flex: none;
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
  }
  .kind .link {
    align-self: center;
  }
  .types {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }
  .type {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 3px;
    padding: 8px 10px;
    text-align: left;
    font: inherit;
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid #444;
    cursor: pointer;
  }
  .type:hover,
  .type:focus-visible {
    border-color: var(--gold);
    background: rgba(255, 255, 255, 0.05);
    outline: none;
  }
  .tname {
    color: var(--text-bright);
  }
  .tdesc {
    color: var(--text-dim);
    font-size: 12px;
  }
  .link {
    background: none;
    border: 0;
    padding: 0;
    color: var(--gold);
    font: inherit;
    font-size: 12px;
    cursor: pointer;
    text-decoration: underline;
    align-self: flex-start;
  }
  .adv {
    margin-top: 8px;
    border-top: 1px solid #333;
    padding-top: 6px;
  }
  .adv summary {
    cursor: pointer;
    color: var(--text-dim);
  }
  .adv summary:hover {
    color: var(--text-bright);
  }
  .advbody {
    gap: 6px;
    margin-top: 6px;
  }
  .need {
    align-self: center;
    color: var(--text-dim);
    font-size: 12px;
  }
  .grow {
    flex: 1;
  }
  .lbl {
    width: 100px;
    flex: none;
  }
  .hint {
    font-size: 12px;
    margin: 0;
  }
  .h {
    color: var(--gold);
  }
  .small {
    min-width: 0;
    padding: 0 8px;
    align-self: flex-start;
  }
  .rule {
    display: flex;
    gap: 4px;
    align-items: center;
    flex-wrap: wrap;
  }
  .mini {
    display: inline-flex;
    gap: 1px;
    vertical-align: middle;
  }
  .wall {
    border: 1px solid #333;
    padding: 3px 6px;
  }
  .wall summary {
    cursor: pointer;
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .wall[open] {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .cap {
    text-transform: capitalize;
  }
  .edges {
    display: grid;
    grid-template-columns: repeat(6, 36px);
    grid-template-rows: repeat(3, 36px);
    gap: 2px;
  }
  .slot {
    position: relative;
    border: 1px solid transparent;
    cursor: pointer;
    padding: 0;
  }
  .slot.on {
    outline: 1px solid var(--gold);
  }
  .el {
    position: absolute;
    left: 1px;
    top: 0;
    font: var(--fs-small) / 10px var(--font-small);
    color: var(--text-dim);
    text-shadow: 1px 1px 0 #000;
  }
  .top {
    align-items: flex-start;
    gap: 12px;
  }
  .borderedit {
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .opts {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 3px;
  }
  .stamp {
    flex: 1;
    min-width: 0;
  }
  .tilebox {
    width: 220px;
    flex: none;
    gap: 4px;
  }
  .stack {
    max-height: 260px;
    overflow-y: auto;
    padding: 2px;
  }
  .cpv {
    width: 72px;
    flex: none;
  }
  .comp {
    gap: 6px;
  }
</style>
