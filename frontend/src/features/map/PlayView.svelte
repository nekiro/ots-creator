<script lang="ts">
  // The map as a player sees it in the voidcores client: 17 x 13 tiles
  // around the player, the floors the client shows, its light, and the
  // player walking with the client's keys and timing.
  import { onMount, untrack } from "svelte";
  import { errorMessage } from "../../lib/api";
  import { spriteCache } from "../../lib/render/sprites";
  import { Appearances } from "../../lib/map/data/appearances";
  import { ChunkStore, type CreatureData } from "../../lib/map/data/chunks";
  import type { ItemIndex } from "../../lib/map/data/items";
  import { GeoStore } from "../../lib/map/render/geo";
  import { MapRenderer } from "../../lib/map/render/renderer";
  import type { ViewOptions } from "../../lib/map/render/scene";
  import { PLAY, REDRAW_DELAY } from "../../lib/map/constants";
  import { floorOffset } from "../../lib/map/view/coords";
  import { firstVisibleFloor, lastVisibleFloor } from "../../lib/map/play/floors";
  import { Player, type Position } from "../../lib/map/play/player";
  import { playerSettings } from "../../lib/map/play/settings.svelte";
  import { Direction, outfitDirection, playerSpeed, stepDuration } from "../../lib/map/play/walk";
  import { floorWorld, walkWorld } from "../../lib/map/play/world";
  import { map } from "../../lib/map/state.svelte";
  import { app } from "../../lib/state/app.svelte";
  import { Category } from "../../lib/api";
  import NumberField from "../../lib/ui/NumberField.svelte";
  import Slider from "../../lib/ui/Slider.svelte";
  import ColorPicker from "../ColorPicker.svelte";
  import ItemIcon from "./ItemIcon.svelte";

  let {
    mapId,
    start,
    items,
    spriteSize,
    onexit,
    onpopout = null,
    changes = null,
  }: {
    mapId: number;
    start: Position;
    items: ItemIndex;
    spriteSize: number;
    /** Leaves the play view where the player stands. */
    onexit: (position: Position) => void;
    /** Moves the play view to its own window (null: it is in one). */
    onpopout?: ((position: Position) => void) | null;
    /** Calls back with the chunks edits change (the play window hears of them from the backend); returns the unsubscribe. */
    changes?: ((changed: (keys: number[]) => void) => () => void) | null;
  } = $props();

  let host = $state<HTMLDivElement>();
  let canvas = $state<HTMLCanvasElement>();
  let glError = $state("");
  let canvasCss = $state({ width: 0, height: 0 });
  let renderer: MapRenderer | null = null;
  let width = 0;
  let height = 0;

  // The view is made for one map and start (the parent keys it on the map).
  const initial = untrack(() => ({ mapId, start: { ...start }, items }));
  const chunks = new ChunkStore(initial.mapId, () => requestDraw());
  const geo = new GeoStore(initial.mapId, () => requestDraw());
  const looks = new Appearances(() => requestDraw());
  const player = new Player(initial.start, walkWorld(chunks, initial.items), playerSpeed(playerSettings.level));
  const floors = floorWorld(chunks, initial.items, looks);
  let shown = $state<Position>({ ...initial.start });
  let settingsOpen = $state(false);

  // Only what the client shows: no editor overlays.
  const playView = $derived<ViewOptions>({
    grid: false,
    lowerFloors: true,
    ghostHigher: false,
    shade: true,
    creatures: true,
    labels: false,
    spawns: false,
    houses: false,
    special: false,
    zones: false,
    pathing: false,
    animation: true,
    onlyModified: false,
    highlightItems: false,
    wallHooks: false,
    markers: false,
    tooltips: false,
    coords: false,
    ghostItems: false,
    clientBox: false,
    lights: true,
    ambientLight: map.view.ambientLight,
  });

  $effect(() => {
    player.speed = playerSpeed(playerSettings.level);
    requestDraw();
  });


  let raf = 0;
  let timer = 0;
  function requestDraw() {
    if (!raf) raf = requestAnimationFrame(draw);
  }

  /** The mount ridden (0: none). */
  function mountLook(): number {
    return playerSettings.mounted ? playerSettings.mount : 0;
  }

  function toggleAddon(bit: number, on: boolean): void {
    playerSettings.addons = on ? playerSettings.addons | bit : playerSettings.addons & ~bit;
  }

  const speed = $derived(playerSpeed(playerSettings.level));

  /** Outfits and mounts whose sprites were asked for, every frame and direction at once. */
  const prefetched = new Set<number>();
  function prefetch(lookType: number): void {
    if (!lookType || prefetched.has(lookType)) return;
    const look = looks.outfits.get(lookType);
    if (look === undefined) return;
    prefetched.add(lookType);
    if (!look) return;
    void spriteCache.load([...look.s, ...(look.mv?.s ?? [])]).then(requestDraw);
  }

  /** Walk phases of the feet of the outfit, or of the mount when mounted (Creature::getFootAnimPhases). */
  function footPhases(): number {
    const lookType = mountLook() || playerSettings.lookType;
    const look = looks.outfits.get(lookType);
    if (!look) return 0;
    if (look.mv) return look.mv.f;
    return look.f > 2 ? look.f - 1 : look.f;
  }

  function draw() {
    raf = 0;
    if (!renderer || !width) return;
    const now = performance.now();
    prefetch(playerSettings.lookType);
    prefetch(mountLook());
    player.footPhases = footPhases();
    const moving = player.update(now, spriteSize);
    const position = player.position;
    if (position.x !== shown.x || position.y !== shown.y || position.z !== shown.z) shown = { ...position };
    const [offsetX, offsetY] = player.offset(now, spriteSize);
    const firstFloor = firstVisibleFloor(floors, position);
    const creature: CreatureData = {
      npc: false,
      selected: false,
      dir: outfitDirection(player.direction),
      lookType: playerSettings.lookType,
      head: playerSettings.head,
      body: playerSettings.body,
      legs: playerSettings.legs,
      feet: playerSettings.feet,
      addons: playerSettings.addons,
      mount: mountLook(),
      lookItem: 0,
      name: "Player",
    };
    const offset = floorOffset(position.z);
    let result;
    try {
      result = renderer.render({
        width,
        height,
        spriteSize,
        camX: (position.x - offset + 0.5) * spriteSize + offsetX,
        camY: (position.y - offset + 0.5) * spriteSize + offsetY,
        scale: width / (PLAY.tilesWide * spriteSize),
        floor: position.z,
        chunks,
        geo,
        items,
        looks,
        sprites: spriteCache,
        opts: playView,
        time: now,
        house: 0,
        ghosts: [],
        ghostAlpha: 1,
        rects: [],
        markers: [],
        moveOffset: null,
        hover: null,
        play: {
          player: creature,
          x: position.x,
          y: position.y,
          z: position.z,
          offsetX,
          offsetY,
          footPhase: player.footPhase,
          firstFloor,
          lastFloor: Math.max(firstFloor, lastVisibleFloor(position.z)),
          worldLight: playerSettings.worldLight,
        },
      });
    } catch (err) {
      glError = errorMessage(err);
      return;
    }
    clearTimeout(timer);
    if (moving || result.more) requestDraw();
    else if (result.pending) timer = window.setTimeout(requestDraw, REDRAW_DELAY.pending);
    else if (result.animated) timer = window.setTimeout(requestDraw, REDRAW_DELAY.animated);
    chunks.trim((key) => Math.abs((key & 15) - position.z) < 8);
    geo.trim((key) => Math.abs((key & 15) - position.z) < 8);
  }

  // Keys of the client (modules/game_walk): arrows and the numpad walk, Ctrl+arrows turn.
  const ARROWS: Record<string, Direction> = { ArrowUp: Direction.north, ArrowRight: Direction.east, ArrowDown: Direction.south, ArrowLeft: Direction.west };
  const NUMPAD: Record<string, Direction> = {
    Numpad8: Direction.north,
    Numpad9: Direction.northEast,
    Numpad6: Direction.east,
    Numpad3: Direction.southEast,
    Numpad2: Direction.south,
    Numpad1: Direction.southWest,
    Numpad4: Direction.west,
    Numpad7: Direction.northWest,
  };

  function typing(event: KeyboardEvent): boolean {
    const target = event.target as HTMLElement | null;
    return !!target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable);
  }

  function onkeydown(event: KeyboardEvent) {
    if (typing(event)) return;
    if (event.key === "Escape") {
      event.preventDefault();
      if (settingsOpen) settingsOpen = false;
      else onexit({ ...player.position });
      return;
    }
    const now = performance.now();
    const arrow = ARROWS[event.key];
    if (arrow !== undefined && event.ctrlKey) {
      event.preventDefault();
      player.turn(arrow, now, event.repeat);
      requestDraw();
      return;
    }
    const direction = arrow ?? NUMPAD[event.code];
    if (direction === undefined) return;
    event.preventDefault();
    if (!event.repeat) player.pressWalk(direction, now, event.ctrlKey || event.altKey || event.shiftKey || event.metaKey);
    requestDraw();
  }

  function onkeyup(event: KeyboardEvent) {
    const arrow = ARROWS[event.key];
    const direction = arrow ?? NUMPAD[event.code];
    if (direction === undefined) return;
    if (arrow !== undefined && event.ctrlKey) player.releaseTurn(performance.now());
    player.releaseWalk(direction);
    requestDraw();
  }

  onMount(() => {
    try {
      renderer = new MapRenderer(canvas!);
    } catch (err) {
      glError = errorMessage(err);
    }
    // The view keeps the client's aspect, as large as the window allows.
    const resizeObserver = new ResizeObserver(() => {
      const box = host!.getBoundingClientRect();
      const tile = Math.max(1, Math.min(box.width / PLAY.tilesWide, box.height / PLAY.tilesHigh));
      canvasCss = { width: Math.floor(tile * PLAY.tilesWide), height: Math.floor(tile * PLAY.tilesHigh) };
      const dpr = window.devicePixelRatio || 1;
      width = Math.max(1, Math.round(canvasCss.width * dpr));
      height = Math.max(1, Math.round(canvasCss.height * dpr));
      canvas!.width = width;
      canvas!.height = height;
      cancelAnimationFrame(raf);
      raf = 0;
      draw();
    });
    resizeObserver.observe(host!);
    const offChanges = changes?.((keys) => {
      chunks.invalidate(keys, chunks.rev);
      geo.invalidate(keys);
      requestDraw();
    });
    const offSprites = spriteCache.onInvalidate((ids) => {
      renderer?.invalidateSprites(ids);
      requestDraw();
    });
    return () => {
      resizeObserver.disconnect();
      offChanges?.();
      offSprites();
      cancelAnimationFrame(raf);
      clearTimeout(timer);
      renderer?.dispose();
      renderer = null;
    };
  });

  $effect(() => {
    void JSON.stringify(playerSettings);
    void map.view.ambientLight;
    requestDraw();
  });
