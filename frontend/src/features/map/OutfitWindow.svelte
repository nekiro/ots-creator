<script lang="ts" module>
  import type { PlayerSettings } from "../../lib/map/play/settings.svelte";

  export type Outfit = Pick<PlayerSettings, "lookType" | "head" | "body" | "legs" | "feet" | "addons" | "mount" | "mounted" | "level">;
</script>

<script lang="ts">
  // The client's outfit window (Set Outfit), simpler: the outfit and the
  // mount are picked from a list of every looktype, searched by id or by the
  // creatures that wear it, with the addons and the four colors on the
  // client's palette, and a preview turned to every side.
  import { untrack } from "svelte";
  import { Category, normalizeThing, ThingService, type Thing } from "../../lib/api";
  import { frameGroupAt } from "../../lib/thing";
  import { renderPreviewImage } from "../../lib/render/previewImage";
  import { DEFAULT_COLORS, hsiToRgb, PALETTE_SIZE } from "../../lib/render/outfit";
  import { spriteCache } from "../../lib/render/sprites";
  import { map } from "../../lib/map/state.svelte";
  import { playerSpeed, stepDuration } from "../../lib/map/play/walk";
  import NumberField from "../../lib/ui/NumberField.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import SearchField from "../../lib/ui/SearchField.svelte";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import ItemIcon from "./ItemIcon.svelte";
  import "./map.css";

  let { outfit, onapply, onclose }: { outfit: Outfit; onapply: (outfit: Outfit) => void; onclose: () => void } = $props();

  const draft = $state<Outfit>(untrack(() => ({ ...outfit })));
  /** The list picks the outfit or the mount. */
  let tab = $state<"outfit" | "mount">("outfit");
  let search = $state("");
  /** Direction the preview faces (pattern X): north, east, south, west. */
  let direction = $state(2);

  type ColorPart = "head" | "body" | "legs" | "feet";
  const PARTS: { part: ColorPart; label: string }[] = [
    { part: "head", label: "Head" },
    { part: "body", label: "Body" },
    { part: "legs", label: "Legs" },
    { part: "feet", label: "Feet" },
  ];
  let part = $state<ColorPart>("head");
  const hex = (color: number) => "#" + hsiToRgb(color).toString(16).padStart(6, "0");
  const palette = Array.from({ length: PALETTE_SIZE }, (_, color) => color);

  /** Creatures of the palette by the looktype they wear: names for the search and the tooltips. */
  const wornBy = $derived.by(() => {
    const names = new Map<number, string[]>();
    for (const creature of map.palette?.creatures ?? []) {
      if (!creature || creature.outfit.lookItem) continue;
      const lookType = creature.outfit.lookType;
      if (!lookType) continue;
      const list = names.get(lookType);
      if (list) list.push(creature.name);
      else names.set(lookType, [creature.name]);
    }
    return names;
  });

  /** Looktypes with sprites (empty ones are not shown) and the names asset clients give them. */
  let usedLookTypes = $state.raw<number[]>([]);
  let thingNames = $state.raw<Record<number, string>>({});
  $effect(() => {
    let stale = false;
    ThingService.Find(Category.CategoryOutfit, { content: "used", flag: "", name: "" }).then(
      (found) => !stale && (usedLookTypes = found ?? []),
      () => {},
    );
    ThingService.Names(Category.CategoryOutfit).then(
      (names) => !stale && (thingNames = (names ?? {}) as Record<number, string>),
      () => {},
    );
    return () => (stale = true);
  });

  const lookTypes = $derived.by(() => {
    const query = search.trim().toLowerCase();
    if (!query) return usedLookTypes;
    return usedLookTypes.filter(
      (id) => String(id).includes(query) || thingNames[id]?.toLowerCase().includes(query) || wornBy.get(id)?.some((name) => name.toLowerCase().includes(query)),
    );
  });

  const chosen = $derived(tab === "outfit" ? draft.lookType : draft.mount);
  const chosenIndex = $derived(lookTypes.indexOf(chosen));

  function choose(lookType: number): void {
    if (tab === "outfit") draft.lookType = lookType;
    else {
      draft.mount = lookType;
      draft.mounted = true;
    }
  }

  /** The arrows of the client's window: the previous or next looktype of the list. */
  function cycle(step: number): void {
    if (!lookTypes.length) return;
    const index = chosenIndex < 0 ? 0 : (chosenIndex + step + lookTypes.length) % lookTypes.length;
    choose(lookTypes[index]);
  }

  function toggleAddon(bit: number, on: boolean): void {
    draft.addons = on ? draft.addons | bit : draft.addons & ~bit;
  }

  function title(lookType: number): string {
    const names = [...(thingNames[lookType] ? [thingNames[lookType]] : []), ...(wornBy.get(lookType) ?? [])];
    return names.length ? `${lookType} · ${names.slice(0, 6).join(", ")}${names.length > 6 ? "…" : ""}` : String(lookType);
  }

  // The preview: the outfit with its colors and addons, on its mount.
  let outfitThing = $state.raw<Thing | null>(null);
  let mountThing = $state.raw<Thing | null>(null);
  let loaded = $state(0);
  let canvas = $state<HTMLCanvasElement>();
  let previewSize = $state({ width: 0, height: 0 });

  function loadThing(lookType: number, set: (thing: Thing | null) => void): () => void {
    let stale = false;
    if (lookType < 1) set(null);
    else
      ThingService.Get(Category.CategoryOutfit, lookType).then(
        (thing) => !stale && set(thing ? normalizeThing(thing) : null),
        () => !stale && set(null),
      );
    return () => (stale = true);
  }
  $effect(() => loadThing(draft.lookType, (thing) => (outfitThing = thing)));
  $effect(() => loadThing(draft.mounted ? draft.mount : 0, (thing) => (mountThing = thing)));

  const outfitGroup = $derived(frameGroupAt(outfitThing, 0));
  const mountGroup = $derived(frameGroupAt(mountThing, 0));

  $effect(() => {
    const sprites = [...(outfitGroup?.sprites ?? []), ...(mountGroup?.sprites ?? [])];
    if (sprites.length) void spriteCache.load(sprites).then(() => loaded++);
  });

  $effect(() => {
    void loaded;
    if (!canvas || !outfitGroup) return;
    const colors = { ...DEFAULT_COLORS, head: draft.head, body: draft.body, legs: draft.legs, feet: draft.feet };
    const { image } = renderPreviewImage({
      frameGroup: outfitGroup,
      mountGroup,
      size: spriteCache.size,
      pos: { layer: 0, x: Math.min(direction, outfitGroup.patternX - 1), y: 0, z: 0, frame: 0 },
      isOutfit: true,
      mount: draft.mounted && !!mountGroup,
      addon1: (draft.addons & 1) !== 0,
      addon2: (draft.addons & 2) !== 0,
      colors,
      get: (id) => spriteCache.get(id)?.pixels,
    });
    canvas.width = image.width;
    canvas.height = image.height;
    canvas.getContext("2d")!.putImageData(image, 0, 0);
    previewSize = { width: image.width, height: image.height };
  });

  /** The preview as large as fits its box, in whole steps. */
  const PREVIEW_BOX = 144;
  const previewZoom = $derived(Math.max(1, Math.floor(PREVIEW_BOX / Math.max(previewSize.width, previewSize.height, 1))));

  const CELL = 44;
  const speed = $derived(playerSpeed(draft.level));
