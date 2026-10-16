<script lang="ts">
  // Browses OBD files and images of a folder. OBD objects are rendered and
  // can be imported; images are shown as plain pictures, clearly marked as
  // not being objects, and can be imported as sprites. Objects can also be
  // queued and dropped into chosen slots of the client (DropPanel,
  // queue.svelte.ts).
  import { frameGroupAt } from "../../lib/thing";
  import { onMount, tick } from "svelte";
  import { SvelteMap } from "svelte/reactivity";
  import {
    OBJECT_EXT,
    CATEGORIES,
    CATEGORY_LABELS,
    CATEGORY_NAMES,
    Category,
    DialogService,
    SpriteService,
    ThingService,
    ViewerService,
    errorMessage,
    type ImageFile,
    type ObjectFile,
    type Thing,
    type ViewerEntry,
  } from "../../lib/api";
  import { app, run, toast } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Preview, { type PreviewSource } from "../Preview.svelte";
  import PanStage from "../../lib/ui/PanStage.svelte";
  import { ZOOM_LEVELS, stepZoom, zoomLabel } from "../../lib/render/zoom";
  import { compose } from "../../lib/render/compose";
  import { contentBox } from "../../lib/render/bounds";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import { DEFAULT_COLORS } from "../../lib/render/outfit";
  import { fileQueue } from "../../lib/state/queue.svelte";
  import DropPanel from "./DropPanel.svelte";
  import { formatKb, plural } from "../../lib/util/format";
  import { basename, dirname } from "../../lib/util/path";
  import { IMAGE_FILTERS, OBJECT_FILTERS } from "../../lib/commands/shared";
  import { LruCache } from "../../lib/util/lruCache";
  import { decodeObjectFile } from "../../lib/objectFile";
  import { nextIndex } from "../../lib/ui/listNav";

  const FILTERS = [
    { name: "Objects and images (*.otobj, *.obd, *.png, *.bmp, *.gif, *.jpg)", pattern: "*.otobj;*.obd;*.png;*.bmp;*.gif;*.jpg;*.jpeg" },
    OBJECT_FILTERS[0],
    ...IMAGE_FILTERS,
  ];

  let dir = $state("");
  let entries = $state<ViewerEntry[]>([]);
  let current = $state("");
  let list = $state<HTMLDivElement>();
  let scrollTo = $state<number | null>(null);
  let listing = $state(false);

  let file = $state<ObjectFile | null>(null);
  let thing = $state<Thing | null>(null);
  let image = $state<(ImageFile & { url: string }) | null>(null);
  let pixels = $state.raw<Uint8ClampedArray[]>([]);
  let group = $state(0);
  let zoom = $state(3);
  let error = $state("");

  const frameGroup = $derived(frameGroupAt(thing, group));
  const get = (id: number) => (id > 0 ? pixels[id - 1] : undefined);
  const source = $derived<PreviewSource | null>(thing && file ? { key: current, thing, get, size: file.spriteSize } : null);
  const index = $derived(entries.findIndex((candidate) => candidate.path === current));
  const entry = $derived(entries[index] ?? null);
  const name = $derived(basename(current));
  const counts = $derived({ obd: entries.filter((candidate) => candidate.kind === "obd").length, image: entries.filter((candidate) => candidate.kind === "image").length });

  onMount(() => {
    const path = app.obdPath;
    app.obdPath = "";
    if (path) void open(path);
  });

  async function browse() {
    const paths = await DialogService.OpenFiles("objectViewer", "View objects and images", FILTERS, false);
    if (paths?.length) await open(paths[0]);
  }

  // Windows paths compare without case and separator differences.
  const samePath = (first: string, second: string) => first.replace(/\//g, "\\").toLowerCase() === second.replace(/\//g, "\\").toLowerCase();

  /** Shows a file and lists the other files of its folder. */
  async function open(path: string) {
    const folder = dirname(path);
    if (!samePath(folder, dir)) {
      dir = folder;
      wanted.length = 0;
      listing = true;
      try {
        entries = (await ViewerService.List(folder)) ?? [];
      } catch (err) {
        entries = [];
        toast(errorMessage(err), "error");
      } finally {
        listing = false;
      }
    }
    await show(entries.find((candidate) => samePath(candidate.path, path))?.path ?? path);
    await tick();
    list?.querySelector<HTMLElement>(".vgrid")?.focus();
  }

  type Loaded =
    | { kind: "obd"; file: ObjectFile; thing: Thing; pixels: Uint8ClampedArray[] }
    | { kind: "image"; image: ImageFile & { url: string } };

  // Recently read files, so browsing back and forth is instant. Neighbours
  // of the shown file are read ahead (slow drives make every read count).
  const CACHE_SIZE = 32;
  const CELL_WIDTH = 64;
  const CELL_HEIGHT = 64;
  const cache = new LruCache<string, Promise<Loaded>>(CACHE_SIZE);

  function read(path: string): Promise<Loaded> {
    const hit = cache.get(path);
    if (hit) return hit;
    const reading = load(path);
    reading.catch(() => cache.delete(path));
    cache.set(path, reading);
    return reading;
  }

  function load(path: string): Promise<Loaded> {
    return OBJECT_EXT.test(path)
      ? ThingService.ReadObject(path).then((file) => ({ kind: "obd", ...decodeObjectFile(file, "empty object file") }))
      : ViewerService.ReadImage(path).then((imageFile) => {
          if (!imageFile) throw new Error("empty image");
          return { kind: "image", image: { ...imageFile, url: `data:image/png;base64,${imageFile.png}` } };
        });
  }

  // Thumbnails of the grid, read when their cells show up. The newest
  // requests go first, so scrolling fast does not wait for skipped cells.
  // "" marks a file that could not be read.
  const thumbs = new SvelteMap<string, string>();
  const wanted: string[] = [];
  let reading = 0;

  function wantThumb(_node: HTMLElement, path: string) {
    const want = (wantedPath: string) => {
      if (thumbs.has(wantedPath) || wanted.includes(wantedPath)) return;
      wanted.push(wantedPath);
      pump();
    };
    want(path);
    return { update: want };
  }

  function pump() {
    while (reading < 3 && wanted.length) {
      const path = wanted.pop()!;
      reading++;
      (cache.peek(path) ?? load(path))
        .then((loaded) => thumbs.set(path, loaded.kind === "image" ? loaded.image.url : thumbnail(loaded.thing, loaded.pixels, loaded.file.spriteSize)))
        .catch(() => thumbs.set(path, ""))
        .finally(() => {
          reading--;
          pump();
        });
    }
  }

  function prefetch() {
    for (const offset of [1, -1]) {
      const neighbour = entries[index + offset];
      if (neighbour) read(neighbour.path).catch(() => {});
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
      const loaded = await read(path);
      if (current !== path) return;
      if (loaded.kind === "image") {
        image = loaded.image;
        file = null;
        thing = null;
      } else {
        pixels = loaded.pixels;
        file = loaded.file;
        thing = loaded.thing;
        image = null;
        group = 0;
        dropCategory = loaded.thing.category;
        if (!thumbs.get(path)) thumbs.set(path, thumbnail(loaded.thing, loaded.pixels, loaded.file.spriteSize));
      }
    } catch (err) {
      if (current !== path) return;
      file = null;
      thing = null;
      image = null;
      error = errorMessage(err);
    } finally {
      if (current === path) {
        clearTimeout(loaderTimer);
        loading = false;
      }
    }
    scrollTo = null;
    await tick();
    scrollTo = index;
    prefetch();
  }

  function onkeydown(event: KeyboardEvent, columns: number) {
    const pageRows = Math.max(1, Math.floor((list?.clientHeight ?? 0) / CELL_HEIGHT));
    const next = nextIndex(event.key, index < 0 ? 0 : index, entries.length, { columns, pageRows });
    if (next === null || !entries.length) return;
    event.preventDefault();
    void show(entries[next].path);
  }

  // Queue of object files to drop into chosen ids of the client.
  let dropCategory = $state<Category>(app.category);
  const queueCounts = $derived(fileQueue.counts());
  const queuedPaths = $derived(new Set(app.fileQueue.map((item) => item.key)));

  /** First frame of a thing, cropped to its visible pixels, as a PNG data URL. */
  function thumbnail(object: Thing, spritePixels: Uint8ClampedArray[], size: number): string {
    const frameGroup = object.frameGroups[0];
    const outfit = object.category === Category.CategoryOutfit;
    const texture = compose(frameGroup, frameGroup.sprites, size, { pos: { layer: 0, x: outfit ? Math.min(2, frameGroup.patternX - 1) : 0, y: 0, z: 0, frame: 0 }, colors: outfit ? DEFAULT_COLORS : null }, (id) => (id > 0 ? spritePixels[id - 1] : undefined));
    const box = contentBox(texture) ?? { x: 0, y: 0, w: texture.width, h: texture.height };
    const full = document.createElement("canvas");
    full.width = texture.width;
    full.height = texture.height;
    full.getContext("2d")!.putImageData(texture, 0, 0);
    const cropped = document.createElement("canvas");
    cropped.width = box.w;
    cropped.height = box.h;
    cropped.getContext("2d")!.drawImage(full, box.x, box.y, box.w, box.h, 0, 0, box.w, box.h);
    return cropped.toDataURL();
  }

  /** Queues an object file, or takes it out of the queue. */
  async function toggleQueue(path: string) {
    const queued = app.fileQueue.find((item) => item.key === path);
    if (queued) {
      fileQueue.dequeue(queued.category, [path]);
      return;
    }
    let loaded: Loaded;
    try {
      loaded = await read(path);
    } catch (err) {
      toast(errorMessage(err), "error");
      return;
    }
    if (loaded.kind !== "obd") return;
    const thumb = thumbs.get(path) || thumbnail(loaded.thing, loaded.pixels, loaded.file.spriteSize);
    fileQueue.enqueue(loaded.thing.category, [{ key: path, label: basename(path), thumb }]);
    dropCategory = loaded.thing.category;
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

<Dialog title="Object Viewer" width={1000} onclose={() => (app.dialog = null)}>
  <div class="layout">
    <div class="files col">
      <button class="t-label folder" title={dir ? `${dir}
Click to open another file` : "Open a file"} onclick={browse}>{dir || "No folder"}</button>
      <div class="list t-panel" bind:this={list}>
        {#if entries.length}
          <VirtualGrid count={entries.length} cellWidth={CELL_WIDTH} cellHeight={CELL_HEIGHT} gap={2} {scrollTo} {onkeydown}>
            {#snippet cell(i)}
              {@const fileEntry = entries[i]}
              {@const src = thumbs.get(fileEntry.path)}
              <button
                class="entry"
                class:on={fileEntry.path === current}
                class:busy={loading && fileEntry.path === current}
                class:queued={queuedPaths.has(fileEntry.path)}
                title={fileEntry.kind === "obd" ? `${fileEntry.name}${queuedPaths.has(fileEntry.path) ? " (queued)" : ""}\nDouble click queues it` : `${fileEntry.name}\nImage, not an object`}
                onclick={() => show(fileEntry.path)}
                ondblclick={() => fileEntry.kind === "obd" && toggleQueue(fileEntry.path)}
                use:wantThumb={fileEntry.path}
              >
                <span class="thumb">
                  {#if src}
                    <img class="pixel" {src} alt="" draggable="false" />
                  {:else if src === ""}
                    <Icon name="close" />
                  {:else}
                    <span class="t-spinner"></span>
                  {/if}
                  {#if fileEntry.kind === "image"}<span class="badge img">IMG</span>{/if}
                  {#if queuedPaths.has(fileEntry.path)}<span class="badge q">Q</span>{/if}
                </span>
                <span class="fname">{fileEntry.name.replace(/\.[^.]+$/, "")}</span>
              </button>
            {/snippet}
          </VirtualGrid>
        {:else if listing}
          <div class="t-label none"><span class="t-spinner"></span>Reading folder…</div>
        {:else if dir}
          <button class="t-label none open" onclick={browse}>No object files or images here.<br />Click to open another file.</button>
        {:else}
          <button class="t-label none open" onclick={browse}><Icon name="open" /><br />Click to open a file and list its folder.</button>
        {/if}
      </div>
      <div class="t-label">{plural(counts.obd, "object")} · {plural(counts.image, "image")} · arrows to browse</div>
    </div>

    <div class="view col" class:dim={loading}>
      {#if loading}
        <div class="loader t-panel" role="status"><span class="t-spinner"></span>Loading {name}…</div>
      {/if}
      {#if source}
        <Preview {source} bind:group />
      {:else}
        <div class="stage checker">
          {#if image}
            <PanStage {zoom} onzoom={(direction) => (zoom = stepZoom(zoom, direction))} resetKey={current}>
              <img class="pixel" src={image.url} alt={name} />
            </PanStage>
          {:else if error}
            <span class="err">{error}</span>
          {:else if !current && !loading}
            <span class="t-label">Open an .otobj, .obd or image file to view it.</span>
          {/if}
        </div>
        {#if image}
          <div class="row tools">
            <span class="grow"></span>
            <button class="t-icon-btn" title="Zoom out (Ctrl+wheel)" disabled={zoom <= ZOOM_LEVELS[0]} onclick={() => (zoom = stepZoom(zoom, -1))}><Icon name="zoomout" /></button>
            <span class="t-label">{zoomLabel(zoom)}</span>
            <button class="t-icon-btn" title="Zoom in (Ctrl+wheel)" disabled={zoom >= ZOOM_LEVELS[ZOOM_LEVELS.length - 1]} onclick={() => (zoom = stepZoom(zoom, 1))}><Icon name="zoomin" /></button>
          </div>
        {/if}
      {/if}
    </div>

    <div class="side col">
      <strong class="name" title={current}>{name || "Nothing selected"}</strong>
      {#if image}
        <div class="notice">
          <Icon name="info" />
          <span>Plain {image.format.toUpperCase()} image, {image.width}×{image.height} px{entry ? `, ${formatKb(entry.size)}` : ""}. Not an object; it can only be imported as sprites.</span>
        </div>
      {:else if thing && file && frameGroup}
        <div class="info t-panel">
          <span class="t-label">Type</span><span>{CATEGORY_NAMES[thing.category]}</span>
          <span class="t-label">Format</span><span>{file.format === "otobj" ? "OTOBJ" : "OBD"} {file.version}</span>
          <span class="t-label">Client</span><span>{(file.clientVersion / 100).toFixed(2)}</span>
          <span class="t-label">Size</span><span>{frameGroup.width}×{frameGroup.height} ({file.spriteSize}px)</span>
          <span class="t-label">Layers</span><span>{frameGroup.layers}</span>
          <span class="t-label">Pattern</span><span>{frameGroup.patternX}×{frameGroup.patternY}×{frameGroup.patternZ}</span>
          <span class="t-label">Frames</span><span>{frameGroup.frames}</span>
          <span class="t-label">Sprites</span><span>{pixels.length}</span>
        </div>
      {/if}
      <div class="t-tabs cats">
        {#each CATEGORIES as category}
          {@const queuedCount = queueCounts.get(category) ?? 0}
          <button class="t-tab" class:on={dropCategory === category} title={CATEGORY_LABELS[category]} onclick={() => (dropCategory = category)}>{CATEGORY_LABELS[category]}{queuedCount ? ` (${queuedCount})` : ""}</button>
        {/each}
      </div>
      <div class="side-panel">
        {#if app.open}
          <DropPanel queue={fileQueue} category={dropCategory} onshow={(path: string) => open(path)} />
        {:else}
          <span class="t-label">Open a client to drop objects into it.</span>
        {/if}
      </div>
    </div>
  </div>
  {#snippet footer()}
    <button class="t-btn" onclick={browse}>Open…</button>
    <span class="grow"></span>
    {#if image}
      <button class="t-btn primary" disabled={!app.open || !!app.busy} title={app.open ? "" : "Open a client to import"} onclick={importImage}>Import as sprites</button>
    {:else}
      <button class="t-btn primary" disabled={!file} title="Queue the object, then click a slot to drop it there (double click a file queues it too)" onclick={() => file && toggleQueue(current)}
        >{queuedPaths.has(current) ? "Unqueue" : "Queue"}</button
      >
    {/if}
    <button class="t-btn" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .layout {
    display: flex;
    gap: 8px;
    height: 500px;
  }
  .files {
    width: 232px;
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
    display: flex;
    padding: 3px;
  }
  .list :global(.vgrid:focus-visible) {
    box-shadow: 0 0 0 1px #6a6a6a;
  }
  .entry {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
    padding: 2px;
    border: 1px solid transparent;
    background: none;
    font: inherit;
    color: var(--text-dim);
    cursor: pointer;
  }
  .entry :global(*) {
    pointer-events: none;
  }
  .entry:hover {
    background: rgba(255, 255, 255, 0.05);
  }
  .entry.queued {
    border-color: rgba(232, 196, 106, 0.5);
  }
  .entry.on {
    background: var(--select);
    border-color: #b6b6b6;
    color: #fff;
  }
  .thumb {
    position: relative;
    flex: 1;
    min-height: 0;
    width: 100%;
    display: grid;
    place-items: center;
  }
  .thumb img {
    max-width: 44px;
    max-height: 44px;
    object-fit: contain;
  }
  .badge {
    position: absolute;
    top: 0;
    padding: 0 2px;
    font: var(--fs-small) / 10px var(--font-small);
    border: 1px solid currentColor;
    background: rgba(0, 0, 0, 0.6);
  }
  .badge.img {
    left: 0;
    color: var(--gold);
  }
  .badge.q {
    right: 0;
    color: var(--green);
  }
  .fname {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: var(--fs-small) / 12px var(--font-small);
  }
  .none {
    margin: auto;
    padding: 8px;
  }
  .folder {
    border: none;
    background: none;
    font: inherit;
    padding: 0;
    cursor: pointer;
  }
  .folder:hover {
    color: var(--text-bright);
  }
  /* The empty list is one big open button. */
  .none.open {
    margin: 0;
    flex: 1;
    border: 1px dashed transparent;
    background: none;
    font: inherit;
    text-align: center;
    cursor: pointer;
  }
  .none.open:hover {
    color: var(--text-bright);
    border-color: #6a6a6a;
  }
  .view {
    position: relative;
    flex: 1;
    min-width: 0;
    min-height: 0;
    gap: 4px;
  }
  .stage {
    position: relative;
    flex: 1;
    min-height: 0;
    display: grid;
    place-items: center;
    overflow: hidden;
    border: 1px solid #1a1a1a;
  }
  /* Previous content stays visible but faded while the next file loads. */
  .view.dim > :global(:not(.loader)) {
    opacity: 0.35;
  }
  .tools {
    gap: 6px;
    min-height: 22px;
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
  .t-spinner {
    flex: none;
  }
  .none .t-spinner {
    margin-right: 6px;
    vertical-align: -2px;
  }
  .entry.busy .thumb {
    animation: pulse 0.8s ease-in-out infinite alternate;
  }
  @keyframes pulse {
    to {
      opacity: 0.3;
    }
  }
  .side {
    width: 250px;
    flex: none;
    gap: 4px;
    min-height: 0;
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
    grid-template-columns: auto 1fr auto 1fr;
    gap: 1px 6px;
    padding: 4px 6px;
    font: var(--fs-small) / 14px var(--font-small);
  }
  .info > span:nth-child(even) {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cats {
    margin-top: 4px;
  }
  .cats .t-tab {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding-inline: 2px;
  }
  .side-panel {
    position: relative;
    flex: 1;
    min-height: 0;
  }
  .side-panel > :global(.drop) {
    position: absolute;
    inset: 0;
  }
  .err {
    color: #ff9c9c;
    padding: 12px;
  }
</style>
