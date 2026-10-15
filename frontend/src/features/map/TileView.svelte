<script lang="ts">
  import { ask } from "../../lib/confirm.svelte";
  import { MapService, TILE_FLAGS, fmtPos, type Attr, type ItemRef, type TileItem, type Position, type ItemPatch } from "../../lib/map/api";
  import { goTo, mapCommands } from "../../lib/map/commands";
  import { edit, map, selectBrush, useCreature } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import ItemIcon from "./ItemIcon.svelte";

  const FLUIDS = [
    [0, "Empty"],
    [1, "Water"],
    [2, "Blood"],
    [3, "Beer"],
    [4, "Slime"],
    [5, "Lemonade"],
    [6, "Milk"],
    [7, "Mana"],
    [10, "Life"],
    [11, "Oil"],
    [13, "Urine"],
    [14, "Coconut milk"],
    [15, "Wine"],
    [19, "Mud"],
    [21, "Fruit juice"],
    [26, "Lava"],
    [27, "Rum"],
    [28, "Swamp"],
    [35, "Tea"],
    [43, "Mead"],
  ] as const;
  const DIRS = [
    { value: 0, label: "North" },
    { value: 1, label: "East" },
    { value: 2, label: "South" },
    { value: 3, label: "West" },
  ];
  const ATTR_TYPES = [
    { value: 1, label: "Text" },
    { value: 2, label: "Integer" },
    { value: 3, label: "Float" },
    { value: 4, label: "Boolean" },
    { value: 5, label: "Double" },
  ];

  const t = $derived(map.tile);
  const p = $derived(map.inspect);
  /** Selected entry: -1 ground, n items index; path into containers. */
  let sel = $state<{ index: number; path: number[] } | null>(null);
  let addId = $state("");

  // A different tile resets the item selection.
  let lastPos = "";
  $effect(() => {
    const k = fmtPos(map.inspect);
    if (k !== lastPos) {
      lastPos = k;
      sel = null;
    }
  });

  /** Items top first, with their index in the tile (-1 ground). */
  const stack = $derived.by(() => {
    if (!t) return [] as { it: TileItem; index: number }[];
    const out = (t.items ?? []).map((it, index) => ({ it, index })).reverse();
    if (t.ground) out.push({ it: t.ground, index: -1 });
    return out;
  });

  function itemAt(index: number, path: number[]): TileItem | null {
    if (!t) return null;
    let it: TileItem | null | undefined = index < 0 ? t.ground : t.items?.[index];
    for (const i of path) it = it?.contents?.[i];
    return it ?? null;
  }

  const current = $derived(sel ? itemAt(sel.index, sel.path) : null);
  const ref = $derived<ItemRef | null>(sel && p ? { pos: p, index: sel.index, path: sel.path } : null);

  function patch(pt: ItemPatch) {
    const r = ref;
    if (r) void edit((id) => MapService.UpdateItem(id, r, pt));
  }

  function num(v: string): number {
    const n = Math.trunc(Number(v));
    return Number.isFinite(n) && n >= 0 ? Math.min(n, 65535) : 0;
  }

  function pick(it: TileItem, index: number) {
    sel = { index, path: [] };
    if (p) void edit((id) => MapService.SelectItem(id, p, index, false));
    void it;
  }

  async function removeSel() {
    const r = ref;
    if (!r) return;
    await edit((id) => MapService.RemoveItem(id, r));
    sel = null;
  }

  async function move(index: number, to: number) {
    if (!p || !t || to < 0 || to >= (t.items?.length ?? 0)) return;
    await edit((id) => MapService.MoveItem(id, p, index, to));
    if (sel?.index === index) sel = { index: to, path: [] };
  }

  // Dragging a row changes the stack order. Rows are top first, so "above
  // a row" means a higher index in the tile.
  let dragFrom = $state<number | null>(null);
  let dropAt = $state<{ index: number; above: boolean } | null>(null);
  function dragOver(e: DragEvent, index: number) {
    if (dragFrom === null) return;
    e.preventDefault();
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    // Nothing goes under the ground.
    dropAt = { index, above: index < 0 || e.clientY < r.top + r.height / 2 };
  }
  function drop() {
    const from = dragFrom;
    const at = dropAt;
    dragFrom = dropAt = null;
    if (from === null || !at) return;
    const j = at.index;
    let to = at.above ? (from < j ? j : j + 1) : from < j ? j - 1 : j;
    to = Math.min(Math.max(to, 0), (t?.items?.length ?? 1) - 1);
    if (to !== from) void move(from, to);
  }

  async function addItem() {
    const id = Number(addId);
    if (!p || !map.items?.has(id)) return;
    await edit((m) => MapService.AddItem(m, p, id));
    addId = "";
  }

  let contentId = $state("");
  async function addContent() {
    const id = Number(contentId);
    const r = ref;
    if (!r || !map.items?.has(id)) return;
    await edit((m) => MapService.AddToContainer(m, r, id));
    contentId = "";
  }

  function pickDest() {
    const r = ref;
    if (!r) return;
    map.pick = {
      label: "Click the teleport destination",
      done: (d: Position) => void edit((id) => MapService.UpdateItem(id, r, { dest: d })),
    };
  }

  function setFlag(bit: number, on: boolean) {
    if (!p || !t) return;
    const flags = on ? (t.flags | bit) >>> 0 : t.flags & ~bit;
    void edit((id) => MapService.UpdateTile(id, p, { flags }));
  }

  function attrsOf(it: TileItem): Attr[] {
    return [...(it.ext?.attrs ?? [])].filter((a): a is Attr => !!a);
  }

  function setAttrs(attrs: Attr[]) {
    patch({ attrs });
  }

  function focusTown(name: string) {
    const town = map.active?.towns?.find((x) => x?.name === name);
    if (town) mapCommands.focus({ kind: "town", id: town.id });
  }
  function focusHouse(name: string) {
    const h = map.active?.houses?.find((x) => x?.name === name);
    if (h) mapCommands.focus({ kind: "house", id: h.id });
  }

  const houses = $derived([{ value: 0, label: "No house" }, ...(map.active?.houses ?? []).filter((h) => !!h).map((h) => ({ value: h.id, label: `${h.id} · ${h.name}` }))]);
