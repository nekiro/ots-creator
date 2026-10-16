<script lang="ts">
  import { ask } from "../../lib/confirm.svelte";
  import { MapService, TILE_FLAGS, fmtPos, type Attr, type ItemRef, type TileItem, type Position, type ItemPatch } from "../../lib/map/api";
  import { goTo, mapCommands } from "../../lib/map/commands";
  import { edit, map, selectBrush, useCreature } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import ItemIcon from "./ItemIcon.svelte";
  import { ATTR_TYPE_OPTIONS, DIRECTION_OPTIONS, FLUID_OPTIONS, parseCount } from "../../lib/map/itemAttrs";
  import { clamp } from "../../lib/math";

  const tile = $derived(map.tile);
  const inspected = $derived(map.inspect);
  /** Selected entry: -1 ground, n items index; path into containers. */
  let sel = $state<{ index: number; path: number[] } | null>(null);
  let addId = $state("");

  // A different tile resets the item selection.
  let lastPos = "";
  $effect(() => {
    const key = fmtPos(map.inspect);
    if (key !== lastPos) {
      lastPos = key;
      sel = null;
    }
  });

  /** Items top first, with their index in the tile (-1 ground). */
  const stack = $derived.by(() => {
    if (!tile) return [] as { it: TileItem; index: number }[];
    const out = (tile.items ?? []).map((it, index) => ({ it, index })).reverse();
    if (tile.ground) out.push({ it: tile.ground, index: -1 });
    return out;
  });

  function itemAt(index: number, path: number[]): TileItem | null {
    if (!tile) return null;
    let item: TileItem | null | undefined = index < 0 ? tile.ground : tile.items?.[index];
    for (const i of path) item = item?.contents?.[i];
    return item ?? null;
  }

  const current = $derived(sel ? itemAt(sel.index, sel.path) : null);
  const ref = $derived<ItemRef | null>(sel && inspected ? { pos: inspected, index: sel.index, path: sel.path } : null);

  function patch(changes: ItemPatch) {
    const itemRef = ref;
    if (itemRef) void edit((id) => MapService.UpdateItem(id, itemRef, changes));
  }

  function pick(index: number) {
    sel = { index, path: [] };
    if (inspected) void edit((id) => MapService.SelectItem(id, inspected, index, false));
  }

  async function removeSel() {
    const itemRef = ref;
    if (!itemRef) return;
    await edit((id) => MapService.RemoveItem(id, itemRef));
    sel = null;
  }

  async function removeAt(index: number) {
    if (!inspected) return;
    const pos = inspected;
    await edit((id) => MapService.RemoveItem(id, { pos, index, path: [] }));
    if (sel && !sel.path.length && (sel.index === index || (index >= 0 && sel.index > index))) sel = sel.index === index ? null : { index: sel.index - 1, path: [] };
  }

  async function move(index: number, to: number) {
    if (!inspected || !tile || to < 0 || to >= (tile.items?.length ?? 0)) return;
    await edit((id) => MapService.MoveItem(id, inspected, index, to));
    if (sel?.index === index) sel = { index: to, path: [] };
  }

  // Dragging a row changes the stack order. Rows are top first, so "above
  // a row" means a higher index in the tile.
  let dragFrom = $state<number | null>(null);
  let dropAt = $state<{ index: number; above: boolean } | null>(null);
  function dragOver(event: DragEvent, index: number) {
    if (dragFrom === null) return;
    event.preventDefault();
    const bounds = (event.currentTarget as HTMLElement).getBoundingClientRect();
    // Nothing goes under the ground.
    dropAt = { index, above: index < 0 || event.clientY < bounds.top + bounds.height / 2 };
  }
  function drop() {
    const from = dragFrom;
    const at = dropAt;
    dragFrom = dropAt = null;
    if (from === null || !at) return;
    const target = at.index;
    let to = at.above ? (from < target ? target : target + 1) : from < target ? target - 1 : target;
    to = clamp(to, 0, (tile?.items?.length ?? 1) - 1);
    if (to !== from) void move(from, to);
  }

  async function addItem() {
    const id = Number(addId);
    if (!inspected || !map.items?.has(id)) return;
    await edit((mapId) => MapService.AddItem(mapId, inspected, id));
    addId = "";
  }

  let contentId = $state("");
  async function addContent() {
    const id = Number(contentId);
    const itemRef = ref;
    if (!itemRef || !map.items?.has(id)) return;
    await edit((mapId) => MapService.AddToContainer(mapId, itemRef, id));
    contentId = "";
  }

  function pickDest() {
    const itemRef = ref;
    if (!itemRef) return;
    map.pick = {
      label: "Click the teleport destination",
      done: (dest: Position) => void edit((id) => MapService.UpdateItem(id, itemRef, { dest })),
    };
  }

  function setFlag(bit: number, on: boolean) {
    if (!inspected || !tile) return;
    const flags = on ? (tile.flags | bit) >>> 0 : tile.flags & ~bit;
    void edit((id) => MapService.UpdateTile(id, inspected, { flags }));
  }

  function attrsOf(item: TileItem): Attr[] {
    return [...(item.ext?.attrs ?? [])].filter((attr): attr is Attr => !!attr);
  }

  function setAttrs(attrs: Attr[]) {
    patch({ attrs });
  }

  function focusTown(name: string) {
    const town = map.active?.towns?.find((candidate) => candidate?.name === name);
    if (town) mapCommands.focus({ kind: "town", id: town.id });
  }
  function focusHouse(name: string) {
    const house = map.active?.houses?.find((candidate) => candidate?.name === name);
    if (house) mapCommands.focus({ kind: "house", id: house.id });
  }

  const houses = $derived([{ value: 0, label: "No house" }, ...(map.active?.houses ?? []).filter((house) => !!house).map((house) => ({ value: house.id, label: `${house.id} · ${house.name}` }))]);