</script>

<svelte:window {onkeydown} {onkeyup} onblur={() => player.releaseAll()} />

<div class="play">
  <div class="bar t-flat">
    <button class="t-btn sm" title="Back to the editor where the player stands (Esc)" onclick={() => onexit({ ...player.position })}>Exit</button>
    {#if onpopout}
      <button class="t-btn sm" title="Show the play view in its own window and go on editing here" onclick={() => onpopout({ ...player.position })}>Pop out</button>
    {/if}
    <span class="t-vsep"></span>
    <div class="m-seg" title="Light of the world the server sends">
      <button class:on={playerSettings.worldLight >= PLAY.dayLight} onclick={() => (playerSettings.worldLight = PLAY.dayLight)}>Day</button>
      <button class:on={playerSettings.worldLight <= PLAY.nightLight} onclick={() => (playerSettings.worldLight = PLAY.nightLight)}>Night</button>
    </div>
    <span class="t-label" title="Client option: the least light the map is shown with">Ambient</span>
    <Slider bind:value={map.view.ambientLight} min={0} max={100} width={70} />
    <span class="val">{map.view.ambientLight}%</span>
    <span class="t-vsep"></span>
    <button class="t-btn sm" class:on={settingsOpen} title="Outfit and level of the player" onclick={() => (settingsOpen = !settingsOpen)}>Player</button>
    <span class="grow"></span>
    <span class="t-label pos">{shown.x}, {shown.y}, {shown.z}</span>
    <span class="t-label keys">Arrows / numpad walk · Ctrl+arrows turn · Esc exits</span>
  </div>
  {#if settingsOpen}
    <div class="settings t-flat">
      <span class="look t-frame-down" title="Outfit {playerSettings.lookType}"><ItemIcon outfit={playerSettings.lookType} size={48} /></span>
      <div class="group">
        <NumberField label="Outfit" value={playerSettings.lookType} min={1} max={app.maxId(Category.CategoryOutfit)} width={60} title="Looktype of the player" onchange={(value) => (playerSettings.lookType = value)} />
        <div class="row">
          <label class="row"><input class="t-check" type="checkbox" checked={(playerSettings.addons & 1) !== 0} onchange={(event) => toggleAddon(1, event.currentTarget.checked)} />Addon 1</label>
          <label class="row"><input class="t-check" type="checkbox" checked={(playerSettings.addons & 2) !== 0} onchange={(event) => toggleAddon(2, event.currentTarget.checked)} />Addon 2</label>
        </div>
      </div>
      <span class="t-vsep"></span>
      <div class="colors">
        <ColorPicker label="Head" value={playerSettings.head} onchange={(value) => (playerSettings.head = value)} />
        <ColorPicker label="Body" value={playerSettings.body} onchange={(value) => (playerSettings.body = value)} />
        <ColorPicker label="Legs" value={playerSettings.legs} onchange={(value) => (playerSettings.legs = value)} />
        <ColorPicker label="Feet" value={playerSettings.feet} onchange={(value) => (playerSettings.feet = value)} />
      </div>
      <span class="t-vsep"></span>
      <div class="group">
        <label class="row" title="Ride a mount: any looktype"><input class="t-check" type="checkbox" bind:checked={playerSettings.mounted} />Mount</label>
        {#if playerSettings.mounted}
          <div class="row">
            <span class="look small t-frame-down"><ItemIcon outfit={playerSettings.mount} size={32} /></span>
            <NumberField value={playerSettings.mount} min={0} max={app.maxId(Category.CategoryOutfit)} width={60} title="Mount looktype" onchange={(value) => (playerSettings.mount = value)} />
          </div>
        {/if}
      </div>
      <span class="t-vsep"></span>
      <div class="group">
        <NumberField label="Level" value={playerSettings.level} min={1} max={5000} width={60} title="Sets the speed: 220 + 2 per level above 1" onchange={(value) => (playerSettings.level = value)} />
        <span class="t-label hint">Speed {speed} · step {stepDuration(150, speed)} ms on grass</span>
      </div>
    </div>
  {/if}
  <div class="stage" bind:this={host}>
    <canvas bind:this={canvas} style="width:{canvasCss.width}px;height:{canvasCss.height}px"></canvas>
    {#if glError}<div class="error">{glError}</div>{/if}
  </div>
</div>

<style>
  .play {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 4px;
    flex: none;
    overflow: hidden;
    white-space: nowrap;
  }
  .t-btn.sm {
    min-width: 0;
    padding: 0 8px;
  }
  .t-vsep {
    align-self: center;
    height: 18px;
  }
  .grow {
    flex: 1;
  }
  .val {
    min-width: 30px;
    color: var(--text-bright);
  }
  .keys {
    opacity: 0.7;
  }
  .settings {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    padding: 4px 6px;
    flex: none;
  }
  .settings {
    align-items: center;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .colors {
    display: grid;
    grid-template-columns: auto auto;
    gap: 4px 12px;
  }
  .look {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 56px;
    height: 56px;
  }
  .look.small {
    width: 36px;
    height: 36px;
  }
  .hint {
    opacity: 0.7;
  }
  .stage {
    flex: 1;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #000;
    overflow: hidden;
    position: relative;
  }
  canvas {
    display: block;
    image-rendering: pixelated;
  }
  .error {
    position: absolute;
    color: #f66;
  }
</style>