</script>

{#if !p}
    <div class="none t-label">Click a tile with the selection tool (S) to see and edit everything on it.</div>
  {:else if !t}
    <div class="none t-label">Loading…</div>
  {:else}
    <div class="body">
      {#if t.temple || t.houseExit || t.waypoints?.length}
        <div class="badges">
          {#if t.temple}<button class="badge gold" title="Show the town" onclick={() => focusTown(t.temple)}>{t.temple} temple</button>{/if}
          {#if t.houseExit}<button class="badge blue" title="Show the house" onclick={() => focusHouse(t.houseExit)}>{t.houseExit} exit</button>{/if}
          {#each t.waypoints ?? [] as w}<button class="badge green" title="Show the waypoint" onclick={() => mapCommands.focus({ kind: "waypoint", name: w })}>waypoint {w}</button>{/each}
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
            ondragstart={(e) => {
              dragFrom = index;
              e.dataTransfer?.setData("text/plain", String(index));
              if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
            }}
            ondragover={(e) => dragOver(e, index)}
            ondrop={(e) => (e.preventDefault(), drop())}
            ondragend={() => (dragFrom = dropAt = null)}
          >
            <button class="pickbtn" title="{it.name} · server id {it.id}, client id {it.clientId}" onclick={() => pick(it, index)} ondblclick={() => selectBrush({ kind: "raw", item: it.id, look: it.id, label: it.name })}>
              <span class="t-slot"><ItemIcon id={it.id} /></span>
              <span class="nm">
                <span class="n">{it.name}</span>
                <span class="d">
                  {it.id}{#if it.subtype}{` · ${it.kind === "fluid" ? "fluid" : "count"} ${it.subtype}`}{/if}{#if it.ext?.actionId}{` · aid ${it.ext.actionId}`}{/if}{#if it.ext?.uniqueId}{` · uid ${it.ext.uniqueId}`}{/if}{#if it.ext?.dest}{` · → ${fmtPos(it.ext.dest)}`}{/if}{#if it.contents?.length}{` · ${it.contents.length} inside`}{/if}{#if it.brush}{` · ${it.brush}`}{/if}
                </span>
              </span>
            </button>
            {#if index >= 0}
              <span class="ord">
                <button class="mini" title="Move up" disabled={index >= (t.items?.length ?? 0) - 1} onclick={() => move(index, index + 1)}>▲</button>
                <button class="mini" title="Move down" disabled={index <= 0} onclick={() => move(index, index - 1)}>▼</button>
              </span>
            {/if}
          </div>
        {:else}
          <div class="t-label empty">Empty tile</div>
        {/each}
      </div>
      <div class="row add">
        <input class="t-input grow" placeholder="Add item id…" bind:value={addId} onkeydown={(e) => e.key === "Enter" && addItem()} />
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
            {#if it.kind === "door" && !sel.path.length}<button class="t-icon-btn" title="Open or close" onclick={() => p && sel && edit((id) => MapService.SwitchDoor(id, p!, sel!.index))}><Icon name="replace" /></button>{/if}
            <button class="t-icon-btn" title="Use as brush" onclick={() => selectBrush({ kind: "raw", item: it.id, look: it.id, label: it.name })}><Icon name="pencil" /></button>
            <button class="t-icon-btn danger" title="Remove (Del)" onclick={removeSel}><Icon name="trash" /></button>
          </div>
          <div class="grid2">
            <label>Action id<input class="t-input" value={it.ext?.actionId || ""} placeholder="0" onchange={(e) => patch({ actionId: num(e.currentTarget.value) })} /></label>
            <label>Unique id<input class="t-input" value={it.ext?.uniqueId || ""} placeholder="0" onchange={(e) => patch({ uniqueId: num(e.currentTarget.value) })} /></label>
            {#if it.kind === "stackable"}
              <label>Count<input class="t-input" value={it.subtype} onchange={(e) => patch({ subtype: Math.max(1, Math.min(255, num(e.currentTarget.value))) })} /></label>
            {:else if it.kind === "charges" || it.kind === "rune"}
              <label>Charges<input class="t-input" value={it.subtype} onchange={(e) => patch({ subtype: num(e.currentTarget.value) })} /></label>
            {:else if it.kind === "fluid"}
              <label class="wide">Fluid<Select grow value={it.subtype} options={FLUIDS.map(([v, l]) => ({ value: v as number, label: l }))} onchange={(v) => patch({ subtype: v })} /></label>
            {/if}
            {#if it.kind === "door"}
              <label>Door id<input class="t-input" value={it.ext?.doorId || ""} placeholder="0" onchange={(e) => patch({ doorId: Math.min(255, num(e.currentTarget.value)) })} /></label>
            {/if}
            {#if it.kind === "depot"}
              <label>Depot id<input class="t-input" value={it.ext?.depotId || ""} placeholder="0" onchange={(e) => patch({ depotId: num(e.currentTarget.value) })} /></label>
            {/if}
          </div>
          {#if it.kind === "teleport" || it.ext?.dest}
            <div class="row dest">
              <span class="t-label">Destination</span>
              <input class="t-input grow" value={it.ext?.dest ? fmtPos(it.ext.dest) : ""} placeholder="x, y, z" onchange={(e) => {
                const m = e.currentTarget.value.match(/(\d+)\D+(\d+)\D+(\d+)/);
                if (m) patch({ dest: { x: +m[1], y: +m[2], z: +m[3] } });
              }} />
              <button class="t-icon-btn" title="Pick on the map" onclick={pickDest}><Icon name="picker" /></button>
              {#if it.ext?.dest}<button class="t-icon-btn" title="Go to destination" onclick={() => it.ext?.dest && goTo(it.ext.dest)}><Icon name="arrowE" /></button>{/if}
            </div>
          {/if}
          {#if it.kind === "writeable" || it.ext?.text}
            <label class="col">Text<textarea class="t-input text" value={it.ext?.text ?? ""} onchange={(e) => patch({ text: e.currentTarget.value })}></textarea></label>
          {/if}
          <label class="col">Description<input class="t-input" value={it.ext?.description ?? ""} onchange={(e) => patch({ description: e.currentTarget.value })} /></label>
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
              <input class="t-input grow" placeholder="Put item id…" bind:value={contentId} onkeydown={(e) => e.key === "Enter" && addContent()} />
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
                <input class="t-input key" value={a.key} onchange={(e) => setAttrs(attrsOf(it).map((x, k) => (k === ai ? { ...x, key: e.currentTarget.value } : x)))} />
                <Select value={a.type} width={80} options={ATTR_TYPES} onchange={(v) => setAttrs(attrsOf(it).map((x, k) => (k === ai ? { ...x, type: v } : x)))} />
                <input class="t-input grow" value={a.type === 1 ? (a.str ?? "") : a.type === 4 ? String(!!a.value) : a.type === 2 ? String(a.int ?? 0) : String(a.num ?? 0)} onchange={(e) => {
                  const v = e.currentTarget.value;
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

      {#if t.creatures?.length}
        <div class="t-sep"></div>
        <div class="t-label sect">Creatures</div>
        {#each t.creatures as c, ci}
          <div class="creature">
            <span class="t-slot"><ItemIcon outfit={c.outfit.lookItem ? 0 : c.outfit.lookType} id={c.outfit.lookItem} /></span>
            <div class="col grow">
              <span class:unknown={c.unknown} title={c.unknown ? "Not in the creature list" : ""}>{c.name}{c.npc ? " (NPC)" : ""}</span>
              <div class="row">
                <input class="t-input sp" title="Spawn time (seconds)" value={c.spawnTime} onchange={(e) => p && edit((id) => MapService.UpdateCreature(id, p!, ci, { spawnTime: Math.max(1, Math.trunc(+e.currentTarget.value) || 60), remove: false }))} />
                <span class="t-label">s</span>
                <Select value={c.direction} width={70} options={DIRS} onchange={(v) => p && edit((id) => MapService.UpdateCreature(id, p!, ci, { direction: v, remove: false }))} />
              </div>
            </div>
            <button class="t-icon-btn" title="Use as brush" onclick={() => useCreature(c)}><Icon name="pencil" /></button>
            <button class="t-icon-btn danger" title="Remove" onclick={() => p && edit((id) => MapService.UpdateCreature(id, p!, ci, { remove: true }))}><Icon name="trash" /></button>
          </div>
        {/each}
      {/if}
      {#each [{ s: t.spawn, npc: false, label: "Spawn" }, { s: t.spawnNpc, npc: true, label: "NPC spawn" }] as sp}
        {#if sp.s}
          <div class="row spawn">
            <span class="t-label">{sp.label} radius</span>
            <input class="t-input sp" value={sp.s.radius} onchange={(e) => p && edit((id) => MapService.UpdateSpawn(id, p!, sp.npc, Math.max(1, Math.trunc(+e.currentTarget.value) || 1)))} />
            <button class="t-icon-btn danger" title="Remove spawn" onclick={() => p && edit((id) => MapService.UpdateSpawn(id, p!, sp.npc, 0))}><Icon name="trash" /></button>
          </div>
        {/if}
      {/each}

      <div class="t-sep"></div>
      <div class="flags">
        {#each [[TILE_FLAGS.pz, "PZ"], [TILE_FLAGS.noPvp, "No PvP"], [TILE_FLAGS.noLogout, "No logout"], [TILE_FLAGS.pvpZone, "PvP zone"]] as [bit, label]}
          <label class="row"><input class="t-check" type="checkbox" checked={(t.flags & (bit as number)) !== 0} onchange={(e) => setFlag(bit as number, e.currentTarget.checked)} />{label}</label>
        {/each}
      </div>
      <div class="row">
        <span class="t-label">House</span>
        <Select grow value={t.houseId} options={houses} onchange={(v) => p && edit((id) => MapService.UpdateTile(id, p!, { houseId: v }))} />
        {#if t.houseId}<button class="t-icon-btn" title="Show the house" onclick={() => mapCommands.focus({ kind: "house", id: t.houseId })}><Icon name="arrowE" /></button>{/if}
      </div>
      {#if t.zones?.length}
        <div class="t-label zones">Zones: {t.zones.map((z) => map.active?.zones?.find((x) => x.id === z)?.name ?? z).join(", ")}</div>
      {/if}
      <div class="row foot">
        <button class="t-btn" onclick={() => p && goTo(p)}>Center</button>
        <button class="t-btn" onclick={() => p && edit((id) => MapService.SelectTiles(id, [p!], false))}>Select tile</button>
        <span class="grow"></span>
        <button class="t-icon-btn" title="Close" onclick={() => mapCommands.inspect(null)}><Icon name="close" /></button>
      </div>
      {#if t.modified}<div class="t-label mod">Changed since the map was saved</div>{/if}
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
  .ord {
    display: flex;
    flex-direction: column;
  }
  .mini {
    background: none;
    border: 0;
    color: var(--text-dim);
    font-size: 8px;
    line-height: 10px;
    padding: 0 3px;
    cursor: pointer;
  }
  .mini:disabled {
    opacity: 0.3;
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
    grid-template-columns: 1fr 1fr;
    gap: 4px;
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
