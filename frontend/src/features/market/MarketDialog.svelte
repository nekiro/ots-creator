<script lang="ts">
  // Browses the shared market (see internal/market): sections, tags,
  // search and sort run on the downloaded index; objects are previewed live
  // and imported into the open client. Own entries (and, with the admin
  // token, every entry) can be deleted. Share opens the share window with a
  // picker for an object or sprites.
  import { onMount } from "svelte";
  import { MarketService, decodeBytes, errorMessage, normalizeThing, type MarketIndex, type OBDFile, type Thing } from "../../lib/api";
  import { commands } from "../../lib/commands";
  import { ask } from "../../lib/confirm.svelte";
  import { filterEntries, LICENSES, sectionCounts, sectionOf, topTags, type MarketEntry, type MarketSort } from "../../lib/market";
  import { app, run, select, setCategory, toast } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import TagPicker from "../../lib/ui/TagPicker.svelte";
  import ThingCanvas from "../../lib/ui/ThingCanvas.svelte";

  const SECTIONS = [
    ["all", "All"],
    ["item", "Items"],
    ["outfit", "Outfits"],
    ["effect", "Effects"],
    ["missile", "Missiles"],
    ["sprites", "Sprites"],
  ];
  const SECTION_NAMES: Record<string, string> = { item: "Item", outfit: "Outfit", effect: "Effect", missile: "Missile", sprites: "Sprites" };
  const SORTS = [
    { value: "new" as MarketSort, label: "Newest" },
    { value: "old" as MarketSort, label: "Oldest" },
    { value: "name" as MarketSort, label: "Name" },
  ];
  const PAGE = 120;

  let index = $state<MarketIndex | null>(null);
  let loading = $state(false);
  let error = $state("");
  let section = $state("all");
  let tags = $state<string[]>([]);
  let query = $state("");
  let sort = $state<MarketSort>("new");
  let sameSize = $state(true);
  let shown = $state(PAGE);
  let selectedId = $state("");

  const size = $derived(app.project?.info.features.spriteSize ?? 0);
  const entries = $derived((index?.entries ?? []) as MarketEntry[]);
  const filtered = $derived(filterEntries(entries, { section, tags, query, sort, spriteSize: sameSize ? size : 0 }));
  const counts = $derived(sectionCounts(entries));
  // Every tag used in the section, most used first, for the tag filter.
  const tagOptions = $derived(topTags(section === "all" ? entries : entries.filter((e) => sectionOf(e) === section), 1000));
  const selected = $derived(entries.find((e) => e.id === selectedId) ?? null);
  const url = (path: string) => (index ? index.base + path : "");
  const canDelete = $derived(!!selected && !!index && (index.admin || (index.mine ?? []).includes(selected.id)));

  $effect(() => {
    void section, tags, query, sort, sameSize;
    shown = PAGE;
  });

  onMount(() => void load(false));

  async function load(refresh: boolean) {
    loading = true;
    error = "";
    try {
      index = await MarketService.Index(refresh);
    } catch (e) {
      error = errorMessage(e);
    } finally {
      loading = false;
    }
  }

  function toggleTag(t: string) {
    tags = tags.includes(t) ? tags.filter((x) => x !== t) : [...tags, t];
  }

  // Live preview of the selected object (downloaded once per entry).
  type Loaded = { file: OBDFile; thing: Thing; pixels: Uint8ClampedArray[] };
  const cache = new Map<string, Promise<Loaded>>();
  let loaded = $state<Loaded | null>(null);
  let previewError = $state("");
  let previewLoading = $state(false);
  let group = $state(0);
  let direction = $state(2);
  let ready = $state(0);

  function read(id: string): Promise<Loaded> {
    let p = cache.get(id);
    if (!p) {
      p = MarketService.Read(id).then((f) => {
        if (!f?.thing) throw new Error("empty object");
        return { file: f, thing: normalizeThing(f.thing), pixels: (f.sprites ?? []).map((b) => decodeBytes(b as unknown as string)) };
      });
      p.catch(() => cache.delete(id));
      cache.set(id, p);
    }
    return p;
  }

  $effect(() => {
    const e = selected;
    loaded = null;
    previewError = "";
    group = 0;
    direction = 2;
    if (!e || e.kind !== "object") return;
    previewLoading = true;
    read(e.id)
      .then((r) => {
        if (selectedId !== e.id) return;
        loaded = r;
        ready++;
      })
      .catch((err) => selectedId === e.id && (previewError = errorMessage(err)))
      .finally(() => selectedId === e.id && (previewLoading = false));
  });

  const isOutfit = $derived(selected?.category === "outfit" && (loaded?.thing.frameGroups[0]?.patternX ?? 0) >= 4);
  const get = (id: number) => (id > 0 ? loaded?.pixels[id - 1] : undefined);
  const kb = (n: number) => (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`);
  const date = (s: string) => new Date(s).toLocaleDateString();
  const licenseLabel = (v: string) => LICENSES.find((l) => l.value === v)?.label ?? v;
  const sizeMismatch = $derived(!!selected && !!size && !!selected.spriteSize && selected.spriteSize !== size);

  async function importEntry() {
    const e = selected;
    if (!e || !app.open) return;
    const r = await run(`Importing ${e.name}`, () => MarketService.Import(e.id));
    if (!r) return;
    if (r.kind === "sprites") {
      toast(`Added ${r.sprites?.length ?? 0} sprite(s) from ${e.name}.`, "success");
      if (r.sprites?.length) app.selectedSprites = [r.sprites[0]];
      return;
    }
    toast(`Imported ${e.name} as ${SECTION_NAMES[e.category ?? ""]?.toLowerCase()} #${r.id}.`, "success");
    if (r.category !== app.category) await setCategory(r.category);
    await select(r.id);
  }

  async function deleteEntry() {
    const e = selected;
    if (!e) return;
    const own = (index?.mine ?? []).includes(e.id);
    const ok = await ask({
      title: "Delete from market",
      message: own ? `Delete ${e.name} from the market? Everyone loses access to it.` : `Delete ${e.name} by ${e.author} as admin?`,
      ok: "Delete",
    });
    if (!ok) return;
    const done = await run(`Deleting ${e.name}`, () => MarketService.Delete(e.id).then(() => true));
    if (!done) return;
    toast(`${e.name} was deleted from the market.`, "success");
    selectedId = "";
    await load(false);
  }

  function onkeydown(e: KeyboardEvent) {
    const step: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1 };
    if (!(e.key in step) || !filtered.length || (e.target as HTMLElement).closest("input")) return;
    e.preventDefault();
    const i = filtered.findIndex((x) => x.id === selectedId);
    selectedId = filtered[Math.min(Math.max(i + step[e.key], 0), filtered.length - 1)].id;
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<Dialog title="Market" width={840} onclose={() => (app.dialog = null)}>
  <div class="market col" {onkeydown}>
    <div class="t-tabs sections">
      {#each SECTIONS as [key, label]}
        <button class="t-tab" class:on={section === key} onclick={() => (section = key)}>
          {label}<span class="count">{counts[key] ?? 0}</span>
        </button>
      {/each}
    </div>

    <div class="row tools">
      <label class="search t-input">
        <Icon name="search" />
        <input placeholder="Search names, tags, authors…" bind:value={query} />
        {#if query}<button class="clear" title="Clear" onclick={() => (query = "")}><Icon name="close" /></button>{/if}
      </label>
      <Select value={sort} options={SORTS} width={84} onchange={(v) => (sort = v)} />
      {#if size}
        <button class="t-btn" class:on={sameSize} title="Hide entries with other sprite sizes" onclick={() => (sameSize = !sameSize)}>{size}px only</button>
      {/if}
      <button class="t-icon-btn" title="Reload the market" disabled={loading} onclick={() => load(true)}><Icon name="refresh" /></button>
    </div>

    <TagPicker value={tags} options={tagOptions} placeholder="Filter by tags…" onchange={(t) => (tags = t)} />

    <div class="body">
      <div class="list t-panel">
        {#if loading && !index}
          <div class="msg"><span class="spinner"></span>Loading the market…</div>
        {:else if error}
          <div class="msg col">
            <span class="err">{error}</span>
            <button class="t-btn" onclick={() => load(true)}>Try again</button>
          </div>
        {:else if filtered.length === 0}
          <div class="msg t-label">{entries.length ? "Nothing matches the filters." : "Nothing here yet. Be the first to share something!"}</div>
        {:else}
          <div class="grid">
            {#each filtered.slice(0, shown) as e (e.id)}
              <button class="card" class:on={e.id === selectedId} title={e.name} onclick={() => (selectedId = e.id)} ondblclick={importEntry}>
                <span class="thumb checker"><img class="pixel" src={url(e.preview)} alt="" loading="lazy" /></span>
                <span class="cname">{e.name}</span>
                <span class="cmeta">{SECTION_NAMES[sectionOf(e)]}</span>
              </button>
            {/each}
          </div>
          {#if filtered.length > shown}
            <button class="t-btn more" onclick={() => (shown += PAGE)}>Show more ({filtered.length - shown})</button>
          {/if}
        {/if}
      </div>

      <aside class="detail col">
        {#if selected}
          <div class="stage checker">
            {#if loaded}
              <ThingCanvas thing={loaded.thing} {get} size={loaded.file.spriteSize} {ready} {group} {direction} zoom={2} />
            {:else if selected.kind === "sprites"}
              <img class="pixel pack" src={url(selected.file)} alt={selected.name} />
            {:else if previewError}
              <span class="err">{previewError}</span>
            {:else}
              <img class="pixel big" src={url(selected.preview)} alt={selected.name} />
              {#if previewLoading}<span class="spinner corner"></span>{/if}
            {/if}
          </div>
          {#if loaded && (loaded.thing.frameGroups.length > 1 || isOutfit)}
            <div class="row view">
              {#if loaded.thing.frameGroups.length > 1}
                <div class="t-tabs">
                  <button class="t-tab" class:on={group === 0} onclick={() => (group = 0)}>Idle</button>
                  <button class="t-tab" class:on={group === 1} onclick={() => (group = 1)}>Walk</button>
                </div>
              {/if}
              <span class="grow"></span>
              {#if isOutfit}
                {#each ["arrowN", "arrowE", "arrowS", "arrowW"] as icon, i}
                  <button class="t-icon-btn dir" class:on={direction === i} onclick={() => (direction = i)}><Icon name={icon} /></button>
                {/each}
              {/if}
            </div>
          {/if}
          <div class="head">
            <strong class="name" title={selected.name}>{selected.name}</strong>
            <span class="by">by <b>{selected.author}</b> · {date(selected.created)}</span>
          </div>
          {#if selected.description}<p class="desc" title={selected.description}>{selected.description}</p>{/if}
          {#if selected.tags?.length}
            <div class="dtags">
              {#each selected.tags as t}<button class="tag" class:on={tags.includes(t)} onclick={() => toggleTag(t)}>{t}</button>{/each}
            </div>
          {/if}
          <div class="info">
            <span class="t-label">Kind</span><span>{SECTION_NAMES[sectionOf(selected)]}</span>
            {#if selected.kind === "object"}
              <span class="t-label">Size</span><span>{selected.width}×{selected.height} · {selected.spriteSize}px</span>
              <span class="t-label">Frames</span><span>{selected.frames}</span>
            {:else}
              <span class="t-label">Sprites</span><span>{selected.sprites} · {selected.spriteSize}px</span>
            {/if}
            <span class="t-label">License</span><span title={selected.license}>{licenseLabel(selected.license)}</span>
            <span class="t-label">File</span><span>{kb(selected.bytes)}</span>
          </div>
          {#if sizeMismatch}<p class="err small">Uses {selected.spriteSize}px sprites; this client uses {size}px.</p>{/if}
          <button
            class="t-btn primary wide"
            disabled={!app.open || sizeMismatch || !!app.busy}
            title={app.open ? "Double clicking a card does the same" : "Open a client to import"}
            onclick={importEntry}>{selected.kind === "sprites" ? "Import sprites" : "Import object"}</button
          >
          {#if canDelete}<button class="t-btn danger wide" disabled={!!app.busy} onclick={deleteEntry}>Delete from market</button>{/if}
        {:else}
          <div class="empty">
            <Icon name="market" />
            <span>Pick an entry to preview it.</span>
            <span class="t-label small">Double click imports it.</span>
          </div>
        {/if}
      </aside>
    </div>
  </div>
  {#snippet footer()}
    <button class="t-btn share" title="Share an object, sprites or a file" onclick={() => commands.sharePick()}
      ><Icon name="export" />Share…</button
    >
    <span class="t-label status">{filtered.length} of {entries.length}{loading && index ? " · reloading…" : ""}</span>
    <span class="grow"></span>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .market {
    gap: 6px;
    outline: none;
  }
  .sections .count {
    margin-left: 5px;
    color: var(--text-dim);
    font: var(--fs-small) / 10px var(--font-small);
  }
  .tools {
    gap: 4px;
  }
  .search {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 0 4px 0 6px;
    color: var(--text-dim);
  }
  .search input {
    flex: 1;
    min-width: 0;
    border: 0;
    background: none;
    outline: none;
    font: inherit;
    color: var(--text-bright);
  }
  .clear {
    display: grid;
    place-items: center;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-dim);
    cursor: pointer;
  }
  .clear:hover {
    color: var(--text-bright);
  }
  .tag {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px;
    border: 1px solid #3a3a3a;
    background: rgba(0, 0, 0, 0.25);
    font: var(--fs-small) / 11px var(--font-small);
    color: var(--text);
    cursor: pointer;
  }
  .tag:hover {
    border-color: #6a6a6a;
    color: var(--text-bright);
  }
  .tag.on {
    border-color: var(--gold);
    color: var(--gold);
    background: rgba(232, 196, 106, 0.08);
  }
  .body {
    display: flex;
    gap: 8px;
    height: 400px;
  }
  .list {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    padding: 3px;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, 78px);
    gap: 2px;
  }
  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
    min-width: 0;
    padding: 2px 2px 1px;
    border: 1px solid transparent;
    background: rgba(255, 255, 255, 0.025);
    font: inherit;
    color: var(--text);
    cursor: pointer;
  }
  .card:hover {
    background: rgba(255, 255, 255, 0.06);
    border-color: #4a4a4a;
  }
  .card.on {
    border-color: var(--gold);
    background: rgba(232, 196, 106, 0.1);
  }
  .thumb {
    width: 64px;
    height: 64px;
    display: grid;
    place-items: center;
    overflow: hidden;
    margin-bottom: 1px;
  }
  .thumb img {
    max-width: 64px;
    max-height: 64px;
    min-width: 32px;
    min-height: 32px;
    object-fit: contain;
  }
  .cname,
  .cmeta {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cname {
    color: var(--text-bright);
  }
  .card.on .cname {
    color: var(--gold);
  }
  .cmeta {
    font: var(--fs-small) / 10px var(--font-small);
    color: var(--text-dim);
  }
  .more {
    display: block;
    margin: 8px auto 2px;
  }
  .msg {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    text-align: center;
  }
  .detail {
    width: 220px;
    flex: none;
    gap: 6px;
    min-height: 0;
    overflow: hidden;
  }
  /* The preview takes what the text leaves, so the panel never scrolls. */
  .stage {
    position: relative;
    flex: 1 1 150px;
    min-height: 90px;
    display: grid;
    place-items: center;
    overflow: hidden;
    border: 1px solid #1a1a1a;
  }
  .big {
    zoom: 2;
  }
  .pack {
    max-width: 100%;
    max-height: 100%;
  }
  .corner {
    position: absolute;
    right: 6px;
    top: 6px;
  }
  .view {
    gap: 2px;
  }
  .dir {
    width: 20px;
    height: 20px;
  }
  .t-icon-btn.on {
    color: var(--text-bright);
    filter: brightness(1.3);
  }
  .head {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .name {
    color: var(--text-bright);
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .by {
    color: var(--text-dim);
  }
  .by b {
    color: var(--gold);
    font-weight: normal;
  }
  .desc {
    flex: none;
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .dtags {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
  }
  .info {
    flex: none;
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2px 8px;
    padding: 5px 7px;
    border: 1px solid #2a2a2a;
    background: rgba(0, 0, 0, 0.2);
  }
  .info span:nth-child(even) {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .wide {
    width: 100%;
    flex: none;
  }
  .view,
  .head,
  .dtags {
    flex: none;
  }
  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    color: var(--text-dim);
    text-align: center;
  }
  .empty :global(svg) {
    width: 32px;
    height: 32px;
    opacity: 0.5;
  }
  .share {
    display: flex;
    align-items: center;
    gap: 5px;
    color: var(--gold);
  }
  .status {
    align-self: center;
    margin-left: 6px;
  }
  .small {
    font: var(--fs-small) / 10px var(--font-small);
  }
  .err {
    color: #ff9c9c;
    margin: 0;
  }
  .spinner {
    display: inline-block;
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
</style>
