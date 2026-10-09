<script lang="ts">
  // Browses OBD files and images of a folder. OBD objects are rendered and
  // can be imported; images are shown as plain pictures, clearly marked as
  // not being objects, and can be imported as sprites.
  import { onMount, tick } from "svelte";
  import {
    CATEGORY_NAMES,
    Category,
    DialogService,
    SpriteService,
    ThingService,
    ViewerService,
    decodeBytes,
    errorMessage,
    normalizeThing,
    type ImageFile,
    type OBDFile,
    type Thing,
    type ViewerEntry,
  } from "../../lib/api";
  import { app, run, select, setCategory, toast } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import ThingCanvas from "../../lib/ui/ThingCanvas.svelte";
  import PanStage from "../../lib/ui/PanStage.svelte";
  import { ZOOM_LEVELS, stepZoom, zoomLabel } from "../../lib/pan.svelte";

  const FILTERS = [
    { name: "Objects and images (*.obd, *.png, *.bmp, *.gif, *.jpg)", pattern: "*.obd;*.png;*.bmp;*.gif;*.jpg;*.jpeg" },
    { name: "Object Builder Data (*.obd)", pattern: "*.obd" },
    { name: "Images (*.png, *.bmp, *.gif, *.jpg)", pattern: "*.png;*.bmp;*.gif;*.jpg;*.jpeg" },
  ];
  const DIRECTIONS = [
    ["arrowN", "North"],
    ["arrowE", "East"],
    ["arrowS", "South"],
    ["arrowW", "West"],
  ];

  let dir = $state("");
  let entries = $state<ViewerEntry[]>([]);
  let current = $state("");
  let list = $state<HTMLDivElement>();
  let listing = $state(false);

  let file = $state<OBDFile | null>(null);
  let thing = $state<Thing | null>(null);
  let image = $state<(ImageFile & { url: string }) | null>(null);
  let pixels = $state.raw<Uint8ClampedArray[]>([]);
  let ready = $state(0);
  let group = $state(0);
  let direction = $state(2);
  let zoom = $state(3);
  let error = $state("");

  const g = $derived(thing?.frameGroups[Math.min(group, thing.frameGroups.length - 1)] ?? null);
  const isOutfit = $derived(thing?.category === Category.CategoryOutfit);
  const index = $derived(entries.findIndex((e) => e.path === current));
  const entry = $derived(entries[index] ?? null);
  const name = $derived(current.replace(/^.*[\\/]/, ""));
  const counts = $derived({ obd: entries.filter((e) => e.kind === "obd").length, image: entries.filter((e) => e.kind === "image").length });

  onMount(() => {
    const path = app.obdPath;
    app.obdPath = "";
    if (path) void open(path);
    else void browse();
  });

  async function browse() {
    const paths = await DialogService.OpenFiles("View objects and images", FILTERS, false);
    if (paths?.length) await open(paths[0]);
    else if (!current) app.dialog = null;
  }

  // Windows paths compare without case and separator differences.
  const samePath = (a: string, b: string) => a.replace(/\//g, "\\").toLowerCase() === b.replace(/\//g, "\\").toLowerCase();

  /** Shows a file and lists the other files of its folder. */
  async function open(path: string) {
    const folder = path.replace(/[\\/][^\\/]*$/, "");
    if (!samePath(folder, dir)) {
      dir = folder;
      listing = true;
      try {
        entries = (await ViewerService.List(folder)) ?? [];
      } catch (e) {
        entries = [];
        toast(errorMessage(e), "error");
      } finally {
        listing = false;
      }
    }
    await show(entries.find((e) => samePath(e.path, path))?.path ?? path);
    await tick();
    list?.focus();
  }

  type Loaded =
    | { kind: "obd"; file: OBDFile; thing: Thing; pixels: Uint8ClampedArray[] }
    | { kind: "image"; image: ImageFile & { url: string } };

  // Recently read files, so browsing back and forth is instant. Neighbours
  // of the shown file are read ahead (slow drives make every read count).
  const CACHE_SIZE = 32;
  const cache = new Map<string, Promise<Loaded>>();

  function read(path: string): Promise<Loaded> {
    const hit = cache.get(path);
    if (hit) {
      cache.delete(path);
      cache.set(path, hit); // most recent last
      return hit;
    }
    const p: Promise<Loaded> = /\.obd$/i.test(path)
      ? ThingService.ReadOBD(path).then((f) => {
          if (!f?.thing) throw new Error("empty OBD file");
          return { kind: "obd", file: f, thing: normalizeThing(f.thing), pixels: (f.sprites ?? []).map((b) => decodeBytes(b as unknown as string)) };
        })
      : ViewerService.ReadImage(path).then((img) => {
          if (!img) throw new Error("empty image");
          return { kind: "image", image: { ...img, url: `data:image/png;base64,${img.png}` } };
        });
    p.catch(() => cache.delete(path));
    cache.set(path, p);
    while (cache.size > CACHE_SIZE) cache.delete(cache.keys().next().value!);
    return p;
  }

  function prefetch() {
    for (const d of [1, -1]) {
      const e = entries[index + d];
      if (e) read(e.path).catch(() => {});
    }
  }

  // The loader appears only when a read takes a moment, so fast reads do
  // not flash it.
  let loading = $state(false);
  let loaderTimer: ReturnType<typeof setTimeout> | undefined;

  async function show(path: string) {
    current = path;
    error = "";
    clearTimeout(loaderTimer);
    loaderTimer = setTimeout(() => current === path && (loading = true), 120);
    try {
      const r = await read(path);
      if (current !== path) return;
      if (r.kind === "image") {
        image = r.image;
        file = null;
        thing = null;
      } else {
        pixels = r.pixels;
        file = r.file;
        thing = r.thing;
        image = null;
        group = 0;
        direction = 2;
        ready++;
      }
    } catch (e) {
      if (current !== path) return;
      file = null;
      thing = null;
      image = null;
      error = errorMessage(e);
    } finally {
      if (current === path) {
        clearTimeout(loaderTimer);
        loading = false;
      }
    }
    list?.querySelector(".entry.on")?.scrollIntoView({ block: "nearest" });
    prefetch();
  }

  function onkeydown(e: KeyboardEvent) {
    const step: Record<string, number> = { ArrowUp: -1, ArrowDown: 1, PageUp: -10, PageDown: 10 };
    if (!(e.key in step) || !entries.length) return;
    e.preventDefault();
    const i = Math.min(Math.max((index < 0 ? 0 : index) + step[e.key], 0), entries.length - 1);
    void show(entries[i].path);
  }

  const get = (id: number) => (id > 0 ? pixels[id - 1] : undefined);
  const kb = (n: number) => (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`);

  async function importObject() {
    if (!file || !app.open) return;
    const res = await run("Importing", () => ThingService.ImportOBD([file!.path], 0));
    const r = res?.[0];
    if (!r) return;
    if (r.error) {
      toast(r.error, "error");
      return;
    }
    toast(`Imported as ${CATEGORY_NAMES[r.category]} #${r.id}.`, "success");
    if (r.category !== app.category) await setCategory(r.category);
    await select(r.id);
  }

  async function importImage() {
    if (!image || !app.open) return;
    const ids = await run("Importing", () => SpriteService.ImportImages([image!.path]));
    if (ids?.length) {
      toast(`Added ${ids.length} sprite(s) from ${name}.`, "success");
      app.selectedSprites = [ids[0]];
    } else if (ids) toast(`${name} has no visible pixels.`);
  }
</script>

<Dialog title="Object Viewer" width={880} onclose={() => (app.dialog = null)}>
  <div class="layout">
    <div class="files col">
      <div class="t-label folder" title={dir}>{dir || "No folder"}</div>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <div class="list t-panel" bind:this={list} tabindex="0" role="listbox" aria-label="Files" {onkeydown}>
        {#each entries as e (e.path)}
          <button class="entry" class:on={e.path === current} class:busy={loading && e.path === current} role="option" aria-selected={e.path === current} title={e.name} onclick={() => show(e.path)}>
            <span class="kind {e.kind}">{e.kind === "obd" ? "OBD" : "IMG"}</span>
            <span class="fname">{e.name}</span>
          </button>
        {:else}
          {#if listing}
            <div class="t-label none"><span class="spinner"></span>Reading folder…</div>
          {:else}
            <div class="t-label none">No OBD files or images here.</div>
          {/if}
        {/each}
      </div>
      <div class="t-label small">{counts.obd} OBD · {counts.image} image{counts.image === 1 ? "" : "s"} · ↑↓ to browse</div>
    </div>

    <div class="stage checker" class:dim={loading}>
      {#if loading}
        <div class="loader t-panel" role="status"><span class="spinner"></span>Loading {name}…</div>
      {/if}
      {#if thing || image}
        <PanStage {zoom} onzoom={(d) => (zoom = stepZoom(zoom, d))} resetKey={current}>
          {#if thing}
            <ThingCanvas {thing} {get} size={file?.spriteSize ?? 32} {ready} {group} {direction} />
          {:else if image}
            <img class="pixel" src={image.url} alt={name} />
          {/if}
        </PanStage>
      {:else if error}
        <span class="err">{error}</span>
      {/if}
    </div>

    <div class="side col">
      <strong class="name" title={current}>{name || "Nothing selected"}</strong>
      {#if image}
        <div class="notice">
          <Icon name="info" />
          <span>Not an OBD object. This is a plain {image.format.toUpperCase()} image; it can only be imported as sprites.</span>
        </div>
        <div class="info t-panel">
          <span class="t-label">Type</span><span>{image.format.toUpperCase()} image</span>
          <span class="t-label">Size</span><span>{image.width}×{image.height} px</span>
          {#if entry}<span class="t-label">File</span><span>{kb(entry.size)}</span>{/if}
        </div>
      {:else if thing && file && g}
        <div class="info t-panel">
          <span class="t-label">Category</span><span>{CATEGORY_NAMES[thing.category]}</span>
          <span class="t-label">OBD version</span><span>{file.version}</span>
          <span class="t-label">Client</span><span>{(file.clientVersion / 100).toFixed(2)}</span>
          <span class="t-label">Size</span><span>{g.width}×{g.height} ({file.spriteSize}px)</span>
          <span class="t-label">Layers</span><span>{g.layers}</span>
          <span class="t-label">Patterns</span><span>{g.patternX}×{g.patternY}×{g.patternZ}</span>
          <span class="t-label">Frames</span><span>{g.frames}</span>
          <span class="t-label">Sprites</span><span>{pixels.length}</span>
          {#if entry}<span class="t-label">File</span><span>{kb(entry.size)}</span>{/if}
        </div>
        {#if thing.frameGroups.length > 1}
          <div class="t-tabs">
            <button class="t-tab" class:on={group === 0} onclick={() => (group = 0)}>Idle</button>
            <button class="t-tab" class:on={group === 1} onclick={() => (group = 1)}>Walking</button>
          </div>
        {/if}
        {#if isOutfit && g.patternX > 1}
          <div class="row dirs">
            {#each DIRECTIONS.slice(0, g.patternX) as [icon, label], i}
              <button class="t-icon-btn" class:on={direction === i} title={label} onclick={() => (direction = i)}><Icon name={icon} /></button>
            {/each}
          </div>
        {/if}
      {/if}
      {#if thing || image}
        <div class="row">
          <button class="t-icon-btn" title="Zoom out (Ctrl+wheel)" disabled={zoom <= ZOOM_LEVELS[0]} onclick={() => (zoom = stepZoom(zoom, -1))}><Icon name="zoomout" /></button>
          <span class="t-label">{zoomLabel(zoom)}</span>
          <button class="t-icon-btn" title="Zoom in (Ctrl+wheel)" disabled={zoom >= ZOOM_LEVELS[ZOOM_LEVELS.length - 1]} onclick={() => (zoom = stepZoom(zoom, 1))}><Icon name="zoomin" /></button>
        </div>
      {/if}
    </div>
  </div>
  {#snippet footer()}
    <button class="t-btn" onclick={browse}>Open…</button>
    <span class="grow"></span>
    {#if image}
      <button class="t-btn primary" disabled={!app.open || !!app.busy} title={app.open ? "" : "Open a client to import"} onclick={importImage}>Import as sprites</button>
    {:else}
      <button class="t-btn primary" disabled={!file || !app.open || !!app.busy} title={app.open ? "" : "Open a client to import"} onclick={importObject}>Import object</button>
    {/if}
    <button class="t-btn" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .layout {
    display: flex;
    gap: 10px;
    height: 400px;
  }
  .files {
    width: 220px;
    flex: none;
    gap: 4px;
    min-height: 0;
  }
  .folder {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl;
    text-align: left;
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 2px;
    outline: none;
  }
  .list:focus-visible {
    box-shadow: 0 0 0 1px #6a6a6a;
  }
  .entry {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 2px 4px;
    border: 1px solid transparent;
    background: none;
    font: inherit;
    color: var(--text);
    text-align: left;
    cursor: pointer;
  }
  .entry:hover {
    background: rgba(255, 255, 255, 0.05);
  }
  .entry.on {
    background: var(--select);
    border-color: #b6b6b6;
    color: #fff;
  }
  .kind {
    flex: none;
    width: 26px;
    font: var(--fs-small) / 10px var(--font-small);
    text-align: center;
    padding: 1px 0;
    border: 1px solid currentColor;
  }
  .kind.obd {
    color: var(--green);
  }
  .kind.image {
    color: var(--gold);
  }
  .fname {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .none {
    padding: 8px;
  }
  .small {
    font: var(--fs-small) / 10px var(--font-small);
  }
  .stage {
    position: relative;
    flex: 1;
    min-width: 0;
    display: grid;
    place-items: center;
    overflow: hidden;
    border: 1px solid #1a1a1a;
  }
  /* Previous content stays visible but faded while the next file loads. */
  .stage.dim > :global(:not(.loader)) {
    opacity: 0.35;
  }
  .loader {
    position: absolute;
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 14px;
    color: var(--text-bright);
    max-width: 90%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .spinner {
    flex: none;
    display: inline-block;
    width: 12px;
    height: 12px;
    margin-right: 6px;
    border: 2px solid rgba(232, 196, 106, 0.25);
    border-top-color: var(--gold);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    vertical-align: -2px;
  }
  .loader .spinner {
    margin-right: 0;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .entry.busy .kind {
    animation: pulse 0.8s ease-in-out infinite alternate;
  }
  @keyframes pulse {
    to {
      opacity: 0.3;
    }
  }
  .side {
    width: 190px;
    flex: none;
    gap: 6px;
  }
  .name {
    color: var(--text-bright);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .notice {
    display: flex;
    gap: 6px;
    padding: 6px 8px;
    color: var(--gold);
    border: 1px solid rgba(232, 196, 106, 0.5);
    background: rgba(232, 196, 106, 0.08);
  }
  .notice :global(svg) {
    flex: none;
    margin-top: 1px;
  }
  .info {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2px 8px;
    padding: 6px 8px;
  }
  .dirs {
    gap: 2px;
  }
  .t-icon-btn.on {
    color: var(--text-bright);
    filter: brightness(1.3);
  }
  .err {
    color: #ff9c9c;
    padding: 12px;
  }
</style>
