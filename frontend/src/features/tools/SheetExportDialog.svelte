<script lang="ts">
  // Sprite sheet export with a live preview of the exact image that will be
  // written (ObjectBuilder layout: columns = patterns Z×X × layers,
  // rows = frames × pattern Y).
  import { CATEGORY_NAMES, ThingService, normalizeThing, res, type Thing } from "../../lib/api";
  import { commands } from "../../lib/commands";
  import { IMAGE_FORMATS, prefs } from "../../lib/prefs.svelte";
  import { app, applyDraft, versions } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import PanStage from "../../lib/ui/PanStage.svelte";
  import { ZOOM_LEVELS, fitZoom, stepZoom, zoomLabel } from "../../lib/pan.svelte";

  const category = app.category;
  const id = app.focused!;
  const size = app.project?.info.features.spriteSize ?? 32;

  let thing = $state<Thing | null>(null);
  const group = app.sheetGroup;
  let format = $state(prefs.settings.exportFormat);
  let transparent = $state(prefs.settings.sheetBackground === "transparent");
  let zoom = $state(0); // 0 = fit
  let loading = $state(true);
  let failed = $state(false);
  let natural = $state<[number, number]>([0, 0]);
  let stageSize = $state<[number, number]>([0, 0]);

  // The saved object: the sheet is rendered from it, not from the editor.
  $effect(() => {
    void versions.thing(category, id);
    ThingService.Get(category, id).then((t) => (thing = t ? normalizeThing(t) : null));
  });

  // ALL_GROUPS: idle and walking frames in one sheet (needs one layout).
  const ALL_GROUPS = -1;
  const groups = $derived(thing?.frameGroups ?? []);
  const sameLayout = $derived(
    groups.length === 2 &&
      (["width", "height", "layers", "patternX", "patternY", "patternZ"] as const).every((k) => groups[0][k] === groups[1][k]),
  );
  // The whole outfit by default; one group when the layouts differ.
  let picked = $state<number | null>(null);
  const effective = $derived(picked ?? (groups.length > 1 ? (sameLayout ? ALL_GROUPS : group) : 0));
  const g = $derived.by(() => {
    if (!thing) return null;
    if (effective !== ALL_GROUPS) return groups[Math.min(effective, groups.length - 1)] ?? null;
    return { ...groups[0], frames: groups[0].frames + groups[1].frames, sprites: [...groups[0].sprites, ...groups[1].sprites] };
  });
  const bg = $derived(transparent && format === "png");
  const url = $derived(res.sheet(category, id, effective, bg, versions.thing(category, id)));
  const columns = $derived(g ? g.patternZ * g.patternX * g.layers : 0);
  const rows = $derived(g ? g.frames * g.patternY : 0);
  // Whole pixels when the sheet fits; big sheets shrink to fit.
  const fit = $derived(fitZoom(natural[0], natural[1], stageSize[0], stageSize[1]));
  const scale = $derived(zoom || fit);

  $effect(() => {
    void url;
    loading = true;
    failed = false;
  });

  function zoomBy(d: number) {
    zoom = stepZoom(scale, d);
  }

  // Fit also centers the sheet again.
  let centered = $state(0);
  function zoomFit() {
    zoom = 0;
    centered++;
  }

  function loaded(e: Event) {
    const img = e.currentTarget as HTMLImageElement;
    natural = [img.naturalWidth, img.naturalHeight];
    loading = false;
  }

  async function save() {
    if (await commands.saveSheet(effective, format, bg)) app.dialog = null;
  }
</script>