</script>

{#if !inspected}
    <div class="none t-label">Click a tile with the selection tool (S) to see and edit everything on it.</div>
  {:else if !tile}
    <div class="none t-label">Loading…</div>
  {:else}
    <div class="body">
      {#if tile.temple || tile.houseExit || tile.waypoints?.length}
        <div class="badges">
          {#if tile.temple}<button class="badge gold" title="Show the town" onclick={() => focusTown(tile.temple)}>{tile.temple} temple</button>{/if}
          {#if tile.houseExit}<button class="badge blue" title="Show the house" onclick={() => focusHouse(tile.houseExit)}>{tile.houseExit} exit</button>{/if}
          {#each tile.waypoints ?? [] as w}<button class="badge green" title="Show the waypoint" onclick={() => mapCommands.focus({ kind: "waypoint", name: w })}>waypoint {w}</button>{/each}
        </div>
      {/if}
      <div class="t-label sect">Items (top first)</div>
      <div class="stack t-panel">
        {#each stack as { it, index } (index)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="it"
            class:on={sel?.index === index && !sel.path.length}
            class:selected={it.selected}
            class:dragging={dragFrom === index}
            class:dropabove={dropAt?.index === index && dropAt.above}
            class:dropbelow={dropAt?.index === index && !dropAt.above}
            draggable={index >= 0}
            title={index >= 0 ? "Drag to change the order" : undefined}
            ondragstart={(event) => {
              dragFrom = index;
              event.dataTransfer?.setData("text/plain", String(index));
              if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
            }}
            ondragover={(event) => dragOver(event, index)}
            ondrop={(event) => (event.preventDefault(), drop())}
            ondragend={() => (dragFrom = dropAt = null)}
          >
            <button class="pickbtn" title="{it.name} · server id {it.id}, client id {it.clientId}" onclick={() => pick(index)} ondblclick={() => selectBrush({ kind: "raw", item: it.id, look: it.id, label: it.name })}>
              <span class="t-slot"><ItemIcon id={it.id} /></span>
              <span class="nm">
                <span class="n">{it.name}</span>
                <span class="d">
                  {it.id}{#if it.subtype}{` · ${it.kind === "fluid" ? "fluid" : "count"} ${it.subtype}`}{/if}{#if it.ext?.actionId}{` · aid ${it.ext.actionId}`}{/if}{#if it.ext?.uniqueId}{` · uid ${it.ext.uniqueId}`}{/if}{#if it.ext?.dest}{` · → ${fmtPos(it.ext.dest)}`}{/if}{#if it.contents?.length}{` · ${it.contents.length} inside`}{/if}{#if it.brush}{` · ${it.brush}`}{/if}
                </span>
              </span>
            </button>
            <button class="t-icon-btn danger del" title="Remove {it.name}" onclick={() => removeAt(index)}><Icon name="trash" /></button>
          </div>
        {:else}
          <div class="t-label empty">Empty tile</div>
        {/each}
      </div>
      <div class="row add">
        <input class="t-input grow" placeholder="Add item id…" bind:value={addId} onkeydown={(event) => event.key === "Enter" && addItem()} />
        {#if addId && map.items?.has(Number(addId))}<span class="t-label hint">{map.items.name(Number(addId))}</span>{/if}
        <button class="t-icon-btn" title="Add item" disabled={!map.items?.has(Number(addId))} onclick={addItem}><Icon name="plus" /></button>
      </div>

      {#if current && sel}
        {@const it = current}
        <div class="t-sep"></div>
        <div class="props">
          <div class="row head">
            <span class="t-slot"><ItemIcon id={it.id} /></span>
            <span class="grow title">{it.name}{#if sel.path.length}<span class="t-label"> (in container)</span>{/if}</span>
            {#if it.rotateTo}<button class="t-icon-btn" title="Rotate" onclick={() => ref && edit((id) => MapService.RotateItem(id, ref!))}><Icon name="refresh" /></button>{/if}
            {#if it.kind === "door" && !sel.path.length}<button class="t-icon-btn" title="Open or close" onclick={() => inspected && sel && edit((id) => MapService.SwitchDoor(id, inspected!, sel!.index))}><Icon name="replace" /></button>{/if}
            <button class="t-icon-btn" title="Use as brush" onclick={() => selectBrush({ kind: "raw", item: it.id, look: it.id, label: it.name })}><Icon name="pencil" /></button>
            <button class="t-icon-btn danger" title="Remove (Del)" onclick={removeSel}><Icon name="trash" /></button>
          </div>
          <div class="grid2">
            <label>Action id<input class="t-input" value={it.ext?.actionId || ""} placeholder="0" onchange={(event) => patch({ actionId: parseCount(event.currentTarget.value) })} /></label>
            <label>Unique id<input class="t-input" value={it.ext?.uniqueId || ""} placeholder="0" onchange={(event) => patch({ uniqueId: parseCount(event.currentTarget.value) })} /></label>
            {#if it.kind === "stackable"}
              <label>Count<input class="t-input" value={it.subtype} onchange={(event) => patch({ subtype: Math.max(1, Math.min(map.settings?.countU16 ? 65535 : 255, parseCount(event.currentTarget.value))) })} /></label>
            {:else if it.kind === "charges" || it.kind === "rune"}
              <label>Charges<input class="t-input" value={it.subtype} onchange={(event) => patch({ subtype: parseCount(event.currentTarget.value) })} /></label>
            {:else if it.kind === "fluid"}
              <label class="wide">Fluid<Select grow value={it.subtype} options={FLUID_OPTIONS} onchange={(value) => patch({ subtype: value })} /></label>
            {/if}
            {#if it.kind === "door"}
              <label>Door id<input class="t-input" value={it.ext?.doorId || ""} placeholder="0" onchange={(event) => patch({ doorId: Math.min(255, parseCount(event.currentTarget.value)) })} /></label>
            {/if}
            {#if it.kind === "depot"}
              <label>Depot id<input class="t-input" value={it.ext?.depotId || ""} placeholder="0" onchange={(event) => patch({ depotId: parseCount(event.currentTarget.value) })} /></label>
            {/if}
          </div>
          {#if it.kind === "teleport" || it.ext?.dest}
            <div class="row dest">
              <span class="t-label">Destination</span>
              <input class="t-input grow" value={it.ext?.dest ? fmtPos(it.ext.dest) : ""} placeholder="x, y, z" onchange={(event) => {
                const m = event.currentTarget.value.match(/(\d+)\D+(\d+)\D+(\d+)/);
                if (m) patch({ dest: { x: +m[1], y: +m[2], z: +m[3] } });
              }} />
              <button class="t-icon-btn" title="Pick on the map" onclick={pickDest}><Icon name="picker" /></button>
              {#if it.ext?.dest}<button class="t-icon-btn" title="Go to destination" onclick={() => it.ext?.dest && goTo(it.ext.dest)}><Icon name="arrowE" /></button>{/if}
            </div>
          {/if}
          {#if it.kind === "writeable" || it.ext?.text || map.items?.is(it.id, "readable")}
            <label class="col">Text<textarea class="t-input text" value={it.ext?.text ?? ""} onchange={(event) => patch({ text: event.currentTarget.value })}></textarea></label>
          {/if}
          <label class="col">Description<input class="t-input" value={it.ext?.description ?? ""} onchange={(event) => patch({ description: event.currentTarget.value })} /></label>
          {#if it.kind === "container" || it.kind === "depot" || it.contents?.length}
            <div class="t-label sect">Contents</div>
            <div class="contents t-panel">
              {#each it.contents ?? [] as c, ci}
                <div class="it">
                  <button class="pickbtn" onclick={() => (sel = { index: sel!.index, path: [...sel!.path, ci] })}>
                    <span class="t-slot"><ItemIcon id={c.id} /></span>
                    <span class="nm"><span class="n">{c.name}</span><span class="d">{c.id}{c.subtype ? ` · ${c.subtype}` : ""}{c.contents?.length ? ` · ${c.contents.length} inside` : ""}</span></span>
                  </button>
                </div>
              {:else}
                <div class="t-label empty">Empty</div>
              {/each}
            </div>
            <div class="row add">
              <input class="t-input grow" placeholder="Put item id…" bind:value={contentId} onkeydown={(event) => event.key === "Enter" && addContent()} />
              <button class="t-icon-btn" title="Put into the container" disabled={!map.items?.has(Number(contentId))} onclick={addContent}><Icon name="plus" /></button>
            </div>
          {/if}
          {#if sel.path.length}
            <button class="t-btn back" onclick={() => (sel = { index: sel!.index, path: sel!.path.slice(0, -1) })}>Back to the container</button>
          {/if}
          <details class="attrs">
            <summary class="t-label">Custom attributes ({attrsOf(it).length})</summary>
            {#each attrsOf(it) as a, ai}
              <div class="row attr">
                <input class="t-input key" value={a.key} onchange={(event) => setAttrs(attrsOf(it).map((x, k) => (k === ai ? { ...x, key: event.currentTarget.value } : x)))} />
                <Select value={a.type} width={80} options={ATTR_TYPE_OPTIONS} onchange={(value) => setAttrs(attrsOf(it).map((x, k) => (k === ai ? { ...x, type: value } : x)))} />
                <input class="t-input grow" value={a.type === 1 ? (a.str ?? "") : a.type === 4 ? String(!!a.value) : a.type === 2 ? String(a.int ?? 0) : String(a.num ?? 0)} onchange={(event) => {
                  const v = event.currentTarget.value;
                  const next = { ...a };
                  if (a.type === 1) next.str = v;
                  else if (a.type === 4) next.value = v === "true" || v === "1";
                  else if (a.type === 2) next.int = Math.trunc(Number(v)) || 0;
                  else next.num = Number(v) || 0;
                  setAttrs(attrsOf(it).map((x, k) => (k === ai ? next : x)));
                }} />
                <button class="t-icon-btn danger" title="Remove" onclick={() => setAttrs(attrsOf(it).filter((_, k) => k !== ai))}><Icon name="close" /></button>
              </div>
            {/each}
            <button class="t-btn" onclick={() => setAttrs([...attrsOf(it), { key: "key", type: 1, str: "" }])}>Add attribute</button>
          </details>
        </div>
      {/if}

      {#if tile.creatures?.length}
        <div class="t-sep"></div>
        <div class="t-label sect">Creatures</div>
        {#each tile.creatures as c, ci}
          <div class="creature">
            <span class="t-slot"><ItemIcon outfit={c.outfit.lookItem ? 0 : c.outfit.lookType} id={c.outfit.lookItem} /></span>
            <div class="col grow">
              <span class:unknown={c.unknown} title={c.unknown ? "Not in the creature list" : ""}>{c.name}{c.npc ? " (NPC)" : ""}</span>
              <div class="row">
                <input class="t-input sp" title="Spawn time (seconds)" value={c.spawnTime} onchange={(event) => inspected && edit((id) => MapService.UpdateCreature(id, inspected!, ci, { spawnTime: Math.max(1, Math.trunc(+event.currentTarget.value) || 60), remove: false }))} />
                <span class="t-label">s</span>
                <Select value={c.direction} width={70} options={DIRECTION_OPTIONS} onchange={(value) => inspected && edit((id) => MapService.UpdateCreature(id, inspected!, ci, { direction: value, remove: false }))} />
              </div>
            </div>
            <button class="t-icon-btn" title="Use as brush" onclick={() => useCreature(c)}><Icon name="pencil" /></button>
            <button class="t-icon-btn danger" title="Remove" onclick={() => inspected && edit((id) => MapService.UpdateCreature(id, inspected!, ci, { remove: true }))}><Icon name="trash" /></button>
          </div>
        {/each}
      {/if}
      {#each [{ s: tile.spawn, npc: false, label: "Spawn" }, { s: tile.spawnNpc, npc: true, label: "NPC spawn" }] as sp}
        {#if sp.s}
          <div class="row spawn">
            <span class="t-label">{sp.label} radius</span>
            <input class="t-input sp" value={sp.s.radius} onchange={(event) => inspected && edit((id) => MapService.UpdateSpawn(id, inspected!, sp.npc, Math.max(1, Math.trunc(+event.currentTarget.value) || 1)))} />
            <button class="t-icon-btn danger" title="Remove spawn" onclick={() => inspected && edit((id) => MapService.UpdateSpawn(id, inspected!, sp.npc, 0))}><Icon name="trash" /></button>
          </div>
        {/if}
      {/each}

      <div class="t-sep"></div>
      <div class="flags">
        {#each [[TILE_FLAGS.pz, "PZ"], [TILE_FLAGS.noPvp, "No PvP"], [TILE_FLAGS.noLogout, "No logout"], [TILE_FLAGS.pvpZone, "PvP zone"]] as [bit, label]}
          <label class="row"><input class="t-check" type="checkbox" checked={(tile.flags & (bit as number)) !== 0} onchange={(event) => setFlag(bit as number, event.currentTarget.checked)} />{label}</label>
        {/each}
      </div>
      <div class="row">
        <span class="t-label">House</span>
        <Select grow value={tile.houseId} options={houses} onchange={(value) => inspected && edit((id) => MapService.UpdateTile(id, inspected!, { houseId: value }))} />
        {#if tile.houseId}<button class="t-icon-btn" title="Show the house" onclick={() => mapCommands.focus({ kind: "house", id: tile.houseId })}><Icon name="arrowE" /></button>{/if}
      </div>
      {#if tile.zones?.length}
        <div class="t-label zones">Zones: {tile.zones.map((z) => map.active?.zones?.find((x) => x.id === z)?.name ?? z).join(", ")}</div>
      {/if}
      <div class="row foot">
        <button class="t-btn" onclick={() => inspected && goTo(inspected)}>Center</button>
        <button class="t-btn" onclick={() => inspected && edit((id) => MapService.SelectTiles(id, [inspected!], false))}>Select tile</button>
        <span class="grow"></span>
        <button class="t-icon-btn" title="Close" onclick={() => mapCommands.inspect(null)}><Icon name="close" /></button>
      </div>
      {#if tile.modified}<div class="t-label mod">Changed since the map was saved</div>{/if}
    </div>
  {/if}

<style>
  .none {
    padding: 12px;
    text-align: center;
  }
  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 0 2px;
  }
  .sect {
    padding: 2px 2px 0;
  }
  .stack,
  .contents {
    padding: 2px;
    display: flex;
    flex-direction: column;
    gap: 1px;
    max-height: 210px;
    overflow-y: auto;
    flex: none;
  }
  .it {
    display: flex;
    align-items: center;
    border: 1px solid transparent;
  }
  .it.on {
    border-color: #b6b6b6;
    background: var(--select);
  }
  .it.selected .n {
    color: #9cc6ff;
  }
  .it.dragging {
    opacity: 0.4;
  }
  .it.dropabove {
    box-shadow: inset 0 2px 0 var(--gold);
  }
  .it.dropbelow {
    box-shadow: inset 0 -2px 0 var(--gold);
  }
  .pickbtn {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 6px;
    align-items: center;
    background: none;
    border: 0;
    padding: 0;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .nm {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .n,
  .d {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .d {
    font: var(--fs-small) / 11px var(--font-small);
    color: var(--text-dim);
  }
  .del {
    flex: none;
    margin-right: 2px;
    color: #fff;
    border-width: 3px;
    border-image: url("../../assets/ui/btn-danger-idle.png") 3 fill / 3px stretch;
  }
  .del:active:not(:disabled) {
    border-image-source: url("../../assets/ui/btn-danger-pressed.png");
  }
  .del :global(svg) {
    width: 12px;
    height: 12px;
  }
  .empty {
    padding: 6px;
    text-align: center;
  }
  .add .hint {
    max-width: 100px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .props {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .head .title {
    color: var(--text-bright);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .grid2 {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 4px;
  }
  .grid2 :global(.t-input) {
    width: 100%;
    box-sizing: border-box;
  }
  .grid2 label {
    display: flex;
    flex-direction: column;
    gap: 1px;
    color: var(--text-dim);
  }
  .grid2 .wide {
    grid-column: span 2;
  }
  .col {
    color: var(--text-dim);
  }
  .text {
    height: 60px;
    resize: vertical;
    font-weight: normal;
    padding: 3px;
  }
  .dest .t-label {
    flex: none;
  }
  .back {
    align-self: flex-start;
  }
  .attrs summary {
    cursor: pointer;
    padding: 2px;
  }
  .attr .key {
    width: 70px;
  }
  .creature {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .unknown {
    color: #e98;
  }
  .sp {
    width: 48px;
  }
  .flags {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 2px 8px;
  }
  .foot {
    margin-top: 2px;
  }
  .foot .t-btn {
    min-width: 0;
    padding: 0 8px;
  }
  .mod {
    font: var(--fs-small) / 10px var(--font-small);
    color: var(--gold);
  }
  .badges {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
  }
  .badge {
    padding: 0 5px;
    border: 1px solid;
    background: none;
    cursor: pointer;
    font: var(--fs-small) / 14px var(--font-small);
  }
  .gold {
    color: var(--gold);
  }
  .blue {
    color: var(--blue);
  }
  .green {
    color: #7c7;
  }
  .zones {
    font-size: 12px;
  }
</style>
