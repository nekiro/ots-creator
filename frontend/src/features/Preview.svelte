<script lang="ts" module>
  let savedMountId = 0;
  let savedSheetView = false;

  export interface PreviewSource {
    /** Changes when another object is shown, which resets the view. */
    key: string;
    thing: Thing;
    get: PixelSource;
    size: number;
    /** The source stores frame durations; guessed from the thing when not given. */
    improved?: boolean;
  }
</script>

<script lang="ts">
  import { fitZoom } from "../lib/render/zoom";
  import { clamp } from "../lib/util/math";
  import { defaultDirection, frameGroupAt, hasImprovedAnimations } from "../lib/thing";
  import { untrack } from "svelte";
  import { Category, encodeBytes, errorMessage, Format, normalizeThing, ThingService, type Thing } from "../lib/api";
  import { Animator, guessImproved, previewDurations } from "../lib/render/animator";
  import { PREVIEW_DROP } from "../lib/commands";
  import { compose, type PixelSource } from "../lib/render/compose";
  import { mountedPattern, renderPreviewImage } from "../lib/render/previewImage";
  import { clampPos, fillSlots, textureSlots, type TexturePos } from "../lib/render/layout";
  import { droppedSprites } from "../lib/dragsprites";
  import SheetEditor from "./SheetEditor.svelte";
  import { DEFAULT_COLORS, type OutfitColors } from "../lib/render/outfit";
  import { spriteCache } from "../lib/render/sprites";
  import { app, toast } from "../lib/state/app.svelte";
  import { PanView } from "../lib/pan.svelte";
  import { changedSlots, drawLine, getPixel, hexToRGBA, rgbaToHex, type RGBA } from "../lib/render/paint";
  import NumberField from "../lib/ui/NumberField.svelte";
  import Slider from "../lib/ui/Slider.svelte";
  import Icon from "../lib/ui/Icon.svelte";
  import ColorPicker from "./ColorPicker.svelte";

  let { group = $bindable(0), source = null }: { group?: number; source?: PreviewSource | null } = $props();
  const readonly = $derived(!!source);

  let canvas = $state<HTMLCanvasElement>();
  // Zoom follows the window (largest whole zoom that fits, up to
  // MAX_FIT_ZOOM) until the user zooms by hand; a hand zoom stays when
  // another object is selected, until Fit.
  let manualZoom = $state<number | null>(null);
  let stage = $state<HTMLDivElement>();
  let stageSize = $state<[number, number]>([0, 0]);
  let showGrid = $state(true);
  let showPixelGrid = $state(false);
  const PIXEL_GRID_ZOOM = 4;
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
  let loaded = $state(0);
  let hoverSlot = $state<number | null>(null);
  let dropSlot = $state<number | null>(null);

  const thing = $derived(source ? source.thing : app.draft);
  const frameGroup = $derived(frameGroupAt(thing, group));
  const isOutfit = $derived(thing?.category === Category.CategoryOutfit);
  const size = $derived(source ? source.size : loaded >= 0 ? spriteCache.size : 32);

  const MAX_ZOOM = 16;
  // Fitting a small object to a big window would blow it up past what is
  // useful to look at.
  const MAX_FIT_ZOOM = 4;
  const STAGE_MARGIN = 34;
  const fittedZoom = $derived.by(() => {
    if (!frameGroup || !stageSize[0]) return 4;
    const contentWidth = canvasSize[0] || frameGroup.width * size;
    const contentHeight = canvasSize[1] || frameGroup.height * size;
    return fitZoom(contentWidth, contentHeight, stageSize[0], stageSize[1], MAX_FIT_ZOOM, STAGE_MARGIN, 1);
  });
  const zoom = $derived(manualZoom ?? fittedZoom);

  $effect(() => {
    if (!stage) return;
    const resizeObserver = new ResizeObserver(() => (stageSize = [stage!.clientWidth, stage!.clientHeight]));
    resizeObserver.observe(stage);
    return () => resizeObserver.disconnect();
  });

  function setZoom(requested: number) {
    const next = clamp(requested, 1, MAX_ZOOM);
    const factor = next / zoom;
    manualZoom = next;
    view.rescale(factor);
  }

  // The stage is a canvas, not a scrolled box: the texture is centered and
  // moved by the pan offset (space + drag, middle button drag, wheel).
  const view = new PanView(
    () => ({
      width: (canvasSize[0] || (frameGroup?.width ?? 1) * size) * zoom,
      height: (canvasSize[1] || (frameGroup?.height ?? 1) * size) * zoom,
      stageWidth: stageSize[0],
      stageHeight: stageSize[1],
    }),
    (delta) => setZoom(zoom + delta),
  );

  // Pixel editing of the shown texture (raw layer, no colors or addons).
  // Strokes paint on a copy and are stored when the mouse is released.
  type Tool = "pencil" | "eraser" | "picker";
  let editing = $state(false);
  let tool = $state<Tool>("pencil");
  let paintHex = $state("#ffffff");
  let shiftAll = $state(true);
  let paintImage = $state.raw<ImageData | null>(null);
  let paintTick = $state(0);
  let committed = false;
  let stroke: { id: number; before: ImageData; last: [number, number] } | null = null;
  const groupIndex = () => Math.min(group, (thing?.frameGroups.length ?? 1) - 1);
  const getSprite: PixelSource = (id) => (source ? source.get(id) : spriteCache.get(id)?.pixels);

  function toggleEditing() {
    editing = !editing;
    if (editing) {
      playing = false;
      sheetView = false;
    }
    paintImage = null;
  }

  /** Edits go to the client directly; unapplied changes would overwrite them. */
  function canEdit(): boolean {
    if (!app.dirty) return true;
    toast("Apply or revert the object changes first.");
    return false;
  }

  function pixelAt(event: PointerEvent): [number, number] {
    const bounds = (event.currentTarget as HTMLElement).getBoundingClientRect();
    return [Math.floor((event.clientX - bounds.left) / zoom), Math.floor((event.clientY - bounds.top) / zoom)];
  }

  function paintDown(event: PointerEvent) {
    if (!editing || event.button !== 0 || view.ready || !frameGroup || stroke) return;
    event.preventDefault();
    event.stopPropagation();
    const base = compose(frameGroup, frameGroup.sprites, size, { pos }, getSprite);
    const [x, y] = pixelAt(event);
    if (tool === "picker") {
      const color = getPixel(base, x, y);
      if (color[3]) paintHex = rgbaToHex(color);
      tool = "pencil";
      return;
    }
    if (!canEdit()) return;
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
    paintImage = new ImageData(new Uint8ClampedArray(base.data), base.width, base.height);
    stroke = { id: event.pointerId, before: base, last: [x, y] };
    drawLine(paintImage, x, y, x, y, paintColor());
    paintTick++;
  }

  function paintMove(event: PointerEvent) {
    if (stroke?.id !== event.pointerId || !paintImage) return;
    const [x, y] = pixelAt(event);
    drawLine(paintImage, stroke.last[0], stroke.last[1], x, y, paintColor());
    stroke.last = [x, y];
    paintTick++;
  }

  function paintColor(): RGBA {
    return tool === "eraser" ? [0, 0, 0, 0] : hexToRGBA(paintHex);
  }

  async function paintUp(event: PointerEvent) {
    const ended = stroke;
    if (ended?.id !== event.pointerId) return;
    stroke = null;
    if (!paintImage || !frameGroup || !thing) return;
    const { slots, pixels } = changedSlots(ended.before, paintImage, frameGroup, pos, size);
    if (!slots.length) {
      paintImage = null;
      return;
    }
    committed = true;
    try {
      await ThingService.SetPixels(thing.category, thing.id, groupIndex(), slots, pixels.map(encodeBytes));
    } catch (err) {
      toast(errorMessage(err), "error");
      paintImage = null;
      committed = false;
    }
  }

  async function shift(dx: number, dy: number) {
    if (!thing || !canEdit()) return;
    try {
      await ThingService.ShiftPixels(thing.category, thing.id, groupIndex(), dx, dy, !shiftAll, pos.frame, pos.x, pos.y, pos.z);
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  let lastKey = "";
  $effect(() => {
    const key = source ? source.key : `${app.category}:${thing?.id}`;
    if (key === lastKey) return;
    lastKey = key;
    if (group >= (thing?.frameGroups.length ?? 1)) group = 0;
    view.reset();
    paintImage = null;
    stroke = null;
    pos = { layer: 0, x: defaultDirection(thing), y: 0, z: 0, frame: 0 };
    hoverSlot = null;
  });

  $effect(() => {
    if (!frameGroup) return;
    const clamped = clampPos(frameGroup, pos);
    if (JSON.stringify(clamped) !== JSON.stringify(pos)) pos = clamped;
  });

  // Any outfit can be a mount: the client has no mount flag.
  $effect(() => {
    savedMountId = mountId;
    const id = mountId;
    void app.rev;
    if (readonly || !isOutfit || !mount || (frameGroup?.patternZ ?? 1) < 2 || id < 1) {
      mountThing = null;
      return;
    }
    let stale = false;
    ThingService.Get(Category.CategoryOutfit, id).then(
      (loadedMount) => !stale && (mountThing = loadedMount ? normalizeThing(loadedMount) : null),
      (err) => {
        if (stale) return;
        mountThing = null;
        toast(errorMessage(err), "error");
      },
    );
    return () => (stale = true);
  });
  const mountGroup = $derived(frameGroupAt(mountThing, group));

  $effect(() => {
    if (mountGroup) spriteCache.load(mountGroup.sprites).then(() => loaded++);
  });

  $effect(() => {
    if (!frameGroup || source) return;
    const ids = frameGroup.sprites;
    void app.rev;
    spriteCache.load(ids).then(() => {
      loaded++;
      // A committed stroke stays on screen until its sprites have arrived.
      if (!stroke && paintImage && untrack(() => committed)) {
        paintImage = null;
        committed = false;
      }
    });
  });

  let animator: Animator | null = null;
  $effect(() => {
    if (!frameGroup || frameGroup.frames < 2 || !playing || editing) {
      animator = null;
      return;
    }
    const info = app.project?.info;
    const durations = previewDurations(frameGroup, {
      outfit: isOutfit,
      groups: thing?.frameGroups.length ?? 1,
      animateAlways: !!thing?.props.animateAlways,
      improved: source ? (source.improved ?? guessImproved(source.thing)) : hasImprovedAnimations(info),
    });
    try {
      animator = new Animator(Number(frameGroup.mode), 0, clamp(frameGroup.startFrame, 0, frameGroup.frames - 1), durations, performance.now());
      animator.setFrame(untrack(() => pos.frame), performance.now());
    } catch {
      animator = null;
      return;
    }
    let raf = 0;
    const tick = (time: number) => {
      if (!animator) return;
      const frame = animator.update(time);
      if (frame !== pos.frame) pos.frame = frame;
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  });

  $effect(() => {
    void loaded;
    void paintTick;
    if (!frameGroup || !canvas) return;
    let image: ImageData;
    if (editing) {
      image = paintImage ?? compose(frameGroup, frameGroup.sprites, size, { pos }, getSprite);
      riderOffset = [0, 0];
    } else {
      const outfitColors = isOutfit && colorize ? colors : null;
      const preview = renderPreviewImage({ frameGroup, mountGroup, size, pos, isOutfit, mount, addon1, addon2, colors: outfitColors, get: getSprite });
      image = preview.image;
      riderOffset = preview.riderOffset;
    }
    canvasSize = [image.width, image.height];
    canvas.width = image.width;
    canvas.height = image.height;
    canvas.getContext("2d")!.putImageData(image, 0, 0);
  });

  const slots = $derived(frameGroup ? textureSlots(frameGroup, { ...pos, z: mountedPattern(frameGroup, isOutfit, mount) ? 1 : pos.z }, size) : []);

  function setDir(direction: number) {
    if (frameGroup && direction < frameGroup.patternX) pos.x = direction;
  }

  function step(delta: number) {
    if (!frameGroup) return;
    playing = false;
    pos.frame = (pos.frame + delta + frameGroup.frames) % frameGroup.frames;
  }

  function assign(slot: number, ids: number[]) {
    if (readonly || !frameGroup || !app.draft) return;
    const draftGroup = frameGroupAt(app.draft, group)!;
    if (draftGroup) draftGroup.sprites = fillSlots(draftGroup.sprites, slot, ids);
  }

  function onDrop(event: DragEvent, slot: number) {
    event.preventDefault();
    dropSlot = null;
    const ids = droppedSprites(event);
    if (ids.length) assign(slot, ids);
  }

  // Sheet view: every texture of the group at once, kept while browsing.
  // It has its own zoom (fit to window until changed by hand).
  const startSheet = untrack(() => !source && savedSheetView);
  let sheetView = $state(startSheet);
  let sheetManualZoom = $state<number | null>(null);
  let sheetFitZoom = $state(1);
  const shownZoom = $derived(sheetView ? (sheetManualZoom ?? sheetFitZoom) : zoom);
  const shownAuto = $derived(sheetView ? sheetManualZoom === null : manualZoom === null);

  function zoomBy(delta: number) {
    if (sheetView) sheetManualZoom = clamp(shownZoom + delta, 1, MAX_ZOOM);
    else setZoom(zoom + delta);
  }

  function zoomFit() {
    if (sheetView) sheetManualZoom = null;
    else {
      manualZoom = null;
      view.reset();
    }
  }

  if (startSheet) playing = false;
  $effect(() => {
    if (!source) savedSheetView = sheetView;
  });

  function openTexture(texturePos: TexturePos) {
    pos = { ...texturePos };
    playing = false;
    sheetView = false;
  }

  function onSlotKey(event: KeyboardEvent, slot: number) {
    if (event.key === "Delete" || event.key === "Backspace") {
      event.preventDefault();
      assign(slot, [0]);
    }
  }

</script>

<!-- Images dropped here are sprite sheets for the object (commands.drop). -->
<svelte:window onkeydown={view.key} onkeyup={view.key} onblur={view.blur} />

<div class="preview" {...readonly ? {} : { id: PREVIEW_DROP, "data-file-drop-target": true }}>
  {#if frameGroup && sheetView}
  <div class="stage-wrap">
    <SheetEditor
      frameGroup={frameGroup}
      {size}
      {loaded}
      selected={app.selectedSprites}
      {showGrid}
      pixelGrid={showPixelGrid}
      colors={isOutfit && colorize ? colors : null}
      resetKey={`${source ? source.key : `${app.category}:${thing?.id}`}:${group}`}
      current={pos}
      onassign={assign}
      get={getSprite}
      {readonly}
      onpick={(slot) => {
        const id = frameGroup.sprites[slot];
        if (readonly) return;
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
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="stage checker"
    class:pan-ready={view.ready}
    class:panning={view.dragging}
    bind:this={stage}
    {...view.handlers}
  >
    {#if frameGroup}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="canvas-wrap"
        class:editing
        onpointerdown={paintDown}
        onpointermove={paintMove}
        onpointerup={paintUp}
        onpointercancel={paintUp}
        style="width:{(canvasSize[0] || frameGroup.width * size) * zoom}px;height:{(canvasSize[1] || frameGroup.height * size) * zoom}px;transform:{view.transform}"
      >
        <canvas bind:this={canvas} class="pixel" style="width:100%;height:100%"></canvas>
        {#if showPixelGrid && zoom >= PIXEL_GRID_ZOOM}
          <div class="pixel-grid" style="background-size:{zoom}px {zoom}px"></div>
        {/if}
        <div class="slots" class:grid={showGrid}>
          {#each slots as slotInfo (slotInfo.slot)}
            {@const id = frameGroup.sprites[slotInfo.slot]}
            <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
            <div
              class="slot"
              class:drop={dropSlot === slotInfo.slot}
              class:sel={id !== 0 && app.selectedSprites.includes(id)}
              style="left:{(slotInfo.x + riderOffset[0]) * zoom}px;top:{(slotInfo.y + riderOffset[1]) * zoom}px;width:{size * zoom}px;height:{size * zoom}px"
              title="Slot {slotInfo.slot} → sprite #{id}{id ? '' : ' (empty)'}{readonly ? '' : '. Drop a sprite here, Del clears.'}"
              tabindex="0"
              role="button"
              onclick={() => id && !readonly && (app.selectedSprites = [id])}
              onkeydown={(event) => onSlotKey(event, slotInfo.slot)}
              onmouseenter={() => (hoverSlot = slotInfo.slot)}
              onmouseleave={() => (hoverSlot = null)}
              ondragover={(event) => {
                if (readonly) return;
                event.preventDefault();
                dropSlot = slotInfo.slot;
              }}
              ondragleave={() => (dropSlot = null)}
              ondrop={(event) => onDrop(event, slotInfo.slot)}
            ></div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="empty">Select an object</div>
    {/if}
  </div>
  {#if frameGroup && hoverSlot !== null}
    <div class="slot-info t-small">slot {hoverSlot} · sprite #{frameGroup.sprites[hoverSlot]}</div>
  {/if}
  </div>
  {/if}

  {#if frameGroup && thing}
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
        <button class="t-icon-btn" title="Previous frame" disabled={frameGroup.frames < 2} onclick={() => step(-1)}><Icon name="prev" /></button>
        <button class="t-icon-btn" class:on={playing} title={playing ? "Pause" : "Play"} disabled={frameGroup.frames < 2} onclick={() => (playing = !playing)}>
          <Icon name={playing ? "pause" : "play"} />
        </button>
        <button class="t-icon-btn" title="Next frame" disabled={frameGroup.frames < 2} onclick={() => step(1)}><Icon name="next" /></button>
        <span class="t-value frame">{pos.frame + 1}/{frameGroup.frames}</span>
        <span class="t-vsep"></span>
        <button class="t-icon-btn" title="Zoom out" onclick={() => zoomBy(-1)}><Icon name="zoomout" /></button>
        <button class="t-value zoom" class:auto={shownAuto} title={shownAuto ? "Fits the window" : "Click to fit the window"} onclick={zoomFit}>
          {shownZoom}x
        </button>
        <button class="t-icon-btn" title="Zoom in (Ctrl+wheel)" onclick={() => zoomBy(1)}><Icon name="zoomin" /></button>
        <button class="t-icon-btn" class:on={showGrid} title="Tile grid" onclick={() => (showGrid = !showGrid)}><Icon name="grid" /></button>
        {#if !readonly}
          <button class="t-icon-btn" class:on={editing} title="Edit pixels" onclick={toggleEditing}><Icon name="pencil" /></button>
        {/if}
        <button
          class="t-icon-btn"
          class:on={showPixelGrid}
          title={shownZoom >= PIXEL_GRID_ZOOM ? "Pixel grid" : `Pixel grid (shown from ${PIXEL_GRID_ZOOM}x zoom)`}
          onclick={() => (showPixelGrid = !showPixelGrid)}><Icon name="pixels" /></button
        >
        <button
          class="t-icon-btn"
          class:on={sheetView}
          title={sheetView ? "Back to one texture" : "All textures: drop sprites on any frame or pattern (double click opens one)"}
          onclick={() => {
            sheetView = !sheetView;
            if (startSheet) playing = false;
          }}><Icon name="film" /></button
        >
      </div>

      <div class="row">
        {#if isOutfit && frameGroup.patternX === 4}
          <span class="t-label">Direction</span>
          {#each ["arrowN", "arrowE", "arrowS", "arrowW"] as icon, d}
            <button class="t-icon-btn" class:on={pos.x === d} title={["North", "East", "South", "West"][d]} onclick={() => setDir(d)}><Icon name={icon} /></button>
          {/each}
        {:else if frameGroup.patternX > 1}
          <div class="row"><span class="t-label">Pattern X</span><Slider max={frameGroup.patternX - 1} bind:value={pos.x} /><span class="t-value num">{pos.x}</span></div>
        {/if}
        {#if isOutfit}
          {#if frameGroup.patternY > 1}
            <label class="row"><input class="t-check" type="checkbox" bind:checked={addon1} />Addon 1</label>
          {/if}
          {#if frameGroup.patternY > 2}
            <label class="row"><input class="t-check" type="checkbox" bind:checked={addon2} />Addon 2</label>
          {/if}
          {#if frameGroup.patternZ > 1}
            <label class="row" title="Show the outfit riding another looktype"><input class="t-check" type="checkbox" bind:checked={mount} />Mount</label>
            {#if mount && !readonly}
              <NumberField value={mountId} min={0} max={app.maxId(Category.CategoryOutfit)} width={50} title="Mount looktype (0 = none)" onchange={(value) => (mountId = value)} />
            {/if}
          {/if}
        {:else}
          {#if frameGroup.patternY > 1}
            <div class="row"><span class="t-label">Pattern Y</span><Slider max={frameGroup.patternY - 1} bind:value={pos.y} /><span class="t-value num">{pos.y}</span></div>
          {/if}
          {#if frameGroup.patternZ > 1}
            <div class="row"><span class="t-label">Pattern Z</span><Slider max={frameGroup.patternZ - 1} bind:value={pos.z} /><span class="t-value num">{pos.z}</span></div>
          {/if}
        {/if}
        {#if frameGroup.layers > 1}
          <div class="row" title="Show the layer instead of the composed outfit">
            <span class="t-label">Layer</span>
            <Slider max={frameGroup.layers - 1} bind:value={pos.layer} />
            <span class="t-value num">{pos.layer}</span>
          </div>
        {/if}
      </div>

      {#if editing && !sheetView}
        <div class="row edit">
          <button class="t-icon-btn" class:on={tool === "pencil"} title="Pencil" onclick={() => (tool = "pencil")}><Icon name="pencil" /></button>
          <button class="t-icon-btn" class:on={tool === "eraser"} title="Eraser" onclick={() => (tool = "eraser")}><Icon name="eraser" /></button>
          <button class="t-icon-btn" class:on={tool === "picker"} title="Pick a color from the texture" onclick={() => (tool = "picker")}><Icon name="picker" /></button>
          <input class="swatch" type="color" title="Paint color" bind:value={paintHex} />
          <span class="t-vsep"></span>
          <span class="t-label">Shift</span>
          <button class="t-icon-btn" title="Move pixels left" onclick={() => shift(-1, 0)}><Icon name="arrowW" /></button>
          <button class="t-icon-btn" title="Move pixels up" onclick={() => shift(0, -1)}><Icon name="arrowN" /></button>
          <button class="t-icon-btn" title="Move pixels down" onclick={() => shift(0, 1)}><Icon name="arrowS" /></button>
          <button class="t-icon-btn" title="Move pixels right" onclick={() => shift(1, 0)}><Icon name="arrowE" /></button>
          <label class="row" title="Move every frame and pattern, or only the shown texture (with all its layers)"
            ><input class="t-check" type="checkbox" bind:checked={shiftAll} />All textures</label
          >
        </div>
      {/if}
      {#if isOutfit && frameGroup.layers > 1}
        <div class="row">
          <label class="row"><input class="t-check" type="checkbox" bind:checked={colorize} />Colors</label>
          <ColorPicker label="Head" value={colors.head} onchange={(value) => (colors.head = value)} />
          <ColorPicker label="Body" value={colors.body} onchange={(value) => (colors.body = value)} />
          <ColorPicker label="Legs" value={colors.legs} onchange={(value) => (colors.legs = value)} />
          <ColorPicker label="Feet" value={colors.feet} onchange={(value) => (colors.feet = value)} />
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
  .preview:global(.file-drop-target-active) {
    outline: 1px dashed var(--gold);
    outline-offset: 2px;
  }
  .stage-wrap {
    flex: 1;
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
    position: relative;
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    display: grid;
    place-items: center;
    border: 1px solid #111;
    box-shadow: inset 0 0 0 1px #4a4a4a;
  }
  .canvas-wrap {
    position: absolute;
    left: 50%;
    top: 50%;
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.5);
  }
  .slots {
    position: absolute;
    inset: 0;
  }
  .canvas-wrap.editing {
    cursor: crosshair;
  }
  .canvas-wrap.editing .slots {
    pointer-events: none;
  }
  .swatch {
    width: 26px;
    height: 20px;
    padding: 0;
    border: 1px solid #111;
    background: none;
    cursor: pointer;
  }
  .stage.pan-ready {
    cursor: grab;
  }
  .stage.pan-ready .canvas-wrap,
  .stage.panning .canvas-wrap {
    cursor: inherit;
  }
  .stage.panning {
    cursor: grabbing;
  }
  .stage.pan-ready .slots,
  .stage.panning .slots {
    pointer-events: none;
  }
  .pixel-grid {
    position: absolute;
    inset: 0;
    pointer-events: none;
    background-image:
      linear-gradient(to right, rgba(128, 128, 128, 0.35) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(128, 128, 128, 0.35) 1px, transparent 1px);
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
  .zoom.auto {
    color: var(--gold);
    cursor: default;
  }
  .num {
    min-width: 16px;
  }
</style>
