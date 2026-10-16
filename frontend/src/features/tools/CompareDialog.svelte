<script lang="ts">
  // Compares the open client (A) with a second client (B) and copies objects
  // in both directions. Copies keep their ids (replacing, or extending the
  // target) or are appended as new objects. Objects of B can also be queued
  // and dropped into chosen slots of A (DropPanel, queue.svelte.ts).
  import { hasImprovedAnimations } from "../../lib/thing";
  import { CATEGORIES, CATEGORY_LABELS, CATEGORY_NAMES, CompareService, Format, normalizeThing, res, ThingService, type Category, type DiffEntry, type DiffResult } from "../../lib/api";
  import { mergeIds, mergeSummary, type MergeStep } from "../../lib/compare";
  import { clickSelection, sliceBetween } from "../../lib/util/selection";
  import { HoverAnim } from "../../lib/render/hoveranim.svelte";
  import { otherSpriteCache, spriteCache } from "../../lib/render/sprites";
  import { app, otherVersions, run, toast, versions } from "../../lib/state/app.svelte";
  import { ask } from "../../lib/confirm.svelte";
  import { commands } from "../../lib/commands";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import { compareQueue } from "../../lib/state/queue.svelte";
  import DropPanel from "./DropPanel.svelte";
  import { openContextMenu, type MenuEntry } from "../../lib/menu.svelte";
  import ThingCanvas from "../../lib/ui/ThingCanvas.svelte";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import { loading as imgLoading } from "../../lib/ui/loading";
  import Preview, { type PreviewSource } from "../Preview.svelte";

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
  const visible = $derived((diff?.entries ?? []).filter((entry) => !(hideEmpty && entry.empty)));
  let selected = $state.raw<number[]>([]);
  const selectedSet = $derived(new Set(selected));
  let anchor = $state<number | null>(null);

  const infoA = $derived(app.project?.info);
  const infoB = $derived(app.other?.info);
  const improvedA = $derived(hasImprovedAnimations(infoA));
  const improvedB = $derived(hasImprovedAnimations(infoB));

  // Hovering a row animates both of its thumbnails, like the object list.
  const hoverAnim = (get: typeof ThingService.Get) => async (id: number) => {
    const requested = category;
    const raw = await get(requested, id);
    return raw && requested === category ? normalizeThing(raw) : null;
  };
  const hoverA = new HoverAnim(spriteCache, hoverAnim(ThingService.Get));
  const hoverB = new HoverAnim(otherSpriteCache, hoverAnim(CompareService.Thing));
  function enter(entry: DiffEntry) {
    if (entry.status !== "onlyB") hoverA.enter(entry.id);
    if (entry.status !== "onlyA") hoverB.enter(entry.id);
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
    const list: DiffEntry[] = visible.filter((entry) => shown[entry.status as Status]);
    const listed = new Set((diff?.entries ?? []).map((entry) => entry.id));
    const extra = Object.entries(copied)
      .filter(([id]) => !listed.has(+id))
      .map(([id, to]) => ({ id: +id, status: "copied", changes: [`copied to ${to}`] }));
    return extra.length ? [...list, ...extra].sort((first, second) => first.id - second.id) : list;
  });
  const counts = $derived.by(() => {
    const out: Record<Status, number> = { changed: 0, onlyA: 0, onlyB: 0 };
    for (const entry of visible) out[entry.status as Status]++;
    return out;
  });
  const picked = $derived(entries.filter((entry) => selectedSet.has(entry.id)));
  const fromA = $derived(picked.filter((entry) => entry.status !== "onlyB" && entry.status !== "copied"));
  const fromB = $derived(picked.filter((entry) => entry.status !== "onlyA" && entry.status !== "copied"));
  const pickedCopies = $derived(picked.filter((entry) => copied[entry.id]).map((entry) => entry.id));

  // Compare again after every change of either client.
  let seq = 0;
  $effect(() => {
    const compared = category;
    void app.project?.rev;
    void app.other?.rev;
    const request = ++seq;
    loading = true;
    CompareService.Diff(compared)
      .then((result) => {
        if (request !== seq) return;
        diff = result;
        error = "";
        const ids = new Set((result.entries ?? []).map((entry) => entry.id));
        selected = selected.filter((id) => ids.has(id));
      })
      .catch((err) => request === seq && ((diff = null), (error = String(err?.message ?? err))))
      .finally(() => request === seq && (loading = false));
  });

  // The last clicked object is previewed, from A or B.
  const focused = $derived(entries.find((entry) => entry.id === anchor && selectedSet.has(entry.id)) ?? null);
  let side = $state<"A" | "B">("B");
  const hasA = $derived(!!focused && focused.status !== "onlyB");
  const hasB = $derived(!!focused && focused.status !== "onlyA");
  const shownSide = $derived(side === "B" ? (hasB ? "B" : "A") : hasA ? "A" : "B");
  let source = $state<PreviewSource | null>(null);
  let group = $state(0);
  let previewSeq = 0;
  $effect(() => {
    const entry = focused;
    const fromSide = shownSide;
    const previewed = category;
    void app.project?.rev;
    void app.other?.rev;
    const request = ++previewSeq;
    if (!entry || entry.status === "copied" || (fromSide === "A" ? !hasA : !hasB)) {
      source = null;
      return;
    }
    const cache = fromSide === "A" ? spriteCache : otherSpriteCache;
    void (async () => {
      try {
        const raw = await (fromSide === "A" ? ThingService.Get : CompareService.Thing)(previewed, entry.id);
        if (!raw || request !== previewSeq) return;
        const thing = normalizeThing(raw);
        await cache.load(thing.frameGroups.flatMap((frameGroup) => frameGroup.sprites));
        if (request !== previewSeq) return;
        source = {
          key: `${fromSide}:${previewed}:${entry.id}`,
          thing,
          get: (id) => cache.get(id)?.pixels,
          size: cache.size,
          improved: fromSide === "A" ? improvedA : improvedB,
        };
      } catch {
        if (request === previewSeq) source = null;
      }
    })();
  });

  function setCategory(chosen: Category) {
    category = chosen;
    copied = {};
    selected = [];
    anchor = null;
  }

  function click(event: MouseEvent, id: number) {
    const next = clickSelection(event, selected, anchor, id, (from) => {
      const fromIndex = entries.findIndex((entry) => entry.id === from);
      const toIndex = entries.findIndex((entry) => entry.id === id);
      return sliceBetween(entries, fromIndex, toIndex).map((entry) => entry.id);
    });
    selected = next.selection;
    anchor = next.anchor;
  }

  // Right click selects the row (unless it is selected) and offers the
  // actions for the selection.
  function oncontextmenu(event: MouseEvent, id: number) {
    if (!selectedSet.has(id)) {
      selected = [id];
      anchor = id;
    }
    const busy = () => !!app.busy;
    const unqueued = fromB.filter((entry) => !queuedIds.has(entry.id)).map((entry) => entry.id);
    const inQueue = picked.filter((entry) => queuedIds.has(entry.id)).map((entry) => entry.id);
    const items: MenuEntry[] = [
      { label: "B → A (same id)", action: () => transfer(false, false, fromB), disabled: () => !fromB.length || busy() },
      { label: "Append to A", action: () => transfer(false, true, fromB), disabled: () => !fromB.length || busy() },
      "-",
      { label: `Queue for A${unqueued.length > 1 ? ` (${unqueued.length})` : ""}`, keys: "Dbl click", action: () => compareQueue.enqueue(category, unqueued), disabled: () => !unqueued.length },
      ...(inQueue.length ? [{ label: "Remove from queue", action: () => compareQueue.dequeue(category, inQueue) }] : []),
      "-",
      { label: "A → B (same id)", action: () => transfer(true, false, fromA), disabled: () => !fromA.length || busy() },
      { label: "Append to B", action: () => transfer(true, true, fromA), disabled: () => !fromA.length || busy() },
      ...(pickedCopies.length ? (["-", { label: "Revert copy", action: () => revert(pickedCopies), disabled: busy }] as MenuEntry[]) : []),
      "-",
      { label: "Copy id", action: () => navigator.clipboard?.writeText(picked.map((entry) => entry.id).join(", ")).then(() => toast("Copied id(s).")) },
    ];
    openContextMenu(event, items);
  }

  // Selects an id in the list and scrolls to it (from the queue and slots).
  let scrollTo = $state<number | null>(null);
  function show(id: number) {
    const position = entries.findIndex((entry) => entry.id === id);
    if (position < 0) {
      toast(`#${id} is not listed: it is the same in both clients or hidden by the filters.`, "info", 3500, "compare-show");
      return;
    }
    selected = [id];
    anchor = id;
    scrollTo = null;
    queueMicrotask(() => (scrollTo = position));
  }

  function onkeydown(event: KeyboardEvent) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "a") {
      event.preventDefault();
      selected = entries.map((entry) => entry.id);
    }
  }

  function selectStatus(status: Status) {
    selected = entries.filter((entry) => entry.status === status).map((entry) => entry.id);
  }

  /** Copies the picked objects; toOther copies A into B. */
  async function transfer(toOther: boolean, appendNew: boolean, list: DiffEntry[]) {
    if (!list.length) return;
    const ids = list.map((entry) => entry.id);
    const target = toOther ? "B" : "A";
    const result = await run(`Copying to ${target}`, () => CompareService.Transfer(toOther, category, ids, appendNew));
    if (!result) return;
    if (!appendNew) for (const id of ids) copied[id] = target;
    const what = `${ids.length} ${CATEGORY_NAMES[category]}(s)`;
    toast(`Copied ${what} to ${target}${appendNew ? ` as #${result.ids?.[0]}…` : ""}, ${result.sprites} new sprite(s).`, "success");
  }

  const queuedIds = $derived(new Set(compareQueue.queued(category)));

  // Double click queues the object of B (or takes it out of the queue).
  function dblclick(entry: DiffEntry) {
    if (entry.status === "onlyA" || entry.status === "copied") return;
    if (queuedIds.has(entry.id)) compareQueue.dequeue(category, [entry.id]);
    else compareQueue.enqueue(category, [entry.id]);
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
    const plan: MergeStep[] = [];
    for (const mergedCategory of CATEGORIES) {
      const result = await run("Comparing", () => CompareService.Diff(mergedCategory));
      if (!result) return;
      const ids = mergeIds(result.entries, hideEmpty);
      if (ids.length) plan.push({ category: mergedCategory, ids });
    }
    if (!plan.length) {
      toast("A already has everything B has.");
      return;
    }
    const summary = mergeSummary(plan);
    const ok = await ask({
      title: "Merge B into A",
      message: `Copy ${summary} from B into A? Changed objects are replaced, new ones keep their ids. Each category can be undone separately.`,
      ok: "Merge",
    });
    if (!ok) return;
    let sprites = 0;
    for (const step of plan) {
      const result = await run(`Merging ${CATEGORY_LABELS[step.category].toLowerCase()}`, () => CompareService.Transfer(false, step.category, step.ids, false));
      if (!result) return;
      if (step.category === category) for (const id of step.ids) copied[id] = "A";
      sprites += result.sprites;
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
    if (infoB?.changed && !(await ask({ title: "Close client B", message: "Client B has uncompiled changes. Close it anyway?", ok: "Close" }))) return;
    app.dialog = null;
    await CompareService.Close();
  }

  function clientName(info: typeof infoA): string {
    if (!info) return "";
    if (!info.datPath) return "new client";
    const parts = info.datPath.split(/[\\/]/);
    return parts.slice(-2).join("/");
  }
</script>

<Dialog title="Compare & Merge Clients" width={1000} onclose={() => (app.dialog = null)}>
  <div class="col compare">
    <div class="clients">
      <div class="client t-panel">
        <strong class="side">A</strong>
        <div class="who">
          <span class="name" title={infoA?.datPath}>{clientName(infoA)}{infoA?.changed ? " *" : ""}</span>
          <span class="t-label">{infoA?.version.name} · {infoA?.counts.sprites.toLocaleString()} sprites</span>
        </div>
        <span class="grow"></span>
        <button class="t-icon-btn" title="Undo the last change in A" disabled={!infoA?.canUndo || !!app.busy} onclick={commands.undo}><Icon name="undo" /></button>
      </div>
      <div class="client t-panel">
        <strong class="side">B</strong>
        <div class="who">
          <span class="name" title={infoB?.datPath}>{clientName(infoB)}{infoB?.changed ? " *" : ""}</span>
          <span class="t-label">{infoB?.version.name} · {infoB?.counts.sprites.toLocaleString()} sprites</span>
        </div>
        <span class="grow"></span>
        <button class="t-icon-btn" title="Open another client as B" onclick={change}><Icon name="open" /></button>
        <button class="t-icon-btn" title="Undo the last change in B" disabled={!infoB?.canUndo || !!app.busy} onclick={undoB}><Icon name="undo" /></button>
        <button class="t-icon-btn" title="Compile B to its files" disabled={!infoB?.changed || !infoB?.datPath || !!app.busy} onclick={compileB}><Icon name="save" /></button>
        <button class="t-icon-btn" title="Close B" onclick={closeB}><Icon name="close" /></button>
      </div>
    </div>

    <div class="row">
      <div class="t-tabs">
        {#each CATEGORIES as tabCategory}
          <button class="t-tab" class:on={category === tabCategory} onclick={() => setCategory(tabCategory)}>{CATEGORY_LABELS[tabCategory]}</button>
        {/each}
      </div>
      <span class="grow"></span>
      {#each Object.keys(STATUS_LABELS) as statusKey}
        {@const status = statusKey as Status}
        <label class="row filter {status}" title="Double click selects all {STATUS_LABELS[status].toLowerCase()}" ondblclick={() => selectStatus(status)}>
          <input class="t-check" type="checkbox" bind:checked={shown[status]} />{STATUS_LABELS[status]} <span class="n">{counts[status]}</span>
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
            <div class="msg loading t-label"><span class="t-spinner"></span>Comparing clients…</div>
          {:else if diff && entries.length === 0}
            <div class="msg t-label">
              {diff.entries?.length ? "Nothing matches the filters." : `All ${diff.same.toLocaleString()} ${CATEGORY_LABELS[category].toLowerCase()} are the same.`}
            </div>
          {:else if diff}
            <VirtualGrid count={entries.length} cellWidth={4000} cellHeight={38} gap={1} {scrollTo} {onkeydown}>
              {#snippet cell(i)}
                {@const entry = entries[i]}
                <button class="entry" class:sel={selectedSet.has(entry.id)} onclick={(event) => click(event, entry.id)} oncontextmenu={(event) => oncontextmenu(event, entry.id)} ondblclick={() => dblclick(entry)} onmouseenter={() => enter(entry)} onmouseleave={leave}>
                  <span class="id">{entry.id}</span>
                  <span class="t-slot">
                    {#if hoverA.current?.id === entry.id}
                      <ThingCanvas thing={hoverA.current.thing} get={(spriteId) => spriteCache.get(spriteId)?.pixels} size={spriteCache.size} ready={hoverA.ready} group={HoverAnim.group(hoverA.current.thing)} fit={32} colorize={false} improved={improvedA} />
                    {:else if entry.status !== "onlyB"}
                      {@const src = res.thumb(category, entry.id, versions.thing(category, entry.id))}
                      <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:imgLoading={src} />
                    {/if}
                  </span>
                  <Icon name="arrowE" />
                  <span class="t-slot">
                    {#if hoverB.current?.id === entry.id}
                      <ThingCanvas thing={hoverB.current.thing} get={(spriteId) => otherSpriteCache.get(spriteId)?.pixels} size={otherSpriteCache.size} ready={hoverB.ready} group={HoverAnim.group(hoverB.current.thing)} fit={32} colorize={false} improved={improvedB} />
                    {:else if entry.status !== "onlyA"}
                      {@const src = res.otherThumb(category, entry.id, otherVersions.thing(category, entry.id))}
                      <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:imgLoading={src} />
                    {/if}
                  </span>
                  {#if queuedIds.has(entry.id)}<span class="tag" title="Queued for A (double click removes it)">queued</span>{/if}
                  <span class="state {entry.status}">{entry.status === "copied" ? "Copied" : STATUS_LABELS[entry.status as Status]}</span>
                  <span class="t-label changes" title={(entry.changes ?? []).map((change) => CHANGE_LABELS[change] ?? change).join(", ")}>{(entry.changes ?? []).map((change) => CHANGE_LABELS[change] ?? change).join(", ")}</span>
                  {#if copied[entry.id]}
                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <span class="revert" role="button" tabindex="-1" title="Revert this copy" onclick={(event) => (event.stopPropagation(), revert([entry.id]))}><Icon name="undo" /></span>
                  {/if}
                </button>
              {/snippet}
            </VirtualGrid>
          {/if}
          {#if loading && diff}<span class="t-spinner corner"></span>{/if}
        </div>

      </div>
      <div class="col view">
        <div class="row">
          <div class="t-tabs">
            <button class="t-tab" class:on={focused && shownSide === "A"} disabled={!hasA} onclick={() => (side = "A")}>A</button>
            <button class="t-tab" class:on={focused && shownSide === "B"} disabled={!hasB} onclick={() => (side = "B")}>B</button>
          </div>
          <strong class="pname">{focused ? `#${focused.id}${source?.thing.name ? ` ${source.thing.name}` : ""}` : ""}</strong>
        </div>
        {#if source}
          <Preview {source} bind:group />
        {:else}
          <div class="stage t-panel t-label">{focused?.status === "copied" ? "Copied: the object is the same in both clients now." : "Click an object to preview it."}</div>
        {/if}
      </div>
      <div class="side-panel"><DropPanel queue={compareQueue} {category} onshow={show} onshowSlot={show} /></div>
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
      <button class="t-btn" title="Queue the selected objects of B, then click slots of A to drop them (double click a row queues it too)" disabled={!fromB.length} onclick={() => compareQueue.enqueue(category, fromB.map((event) => event.id))}
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
    flex: 1;
    min-height: 0;
    padding: 2px;
  }
  .entry {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    height: 100%;
    padding: 0 4px;
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
    width: 42px;
    text-align: right;
    color: var(--text-bright);
  }
  .state {
    flex: none;
    width: 64px;
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
    grid-template-columns: 440px minmax(0, 1fr) 212px;
    gap: 8px;
    height: 400px;
  }
  .view {
    gap: 4px;
    min-width: 0;
    min-height: 0;
  }
  .pname {
    color: var(--text-bright);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .stage {
    flex: 1;
    display: grid;
    place-items: center;
    padding: 12px;
    text-align: center;
  }
  .main {
    gap: 6px;
    min-width: 0;
    min-height: 0;
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