<Dialog title="Export Sprite Sheet" width={820} onclose={() => (app.dialog = null)}>
  <div class="layout">
    <div class="stage checker">
      <PanStage bind:stageSize zoom={scale} onzoom={zoomBy} resetKey={`${effective}:${centered}`}>
        <img
          class="pixel"
          class:hidden={loading || failed}
          src={url}
          alt="Sprite sheet"
          onload={loaded}
          onerror={() => ((failed = true), (loading = false))}
        />
      </PanStage>
      {#if loading && !failed}
        <div class="loader t-panel" role="status"><span class="spinner"></span>Rendering…</div>
      {/if}
      {#if failed}
        <span class="err">The sheet could not be rendered.</span>
      {/if}
    </div>

    <div class="side col">
      <strong class="name">{CATEGORY_NAMES[category]} #{id}</strong>
      {#if thing && thing.frameGroups.length > 1}
        <div class="t-tabs">
          <button
            class="t-tab"
            class:on={effective === ALL_GROUPS}
            disabled={!sameLayout}
            title={sameLayout ? "Idle and walking frames in one sheet" : "Idle and walking groups have different layouts"}
            onclick={() => (picked = ALL_GROUPS)}>All</button
          >
          <button class="t-tab" class:on={effective === 0} onclick={() => (picked = 0)}>Idle</button>
          <button class="t-tab" class:on={effective === 1} onclick={() => (picked = 1)}>Walking</button>
        </div>
      {/if}
      <label class="row">
        <span class="t-label lbl">Format</span>
        <Select grow value={format} options={IMAGE_FORMATS} onchange={(v) => (format = v)} />
      </label>
      <label class="row">
        <span class="t-label lbl">Background</span>
        <Select
          grow
          value={bg ? "transparent" : "magenta"}
          disabled={format !== "png"}
          options={[
            { value: "magenta", label: "Magenta" },
            { value: "transparent", label: "Transparent" },
          ]}
          onchange={(v) => (transparent = v === "transparent")}
        />
      </label>
      {#if format !== "png"}<p class="t-label note">{format.toUpperCase()} has no transparency: empty pixels are magenta.</p>{/if}

      {#if g}
        <div class="info t-panel">
          <span class="t-label">Image</span><span>{natural[0] || "…"}×{natural[1] || "…"} px</span>
          <span class="t-label">Grid</span><span>{columns}×{rows} textures</span>
          <span class="t-label">Texture</span><span>{g.width * size}×{g.height * size} px</span>
          <span class="t-label">Sprites</span><span>{g.sprites.length}</span>
        </div>
      {/if}

      <div class="row">
        <button class="t-icon-btn" title="Zoom out (Ctrl+wheel)" disabled={scale <= ZOOM_LEVELS[0]} onclick={() => zoomBy(-1)}><Icon name="zoomout" /></button>
        <span class="t-label zoom">{zoomLabel(scale)}{zoom ? "" : " (fit)"}</span>
        <button class="t-icon-btn" title="Zoom in (Ctrl+wheel)" disabled={scale >= ZOOM_LEVELS[ZOOM_LEVELS.length - 1]} onclick={() => zoomBy(1)}><Icon name="zoomin" /></button>
        {#if zoom}<button class="t-btn" onclick={zoomFit}>Fit</button>{/if}
      </div>

      {#if app.dirty}
        <div class="notice">
          <span>The editor has unapplied changes; the sheet shows the saved object.</span>
          <button class="t-btn" onclick={applyDraft}>Apply now</button>
        </div>
      {/if}
    </div>
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!g || failed || !!app.busy} onclick={save}>Export…</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .layout {
    display: flex;
    gap: 10px;
    height: 420px;
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
  .stage img {
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.6);
  }
  .hidden {
    visibility: hidden;
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
  .side {
    width: 210px;
    flex: none;
    gap: 6px;
  }
  .name {
    color: var(--text-bright);
  }
  .lbl {
    width: 70px;
  }
  .note {
    margin: 0;
    font: var(--fs-small) / 10px var(--font-small);
  }
  .info {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2px 8px;
    padding: 6px 8px;
  }
  .zoom {
    min-width: 54px;
    text-align: center;
  }
  .notice {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 8px;
    color: var(--gold);
    border: 1px solid rgba(232, 196, 106, 0.5);
    background: rgba(232, 196, 106, 0.08);
  }
  .err {
    color: #ff9c9c;
  }
</style>