</script>

<Dialog title="Set Outfit" width={560} {onclose}>
  <div class="outfit">
    <div class="side">
      <div class="preview t-frame-down" style="width:{PREVIEW_BOX + 8}px;height:{PREVIEW_BOX + 8}px">
        <canvas bind:this={canvas} style="width:{previewSize.width * previewZoom}px;height:{previewSize.height * previewZoom}px"></canvas>
      </div>
      <div class="row center">
        <button class="t-btn sm" title="Turn left" onclick={() => (direction = (direction + 3) % 4)}>⟲</button>
        <button class="t-btn sm" title="Turn right" onclick={() => (direction = (direction + 1) % 4)}>⟳</button>
      </div>
      <div class="row center">
        <button class="t-btn sm" title="Previous {tab}" onclick={() => cycle(-1)}>◀</button>
        <span class="t-label chosen" title={title(chosen)}>{tab === "outfit" ? "Outfit" : "Mount"} {chosen || "none"}</span>
        <button class="t-btn sm" title="Next {tab}" onclick={() => cycle(1)}>▶</button>
      </div>
      <div class="row">
        <label class="row"><input class="t-check" type="checkbox" checked={(draft.addons & 1) !== 0} onchange={(event) => toggleAddon(1, event.currentTarget.checked)} />Addon 1</label>
        <label class="row"><input class="t-check" type="checkbox" checked={(draft.addons & 2) !== 0} onchange={(event) => toggleAddon(2, event.currentTarget.checked)} />Addon 2</label>
      </div>
      <label class="row" title="Ride the mount: any looktype, picked on the Mount tab"><input class="t-check" type="checkbox" bind:checked={draft.mounted} disabled={!draft.mount} />Mounted</label>
      <span class="t-sep"></span>
      <NumberField label="Level" value={draft.level} min={1} max={5000} width={60} title="Sets the speed: 220 + 2 per level above 1" onchange={(value) => (draft.level = value)} />
      <span class="t-label hint">Speed {speed} · step {stepDuration(150, speed)} ms on grass</span>
    </div>

    <div class="main">
      <div class="t-tabs" role="tablist">
        <button class="t-tab" role="tab" class:on={tab === "outfit"} aria-selected={tab === "outfit"} onclick={() => (tab = "outfit")}>Outfit</button>
        <button class="t-tab" role="tab" class:on={tab === "mount"} aria-selected={tab === "mount"} onclick={() => (tab = "mount")}>Mount</button>
      </div>
      <SearchField bind:value={search} placeholder="Looktype or name" />
      <div class="list t-panel">
        {#if lookTypes.length}
          <VirtualGrid count={lookTypes.length} cellWidth={CELL} cellHeight={CELL} gap={2} scrollTo={chosenIndex < 0 ? null : chosenIndex}>
            {#snippet cell(index)}
              {@const lookType = lookTypes[index]}
              <button class="m-cell look" class:on={lookType === chosen} title={title(lookType)} onclick={() => choose(lookType)} ondblclick={() => (choose(lookType), onapply(draft))}>
                <ItemIcon outfit={lookType} size={40} />
              </button>
            {/snippet}
          </VirtualGrid>
        {:else}
          <div class="m-empty">No looktype matches.</div>
        {/if}
      </div>

      <div class="colors">
        <div class="parts">
          {#each PARTS as entry (entry.part)}
            <button class="t-btn sm part" class:on={part === entry.part} onclick={() => (part = entry.part)}>
              <span class="swatch" style="background:{hex(draft[entry.part])}"></span>{entry.label}
            </button>
          {/each}
        </div>
        <div class="palette">
          {#each palette as color (color)}
            <button class="cell" class:on={draft[part] === color} style="background:{hex(color)}" title="{part} {color}" aria-label="Color {color}" onclick={() => (draft[part] = color)}></button>
          {/each}
        </div>
      </div>
    </div>
  </div>

  {#snippet footer()}
    <span class="t-label grow">Double-click a looktype to wear it at once</span>
    <button class="t-btn primary" onclick={() => onapply(draft)}>OK</button>
    <button class="t-btn" onclick={onclose}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .outfit {
    display: flex;
    gap: 10px;
    padding-bottom: 8px;
  }
  .side {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: none;
  }
  .preview {
    display: flex;
    align-items: center;
    justify-content: center;
    background: #1b1b1b;
  }
  .preview canvas {
    image-rendering: pixelated;
  }
  .main {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
    min-width: 0;
  }
  .list {
    display: flex;
    height: 190px;
    padding: 2px;
  }
  .look.m-cell {
    width: 44px;
    height: 44px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .center {
    justify-content: center;
  }
  .chosen {
    min-width: 80px;
    text-align: center;
    color: var(--text-bright);
  }
  .t-btn.sm {
    min-width: 0;
    padding: 0 8px;
  }
  .colors {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
  }
  .parts {
    display: flex;
    gap: 4px;
  }
  .part {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .part.on {
    color: var(--text-bright);
    outline: 1px solid #e8e8e8;
  }
  .swatch {
    width: 10px;
    height: 10px;
    border: 1px solid #000;
  }
  /* The client's palette: 7 rows of 19 colors. */
  .palette {
    display: grid;
    grid-template-columns: repeat(19, 14px);
    gap: 1px;
  }
  .cell {
    width: 14px;
    height: 14px;
    padding: 0;
    border: 1px solid #000;
    cursor: pointer;
  }
  .cell.on,
  .cell:hover {
    outline: 1px solid #fff;
    z-index: 1;
  }
  .grow {
    flex: 1;
  }
  .hint {
    opacity: 0.7;
    white-space: normal;
    max-width: 152px;
  }
</style>
