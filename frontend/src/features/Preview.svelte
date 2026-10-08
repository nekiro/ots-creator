<script lang="ts" module>
  // The chosen mount and view survive switching objects.
  let savedMountId = 0;
  let savedSheetView = false;
</script>

<script lang="ts">
  import { untrack } from "svelte";
  import { Category, errorMessage, normalizeThing, ThingService, type Thing } from "../lib/api";
  import { Animator } from "../lib/render/animator";
  import { compose, stackBottomRight } from "../lib/render/compose";
  import { clampPos, fillSlots, textureSlots, type TexturePos } from "../lib/render/layout";
  import { droppedSprites } from "../lib/dragsprites";
  import SheetEditor from "./SheetEditor.svelte";
  import { DEFAULT_COLORS, type OutfitColors } from "../lib/render/outfit";
  import { spriteCache } from "../lib/render/sprites";
  import { app, toast } from "../lib/state.svelte";
  import NumberField from "../lib/ui/NumberField.svelte";
  import Slider from "../lib/ui/Slider.svelte";
  import Icon from "../lib/ui/Icon.svelte";
  import ColorPicker from "./ColorPicker.svelte";

  let { group = $bindable(0) }: { group?: number } = $props();

  let canvas = $state<HTMLCanvasElement>();
  // Zoom follows the window (largest whole zoom that fits) until the user
  // zooms by hand; selecting another object returns to fitting.
  let manualZoom = $state<number | null>(null);
  let stage = $state<HTMLDivElement>();
  let stageSize = $state<[number, number]>([0, 0]);
  let showGrid = $state(true);
  let playing = $state(true);
  let colorize = $state(true);
  let addon1 = $state(false);
  let addon2 = $state(false);
  let mount = $state(false);
  let mountId = $state(savedMountId);
  let mountThing = $state<Thing | null>(null);
  let riderOffset = $state<[number, number]>([0, 0]);
  let canvasSize = $state<[number, number]>([0, 0]);
  let colors = $state<OutfitColors>({ ...DEFAULT_COLORS });
  let pos = $state<TexturePos>({ layer: 0, x: 0, y: 0, z: 0, frame: 0 });
  let loaded = $state(0); // bumps when sprites arrive
  let hoverSlot = $state<number | null>(null);
  let dropSlot = $state<number | null>(null);

  const thing = $derived(app.draft);
  const g = $derived(thing?.frameGroups[Math.min(group, (thing?.frameGroups.length ?? 1) - 1)] ?? null);
  const isOutfit = $derived(thing?.category === Category.CategoryOutfit);
  const size = $derived(loaded >= 0 ? spriteCache.size : 32);

  const MAX_ZOOM = 16;
  const fitZoom = $derived.by(() => {
    if (!g || !stageSize[0]) return 4;
    const w = canvasSize[0] || g.width * size;
    const h = canvasSize[1] || g.height * size;
    const margin = 34; // canvas-wrap margin and borders
    const r = Math.min((stageSize[0] - margin) / w, (stageSize[1] - margin) / h);
    return Math.min(12, Math.max(1, Math.floor(r)));
  });
  const zoom = $derived(manualZoom ?? fitZoom);

  $effect(() => {
    if (!stage) return;
    const ro = new ResizeObserver(() => (stageSize = [stage!.clientWidth, stage!.clientHeight]));
    ro.observe(stage);
    return () => ro.disconnect();
  });

  function setZoom(z: number) {
    manualZoom = Math.min(MAX_ZOOM, Math.max(1, z));
  }

  // Reset view state when another thing is selected.
  let lastKey = "";
  $effect(() => {
    const key = `${app.category}:${thing?.id}`;
    if (key === lastKey) return;
    lastKey = key;
    group = 0;
    manualZoom = null;
    pos = { layer: 0, x: isOutfit && (thing?.frameGroups[0]?.patternX ?? 1) > 2 ? 2 : 0, y: 0, z: 0, frame: 0 };
    hoverSlot = null;
  });

  // Keep the position valid when the layout changes.
  $effect(() => {
    if (!g) return;
    const c = clampPos(g, pos);
    if (JSON.stringify(c) !== JSON.stringify(pos)) pos = c;
  });

  // Any outfit can be a mount: the client has no mount flag.
  $effect(() => {
    savedMountId = mountId;
    const id = mountId;
    void app.rev;
    if (!isOutfit || !mount || (g?.patternZ ?? 1) < 2 || id < 1) {
      mountThing = null;
      return;
    }
    let stale = false;
    ThingService.Get(Category.CategoryOutfit, id).then(
      (t) => !stale && (mountThing = t ? normalizeThing(t) : null),
      (e) => {
        if (stale) return;
        mountThing = null;
        toast(errorMessage(e), "error");
      },
    );
    return () => (stale = true);
  });
  const mg = $derived(mountThing?.frameGroups[Math.min(group, mountThing.frameGroups.length - 1)] ?? null);

  $effect(() => {
    if (mg) spriteCache.load(mg.sprites).then(() => loaded++);
  });

  // Load all sprites of the current group.
  $effect(() => {
    if (!g) return;
    const ids = g.sprites;
    void app.rev;
    spriteCache.load(ids).then(() => loaded++);
  });

  // Animation.
  let animator: Animator | null = null;
  $effect(() => {
    if (!g || g.frames < 2 || !playing) {
      animator = null;
      return;
    }
    const durations = g.durations?.length === g.frames ? g.durations : Array.from({ length: g.frames }, () => ({ min: 100, max: 100 }));
    try {
      animator = new Animator(Number(g.mode), 0, Math.min(Math.max(g.startFrame, 0), g.frames - 1), durations, performance.now());
      animator.setFrame(untrack(() => pos.frame), performance.now());
    } catch {
      animator = null;
      return;
    }
    let raf = 0;
    const tick = (t: number) => {
      if (!animator) return;
      const f = animator.update(t);
      if (f !== pos.frame) pos.frame = f;
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  });

  // Draw.
  $effect(() => {
    void loaded;
    if (!g || !canvas) return;
    const addons = isOutfit ? [addon1 ? 1 : -1, addon2 ? 2 : -1].filter((y) => y > 0) : [];
    const p = { ...pos, z: isOutfit && mount && g.patternZ > 1 ? 1 : pos.z };
    const get = (id: number) => spriteCache.get(id)?.pixels;
    const outfitColors = isOutfit && colorize ? colors : null;
    const rider = compose(g, g.sprites, size, { pos: p, addons, colors: outfitColors }, get);
    let img = rider;
    riderOffset = [0, 0];
    if (isOutfit && mount && g.patternZ > 1 && mg) {
      const mp = { layer: 0, x: Math.min(pos.x, mg.patternX - 1), y: 0, z: 0, frame: pos.frame % mg.frames };
      const stacked = stackBottomRight([compose(mg, mg.sprites, size, { pos: mp, colors: outfitColors }, get), rider]);
      img = stacked.image;
      riderOffset = stacked.offsets[1];
    }
    canvasSize = [img.width, img.height];
    const c = canvas;
    c.width = img.width;
    c.height = img.height;
    c.getContext("2d")!.putImageData(img, 0, 0);
  });

  const slots = $derived(g ? textureSlots(g, { ...pos, z: isOutfit && mount && g.patternZ > 1 ? 1 : pos.z }, size) : []);

  function setDir(d: number) {
    if (g && d < g.patternX) pos.x = d;
  }

  function step(delta: number) {
    if (!g) return;
    playing = false;
    pos.frame = (pos.frame + delta + g.frames) % g.frames;
  }

  /** Puts sprites into slot and the following ones (multi-drag). */
  function assign(slot: number, ids: number[]) {
    if (!g || !app.draft) return;
    const grp = app.draft.frameGroups[Math.min(group, app.draft.frameGroups.length - 1)];
    if (grp) grp.sprites = fillSlots(grp.sprites, slot, ids);
  }

  function onDrop(e: DragEvent, slot: number) {
    e.preventDefault();
    dropSlot = null;
    const ids = droppedSprites(e);
    if (ids.length) assign(slot, ids);
  }

  // Sheet view: every texture of the group at once, kept while browsing.
  // It has its own zoom (fit to window until changed by hand).
  let sheetView = $state(savedSheetView);
  let sheetManualZoom = $state<number | null>(null);
  let sheetFitZoom = $state(1);
  const shownZoom = $derived(sheetView ? (sheetManualZoom ?? sheetFitZoom) : zoom);
  const shownAuto = $derived(sheetView ? sheetManualZoom === null : manualZoom === null);

  function zoomBy(delta: number) {
    if (sheetView) sheetManualZoom = Math.min(MAX_ZOOM, Math.max(1, shownZoom + delta));
    else setZoom(zoom + delta);
  }

  function zoomFit() {
    if (sheetView) sheetManualZoom = null;
    else manualZoom = null;
  }

  if (savedSheetView) playing = false;
  $effect(() => {
    savedSheetView = sheetView;
  });

  function openTexture(p: TexturePos) {
    pos = { ...p };
    playing = false;
    sheetView = false;
  }

  function onSlotKey(e: KeyboardEvent, slot: number) {
    if (e.key === "Delete" || e.key === "Backspace") {
      e.preventDefault();
      assign(slot, [0]);
    }
  }

  function onWheel(e: WheelEvent) {
    if (!e.ctrlKey) return;
    e.preventDefault();
    setZoom(zoom + (e.deltaY < 0 ? 1 : -1));
  }
</script>

<div class="preview">
  {#if g && sheetView}
  <div class="stage-wrap">
    <SheetEditor
      {g}
      {size}
      {loaded}
      selected={app.selectedSprites}
      {showGrid}
      current={pos}
      onassign={assign}
      onpick={(slot) => {
        const id = g.sprites[slot];
        if (id) app.selectedSprites = [id];
      }}
      onopen={openTexture}
      manualZoom={sheetManualZoom}
      bind:fitZoom={sheetFitZoom}
      onwheelzoom={zoomBy}
    />
  </div>
  {:else}
  <div class="stage-wrap">
  <div class="stage checker" onwheel={onWheel} bind:this={stage}>
    {#if g}
      <div class="canvas-wrap" style="width:{(canvasSize[0] || g.width * size) * zoom}px;height:{(canvasSize[1] || g.height * size) * zoom}px">
        <canvas bind:this={canvas} class="pixel" style="width:100%;height:100%"></canvas>
        <div class="slots" class:grid={showGrid}>
          {#each slots as s (s.slot)}
            {@const id = g.sprites[s.slot]}
            <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
            <div
              class="slot"
              class:drop={dropSlot === s.slot}
              class:sel={id !== 0 && app.selectedSprites.includes(id)}
              style="left:{(s.x + riderOffset[0]) * zoom}px;top:{(s.y + riderOffset[1]) * zoom}px;width:{size * zoom}px;height:{size * zoom}px"
              title="Slot {s.slot} → sprite #{id}{id ? '' : ' (empty)'}. Drop a sprite here, Del clears."
              tabindex="0"
              role="button"
              onclick={() => id && (app.selectedSprites = [id])}
              onkeydown={(e) => onSlotKey(e, s.slot)}
              onmouseenter={() => (hoverSlot = s.slot)}
              onmouseleave={() => (hoverSlot = null)}
              ondragover={(e) => {
                e.preventDefault();
                dropSlot = s.slot;
              }}
              ondragleave={() => (dropSlot = null)}
              ondrop={(e) => onDrop(e, s.slot)}
            ></div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="empty">Select an object</div>
    {/if}
  </div>
  {#if g && hoverSlot !== null}
    <div class="slot-info t-small">slot {hoverSlot} · sprite #{g.sprites[hoverSlot]}</div>
  {/if}
  </div>
  {/if}

  {#if g && thing}
    <div class="controls">
      <div class="row playback">
        {#if thing.frameGroups.length > 1}
          <div class="t-tabs">
            {#each thing.frameGroups as _, i}
              <button class="t-tab" class:on={group === i} onclick={() => (group = i)}>{i === 0 ? "Idle" : "Walking"}</button>
            {/each}
          </div>
          <span class="t-vsep"></span>
        {/if}
        <button class="t-icon-btn" title="Previous frame" disabled={g.frames < 2} onclick={() => step(-1)}><Icon name="prev" /></button>
        <button class="t-icon-btn" class:on={playing} title={playing ? "Pause" : "Play"} disabled={g.frames < 2} onclick={() => (playing = !playing)}>
          <Icon name={playing ? "pause" : "play"} />
        </button>
        <button class="t-icon-btn" title="Next frame" disabled={g.frames < 2} onclick={() => step(1)}><Icon name="next" /></button>
        <span class="t-value frame">{pos.frame + 1}/{g.frames}</span>
        <span class="t-vsep"></span>
        <button class="t-icon-btn" title="Zoom out" onclick={() => zoomBy(-1)}><Icon name="zoomout" /></button>
        <button class="t-value zoom" class:auto={shownAuto} title={shownAuto ? "Fits the window" : "Click to fit the window"} onclick={zoomFit}>
          {shownZoom}x
        </button>
        <button class="t-icon-btn" title="Zoom in (Ctrl+wheel)" onclick={() => zoomBy(1)}><Icon name="zoomin" /></button>
        <button class="t-icon-btn" class:on={showGrid} title="Tile grid" onclick={() => (showGrid = !showGrid)}><Icon name="grid" /></button>
        <button
          class="t-icon-btn"
          class:on={sheetView}
          title={sheetView ? "Back to one texture" : "All textures: drop sprites on any frame or pattern (double click opens one)"}
          onclick={() => {
            sheetView = !sheetView;
            if (sheetView) playing = false;
          }}><Icon name="film" /></button
        >
      </div>

      <div class="row">
        {#if isOutfit && g.patternX === 4}
          <span class="t-label">Direction</span>
          {#each ["arrowN", "arrowE", "arrowS", "arrowW"] as icon, d}
            <button class="t-icon-btn" class:on={pos.x === d} title={["North", "East", "South", "West"][d]} onclick={() => setDir(d)}><Icon name={icon} /></button>
          {/each}
        {:else if g.patternX > 1}
          <div class="row"><span class="t-label">Pattern X</span><Slider max={g.patternX - 1} bind:value={pos.x} /><span class="t-value num">{pos.x}</span></div>
        {/if}
        {#if isOutfit}
          {#if g.patternY > 1}
            <label class="row"><input class="t-check" type="checkbox" bind:checked={addon1} />Addon 1</label>
          {/if}
          {#if g.patternY > 2}
            <label class="row"><input class="t-check" type="checkbox" bind:checked={addon2} />Addon 2</label>
          {/if}
          {#if g.patternZ > 1}
            <label class="row" title="Show the outfit riding another looktype"><input class="t-check" type="checkbox" bind:checked={mount} />Mount</label>
            {#if mount}
              <NumberField value={mountId} min={0} max={app.maxId(Category.CategoryOutfit)} width={50} title="Mount looktype (0 = none)" onchange={(v) => (mountId = v)} />
            {/if}
          {/if}
        {:else}
          {#if g.patternY > 1}
            <div class="row"><span class="t-label">Pattern Y</span><Slider max={g.patternY - 1} bind:value={pos.y} /><span class="t-value num">{pos.y}</span></div>
          {/if}
          {#if g.patternZ > 1}
            <div class="row"><span class="t-label">Pattern Z</span><Slider max={g.patternZ - 1} bind:value={pos.z} /><span class="t-value num">{pos.z}</span></div>
          {/if}
        {/if}
        {#if g.layers > 1}
          <div class="row" title="Show the layer instead of the composed outfit">
            <span class="t-label">Layer</span>
            <Slider max={g.layers - 1} bind:value={pos.layer} />
            <span class="t-value num">{pos.layer}</span>
          </div>
        {/if}
      </div>

      {#if isOutfit && g.layers > 1}
        <div class="row">
          <label class="row"><input class="t-check" type="checkbox" bind:checked={colorize} />Colors</label>
          <ColorPicker label="Head" value={colors.head} onchange={(v) => (colors.head = v)} />
          <ColorPicker label="Body" value={colors.body} onchange={(v) => (colors.body = v)} />
          <ColorPicker label="Legs" value={colors.legs} onchange={(v) => (colors.legs = v)} />
          <ColorPicker label="Feet" value={colors.feet} onchange={(v) => (colors.feet = v)} />
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .preview {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .stage-wrap {
    flex: 1;
    /* The stage gives up space first; controls keep their size. */
    min-height: 80px;
    position: relative;
    display: flex;
  }
  /* Overlay so hovering slots never shifts the layout. */
  .slot-info {
    position: absolute;
    left: 4px;
    bottom: 4px;
    padding: 2px 5px;
    color: var(--text-bright);
    background: rgba(0, 0, 0, 0.7);
    pointer-events: none;
  }
  .stage {
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    display: grid;
    place-items: center;
    border: 1px solid #111;
    box-shadow: inset 0 0 0 1px #4a4a4a;
  }
  .canvas-wrap {
    position: relative;
    margin: 16px;
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.5);
  }
  .slots {
    position: absolute;
    inset: 0;
  }
  .slot {
    position: absolute;
    outline: none;
    cursor: pointer;
  }
  .slots.grid .slot {
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.18);
  }
  .slot:hover,
  .slot:focus-visible {
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.7) !important;
  }
  .slot.sel {
    box-shadow: inset 0 0 0 2px var(--blue) !important;
  }
  .slot.drop {
    background: rgba(106, 166, 232, 0.35);
    box-shadow: inset 0 0 0 2px var(--blue) !important;
  }
  .empty {
    color: var(--text-dim);
  }
  .controls {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: none;
  }
  .controls > .row {
    flex-wrap: wrap;
    min-height: 23px;
  }
  .controls .t-tabs {
    width: 160px;
    flex: 0 1 160px;
    min-width: 96px;
  }
  /* Playback and zoom stay on one line; the group tabs shrink instead. */
  .controls > .playback {
    flex-wrap: nowrap;
    min-width: 0;
    overflow: hidden;
  }
  /* Fixed widths so a changing frame count or zoom never moves the buttons. */
  .frame {
    flex: none;
    width: 58px; /* fits "255/255" */
    text-align: center;
    font-variant-numeric: tabular-nums;
  }
  .zoom {
    flex: none;
    width: 34px;
    text-align: center;
    background: none;
    border: 0;
    padding: 0;
    font: inherit;
    cursor: pointer;
  }
  /* Fitting the window: gold, like other automatic values. */
  .zoom.auto {
    color: var(--gold);
    cursor: default;
  }
  .num {
    min-width: 16px;
  }
</style>
