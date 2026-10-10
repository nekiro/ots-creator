<script lang="ts">
  // Compares the open client (A) with a second client (B) and copies objects
  // in both directions. Copies keep their ids (replacing, or extending the
  // target) or are appended as new objects. Objects of B can also be queued
  // and dropped into chosen slots of A (DropPanel, queue.svelte.ts).
  import { CATEGORIES, CATEGORY_LABELS, CATEGORY_NAMES, CompareService, Format, normalizeThing, res, ThingService, type Category, type DiffEntry, type DiffResult } from "../../lib/api";
  import { HoverAnim } from "../../lib/render/hoveranim.svelte";
  import { otherSpriteCache, spriteCache } from "../../lib/render/sprites";
  import { app, otherVersions, run, toast, versions } from "../../lib/state.svelte";
  import { ask } from "../../lib/confirm.svelte";
  import { commands } from "../../lib/commands";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import { dequeue, enqueue, queued } from "../../lib/queue.svelte";
  import DropPanel from "./DropPanel.svelte";
  import { openContextMenu, type MenuEntry } from "../../lib/menu.svelte";
  import ThingCanvas from "../../lib/ui/ThingCanvas.svelte";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import { loading as imgLoading } from "../../lib/ui/loading";

  type Status = "changed" | "onlyA" | "onlyB";
  const STATUS_LABELS: Record<Status, string> = { changed: "Changed", onlyA: "Only in A", onlyB: "Only in B" };
  const CHANGE_LABELS: Record<string, string> = {
    name: "name",
    props: "properties",
    size: "size",
    patterns: "patterns",
    frames: "frame count",
    groups: "frame groups",
    animation: "animation",
    npcSales: "NPC trade",
    sprites: "sprites",
  };

  let category = $state<Category>(app.category);
  let diff = $state<DiffResult | null>(null);
  let loading = $state(false);
  let error = $state("");
  let shown = $state<Record<Status, boolean>>({ changed: true, onlyA: true, onlyB: true });
  // Ids that exist in one client only and show nothing there are mostly
  // padding at the end of a category.
  let hideEmpty = $state(true);
  const visible = $derived((diff?.entries ?? []).filter((e) => !(hideEmpty && e.empty)));
  let selected = $state<number[]>([]);
  let anchor: number | null = null;

  const a = $derived(app.project?.info);
  const b = $derived(app.other?.info);
  const improvedA = $derived(a?.format === Format.FormatAssets || !!a?.features.improvedAnimations);
  const improvedB = $derived(b?.format === Format.FormatAssets || !!b?.features.improvedAnimations);

  // Hovering a row animates both of its thumbnails, like the object list.
  const hoverAnim = (get: typeof ThingService.Get) => async (id: number) => {
    const c = category;
    const raw = await get(c, id);
    return raw && c === category ? normalizeThing(raw) : null;
  };
  const hoverA = new HoverAnim(spriteCache, hoverAnim(ThingService.Get));
  const hoverB = new HoverAnim(otherSpriteCache, hoverAnim(CompareService.Thing));
  function enter(e: DiffEntry) {
    if (e.status !== "onlyB") hoverA.enter(e.id);
    if (e.status !== "onlyA") hoverB.enter(e.id);
  }
  function leave() {
    hoverA.leave();
    hoverB.leave();
  }
  // Objects copied with their ids are equal afterwards and drop out of the
  // diff. They stay listed as copied (until the category changes), so the
  // list does not shift under the cursor.
  let copied = $state<Record<number, "A" | "B">>({});
  const entries = $derived.by(() => {
    const list: DiffEntry[] = visible.filter((e) => shown[e.status as Status]);
    const listed = new Set((diff?.entries ?? []).map((e) => e.id));
    const extra = Object.entries(copied)
      .filter(([id]) => !listed.has(+id))
      .map(([id, to]) => ({ id: +id, status: "copied", changes: [`copied to ${to}`] }));
    return extra.length ? [...list, ...extra].sort((x, y) => x.id - y.id) : list;
  });
  const counts = $derived.by(() => {
    const out: Record<Status, number> = { changed: 0, onlyA: 0, onlyB: 0 };
    for (const e of visible) out[e.status as Status]++;
    return out;
  });
  const picked = $derived(entries.filter((e) => selected.includes(e.id)));
  const fromA = $derived(picked.filter((e) => e.status !== "onlyB" && e.status !== "copied"));
  const fromB = $derived(picked.filter((e) => e.status !== "onlyA" && e.status !== "copied"));
  const pickedCopies = $derived(picked.filter((e) => copied[e.id]).map((e) => e.id));

  // Compare again after every change of either client.
  let seq = 0;
  $effect(() => {
    const c = category;
    void app.project?.rev;
    void app.other?.rev;
    const n = ++seq;
    loading = true;
    CompareService.Diff(c)
      .then((d) => {
        if (n !== seq) return;
        diff = d;
        error = "";
        const ids = new Set((d.entries ?? []).map((e) => e.id));
        selected = selected.filter((id) => ids.has(id));
      })
      .catch((e) => n === seq && ((diff = null), (error = String(e?.message ?? e))))
      .finally(() => n === seq && (loading = false));
  });

  function setCategory(c: Category) {
    category = c;
    copied = {};
    selected = [];
    anchor = null;
  }

  function click(e: MouseEvent, id: number) {
    if (e.shiftKey && anchor !== null) {
      const from = entries.findIndex((x) => x.id === anchor);
      const to = entries.findIndex((x) => x.id === id);
      const range = entries.slice(Math.min(from, to), Math.max(from, to) + 1).map((x) => x.id);
      selected = e.ctrlKey ? [...new Set([...selected, ...range])] : range;
      return;
    }
    anchor = id;
    if (e.ctrlKey || e.metaKey) selected = selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id];
    else selected = [id];
  }

  // Right click selects the row (unless it is selected) and offers the
  // actions for the selection.
  function oncontextmenu(ev: MouseEvent, id: number) {
    if (!selected.includes(id)) {
      selected = [id];
      anchor = id;
    }
    const busy = () => !!app.busy;
    const unqueued = fromB.filter((e) => !queuedIds.has(e.id)).map((e) => e.id);
    const inQueue = picked.filter((e) => queuedIds.has(e.id)).map((e) => e.id);
    const items: MenuEntry[] = [
      { label: "B → A (same id)", action: () => transfer(false, false, fromB), disabled: () => !fromB.length || busy() },
      { label: "Append to A", action: () => transfer(false, true, fromB), disabled: () => !fromB.length || busy() },
      "-",
      { label: `Queue for A${unqueued.length > 1 ? ` (${unqueued.length})` : ""}`, keys: "Dbl click", action: () => enqueue(category, unqueued), disabled: () => !unqueued.length },
      ...(inQueue.length ? [{ label: "Remove from queue", action: () => dequeue(category, inQueue) }] : []),
      "-",
      { label: "A → B (same id)", action: () => transfer(true, false, fromA), disabled: () => !fromA.length || busy() },
      { label: "Append to B", action: () => transfer(true, true, fromA), disabled: () => !fromA.length || busy() },
      ...(pickedCopies.length ? (["-", { label: "Revert copy", action: () => revert(pickedCopies), disabled: busy }] as MenuEntry[]) : []),
      "-",
      { label: "Copy id", action: () => navigator.clipboard?.writeText(picked.map((e) => e.id).join(", ")).then(() => toast("Copied id(s).")) },
    ];
    openContextMenu(ev, items);
  }

  // Selects an id in the list and scrolls to it (from the queue and slots).
  let scrollTo = $state<number | null>(null);
  function show(id: number) {
    const i = entries.findIndex((e) => e.id === id);
    if (i < 0) {
      toast(`#${id} is not listed: it is the same in both clients or hidden by the filters.`, "info", 3500, "compare-show");
      return;
    }
    selected = [id];
    anchor = id;
    scrollTo = null;
    queueMicrotask(() => (scrollTo = i));
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "a") {
      e.preventDefault();
      selected = entries.map((x) => x.id);
    }
  }

  function selectStatus(s: Status) {
    selected = entries.filter((e) => e.status === s).map((e) => e.id);
  }

  /** Copies the picked objects; toOther copies A into B. */
  async function transfer(toOther: boolean, appendNew: boolean, list: DiffEntry[]) {
    if (!list.length) return;
    const ids = list.map((e) => e.id);
    const target = toOther ? "B" : "A";
    const r = await run(`Copying to ${target}`, () => CompareService.Transfer(toOther, category, ids, appendNew));
    if (!r) return;
    if (!appendNew) for (const id of ids) copied[id] = target;
    const what = `${ids.length} ${CATEGORY_NAMES[category]}(s)`;
    toast(`Copied ${what} to ${target}${appendNew ? ` as #${r.ids?.[0]}…` : ""}, ${r.sprites} new sprite(s).`, "success");
  }

  const queuedIds = $derived(new Set(queued(category)));

  // Double click queues the object of B (or takes it out of the queue).
  function dblclick(e: DiffEntry) {
    if (e.status === "onlyA" || e.status === "copied") return;
    if (queuedIds.has(e.id)) dequeue(category, [e.id]);
    else enqueue(category, [e.id]);
  }

  /** Puts copied objects back as they were before they were copied. */
  async function revert(ids: number[]) {
    for (const to of ["A", "B"] as const) {
      const list = ids.filter((id) => copied[id] === to);
      if (!list.length) continue;
      const ok = await run("Reverting", async () => {
        await CompareService.Revert(to === "B", category, list);
        return true;
      });
      if (!ok) return;
      for (const id of list) delete copied[id];
      toast(`Reverted ${list.length} ${CATEGORY_NAMES[category]}(s) in ${to}.`);
    }
  }

  /** Copies everything B has and A lacks or has differently, in every category. */
  async function mergeAll() {
    const plan: { c: Category; ids: number[] }[] = [];
    for (const c of CATEGORIES) {
      const d = await run("Comparing", () => CompareService.Diff(c));
      if (!d) return;
      const ids = (d.entries ?? []).filter((e) => e.status !== "onlyA" && !(hideEmpty && e.empty)).map((e) => e.id);
      if (ids.length) plan.push({ c, ids });
    }
    if (!plan.length) {
      toast("A already has everything B has.");
      return;
    }
    const summary = plan.map((p) => `${p.ids.length} ${CATEGORY_LABELS[p.c].toLowerCase()}`).join(", ");
    const ok = await ask({
      title: "Merge B into A",
      message: `Copy ${summary} from B into A? Changed objects are replaced, new ones keep their ids. Each category can be undone separately.`,
      ok: "Merge",
    });
    if (!ok) return;
    let sprites = 0;
    for (const p of plan) {
      const r = await run(`Merging ${CATEGORY_LABELS[p.c].toLowerCase()}`, () => CompareService.Transfer(false, p.c, p.ids, false));
      if (!r) return;
      if (p.c === category) for (const id of p.ids) copied[id] = "A";
      sprites += r.sprites;
    }
    toast(`Merged ${summary} into A, ${sprites} new sprite(s).`, "success");
  }

  function change() {
    app.openOther = true;
    app.dialog = "open";
  }

  async function compileB() {
    const ok = await run("Compiling B", async () => {
      await CompareService.Compile();
      return true;
    });
    if (ok) toast("Client B compiled.", "success");
  }

  async function undoB() {
    const label = await run("Undo", () => CompareService.Undo());
    if (label) toast(`Undone in B: ${label}`);
  }

  async function closeB() {
    if (b?.changed && !(await ask({ title: "Close client B", message: "Client B has uncompiled changes. Close it anyway?", ok: "Close" }))) return;
    app.dialog = null;
    await CompareService.Close();
  }

  function clientName(info: typeof a): string {
    if (!info) return "";
    if (!info.datPath) return "new client";
    const parts = info.datPath.split(/[\\/]/);
    return parts.slice(-2).join("/");
  }
</script>

<Dialog title="Compare & Merge Clients" width={900} onclose={() => (app.dialog = null)}>
  <div class="col compare">
    <div class="clients">
      <div class="client t-panel">
        <strong class="side">A</strong>
        <div class="who">
          <span class="name" title={a?.datPath}>{clientName(a)}{a?.changed ? " *" : ""}</span>
          <span class="t-label">{a?.version.name} · {a?.counts.sprites.toLocaleString()} sprites</span>
        </div>
        <span class="grow"></span>
        <button class="t-icon-btn" title="Undo the last change in A" disabled={!a?.canUndo || !!app.busy} onclick={commands.undo}><Icon name="undo" /></button>
      </div>
      <div class="client t-panel">
        <strong class="side">B</strong>
        <div class="who">
          <span class="name" title={b?.datPath}>{clientName(b)}{b?.changed ? " *" : ""}</span>
          <span class="t-label">{b?.version.name} · {b?.counts.sprites.toLocaleString()} sprites</span>
        </div>
        <span class="grow"></span>
        <button class="t-icon-btn" title="Open another client as B" onclick={change}><Icon name="open" /></button>
        <button class="t-icon-btn" title="Undo the last change in B" disabled={!b?.canUndo || !!app.busy} onclick={undoB}><Icon name="undo" /></button>
        <button class="t-icon-btn" title="Compile B to its files" disabled={!b?.changed || !b?.datPath || !!app.busy} onclick={compileB}><Icon name="save" /></button>
        <button class="t-icon-btn" title="Close B" onclick={closeB}><Icon name="close" /></button>
      </div>
    </div>

    <div class="row">
      <div class="t-tabs">
        {#each CATEGORIES as c}
          <button class="t-tab" class:on={category === c} onclick={() => setCategory(c)}>{CATEGORY_LABELS[c]}</button>
        {/each}
      </div>
      <span class="grow"></span>
      {#each Object.keys(STATUS_LABELS) as s}
        {@const st = s as Status}
        <label class="row filter {st}" title="Double click selects all {STATUS_LABELS[st].toLowerCase()}" ondblclick={() => selectStatus(st)}>
          <input class="t-check" type="checkbox" bind:checked={shown[st]} />{STATUS_LABELS[st]} <span class="n">{counts[st]}</span>
        </label>
      {/each}
      <label class="row filter" title="Hide objects that exist in one client only and have no sprites there">
        <input class="t-check" type="checkbox" bind:checked={hideEmpty} />Hide empty
      </label>
    </div>

    <div class="body">
      <div class="col main">
        <div class="list t-panel">
          {#if error}
            <div class="msg err">{error}</div>
          {:else if !diff}
            <div class="msg loading t-label"><span class="spinner"></span>Comparing clients…</div>
          {:else if diff && entries.length === 0}
            <div class="msg t-label">
              {diff.entries?.length ? "Nothing matches the filters." : `All ${diff.same.toLocaleString()} ${CATEGORY_LABELS[category].toLowerCase()} are the same.`}
            </div>
          {:else if diff}
            <VirtualGrid count={entries.length} cellWidth={4000} cellHeight={38} gap={1} {scrollTo} {onkeydown}>
              {#snippet cell(i)}
                {@const e = entries[i]}
                <button class="entry" class:sel={selected.includes(e.id)} onclick={(ev) => click(ev, e.id)} oncontextmenu={(ev) => oncontextmenu(ev, e.id)} ondblclick={() => dblclick(e)} onmouseenter={() => enter(e)} onmouseleave={leave}>
                  <span class="id">{e.id}</span>
                  <span class="t-slot">
                    {#if hoverA.current?.id === e.id}
                      <ThingCanvas thing={hoverA.current.thing} get={(sid) => spriteCache.get(sid)?.pixels} size={spriteCache.size} ready={hoverA.ready} group={HoverAnim.group(hoverA.current.thing)} fit={32} colorize={false} improved={improvedA} />
                    {:else if e.status !== "onlyB"}
                      {@const src = res.thumb(category, e.id, versions.thing(category, e.id))}
                      <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:imgLoading={src} />
                    {/if}
                  </span>
                  <Icon name="arrowE" />
                  <span class="t-slot">
                    {#if hoverB.current?.id === e.id}
                      <ThingCanvas thing={hoverB.current.thing} get={(sid) => otherSpriteCache.get(sid)?.pixels} size={otherSpriteCache.size} ready={hoverB.ready} group={HoverAnim.group(hoverB.current.thing)} fit={32} colorize={false} improved={improvedB} />
                    {:else if e.status !== "onlyA"}
                      {@const src = res.otherThumb(category, e.id, otherVersions.thing(category, e.id))}
                      <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:imgLoading={src} />
                    {/if}
                  </span>
                  {#if queuedIds.has(e.id)}<span class="tag" title="Queued for A (double click removes it)">queued</span>{/if}
                  <span class="state {e.status}">{e.status === "copied" ? "Copied" : STATUS_LABELS[e.status as Status]}</span>
                  <span class="t-label changes">{(e.changes ?? []).map((c) => CHANGE_LABELS[c] ?? c).join(", ")}</span>
                  {#if copied[e.id]}
                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <span class="revert" role="button" tabindex="-1" title="Revert this copy" onclick={(ev) => (ev.stopPropagation(), revert([e.id]))}><Icon name="undo" /></span>
                  {/if}
                </button>
              {/snippet}
            </VirtualGrid>
          {/if}
          {#if loading && diff}<span class="spinner corner"></span>{/if}
        </div>

        <div class="row actions">
          <span class="t-label">{selected.length ? `${picked.length} selected` : "Select objects to copy (Ctrl+A, Shift, Ctrl)."}</span>
          <span class="grow"></span>
          <button class="t-btn" title="Replace in A, or add with the same id" disabled={!fromB.length || !!app.busy} onclick={() => transfer(false, false, fromB)}
            >B → A</button
          >
          <button class="t-btn" title="Add to the end of A as new objects" disabled={!fromB.length || !!app.busy} onclick={() => transfer(false, true, fromB)}
            >Append to A</button
          >
          <button class="t-btn" title="Queue the selected objects of B, then click slots of A to drop them (double click a row queues it too)" disabled={!fromB.length} onclick={() => enqueue(category, fromB.map((e) => e.id))}
            >Queue</button
          >
          <span class="t-sep v"></span>
          <button class="t-btn" title="Replace in B, or add with the same id" disabled={!fromA.length || !!app.busy} onclick={() => transfer(true, false, fromA)}
            >A → B</button
          >
          <button class="t-btn" title="Add to the end of B as new objects" disabled={!fromA.length || !!app.busy} onclick={() => transfer(true, true, fromA)}
            >Append to B</button
          >
        </div>
      </div>
      <div class="side-panel"><DropPanel {category} onshow={show} /></div>
    </div>
  </div>
  {#snippet footer()}
    <button class="t-btn" title="Copy every changed object and every object only B has into A, in all categories" disabled={!!app.busy} onclick={mergeAll}
      >Merge all B into A…</button
    >
    <span class="t-label status">{diff ? `${diff.same.toLocaleString()} same` : ""}</span>
    <span class="grow"></span>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .compare {
    gap: 6px;
  }
  .clients {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }
  .client {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    min-width: 0;
  }
  .side {
    font-size: 16px;
    color: var(--gold);
    width: 14px;
    text-align: center;
  }
  .who {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .name {
    color: var(--text-bright);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .filter {
    gap: 4px;
    white-space: nowrap;
    cursor: pointer;
    user-select: none;
  }
  .n {
    color: var(--text-dim);
  }
  .list {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 340px;
    padding: 2px;
  }
  .entry {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    height: 100%;
    padding: 0 6px;
    border: 1px solid transparent;
    background: rgba(255, 255, 255, 0.02);
    font: inherit;
    color: var(--text);
    text-align: left;
    cursor: pointer;
  }
  .entry :global(*) {
    pointer-events: none;
  }
  .entry:hover {
    background: rgba(255, 255, 255, 0.06);
  }
  .entry.sel {
    background: var(--select);
    color: var(--text-bright);
  }
  .t-slot img {
    max-width: 32px;
    max-height: 32px;
    object-fit: contain;
  }
  .id {
    width: 48px;
    text-align: right;
    color: var(--text-bright);
  }
  .state {
    flex: none;
    width: 70px;
  }
  .changed {
    color: var(--gold);
  }
  .onlyA {
    color: #7fb2e5;
  }
  .onlyB {
    color: #8fd18f;
  }
  .state.copied {
    color: var(--text-dim);
  }
  .changes {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .revert {
    margin-left: auto;
    flex: none;
    display: flex;
    padding: 2px;
    color: var(--text-dim);
    pointer-events: auto !important;
    cursor: pointer;
  }
  .revert:hover {
    color: var(--text-bright);
  }
  .msg {
    margin: auto;
  }
  .loading {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .err {
    color: #ff9c9c;
  }
  .spinner {
    width: 12px;
    height: 12px;
    border: 2px solid rgba(232, 196, 106, 0.25);
    border-top-color: var(--gold);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .corner {
    position: absolute;
    right: 8px;
    bottom: 8px;
  }
  .actions {
    gap: 4px;
  }
  .actions > .t-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag {
    flex: none;
    padding: 0 4px;
    border: 1px solid var(--gold);
    color: var(--gold);
    font: var(--fs-small) / 12px var(--font-small);
  }
  .body {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 250px;
    gap: 8px;
  }
  .main {
    gap: 6px;
    min-width: 0;
  }
  .side-panel {
    position: relative;
  }
  .side-panel > :global(.drop) {
    position: absolute;
    inset: 0;
  }
  .t-sep.v {
    width: 1px;
    align-self: stretch;
    margin: 0 4px;
  }
</style>
