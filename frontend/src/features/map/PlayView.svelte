<script lang="ts">
  // The map as a player sees it in the Tibia client: 17 x 13 tiles
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
  import { MAX_FLOOR, PLAY, REDRAW_DELAY } from "../../lib/map/constants";
  import { drawLabels, loadLabelFonts } from "../../lib/map/render/labels";
  import { floorOffset } from "../../lib/map/view/coords";
  import { firstVisibleFloor, lastVisibleFloor } from "../../lib/map/play/floors";
  import { Player, type Position } from "../../lib/map/play/player";
  import { playerSettings } from "../../lib/map/play/settings.svelte";
  import { Direction, outfitDirection, playerSpeed } from "../../lib/map/play/walk";
  import { floorWorld, walkWorld } from "../../lib/map/play/world";
  import { map } from "../../lib/map/state.svelte";
  import { app } from "../../lib/state/app.svelte";
  import Slider from "../../lib/ui/Slider.svelte";
  import { isTyping } from "../../lib/ui/typing";
  import OutfitWindow, { type Outfit } from "./OutfitWindow.svelte";

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
    onexit: (position: Position) => void;
    onpopout?: ((position: Position) => void) | null;
    /** Calls back with the chunks edits change (the play window hears of them from the backend); returns the unsubscribe. */
    changes?: ((changed: (keys: number[]) => void) => () => void) | null;
  } = $props();

  let host = $state<HTMLDivElement>();
  let canvas = $state<HTMLCanvasElement>();
  let overlay = $state<HTMLCanvasElement>();
  let glError = $state("");
  let canvasCss = $state({ width: 0, height: 0 });
  let renderer: MapRenderer | null = null;
  let width = 0;
  let height = 0;

  const initial = untrack(() => ({ mapId, start: { ...start }, items }));
  const chunks = new ChunkStore(initial.mapId, () => requestDraw());
  const geo = new GeoStore(initial.mapId, () => requestDraw());
  const looks = new Appearances(() => requestDraw());
  const player = new Player(initial.start, walkWorld(chunks, initial.items), playerSpeed(playerSettings.level));
  const floors = floorWorld(chunks, initial.items, looks);
  let shown = $state<Position>({ ...initial.start });
  let outfitOpen = $state(false);

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
    markedItems: false,
    clientBox: false,
    lights: true,
    ambientLight: map.view.ambientLight,
  });

  $effect(() => {
    player.speed = playerSpeed(playerSettings.level);
    requestDraw();
  });

  /** The world light shown: it fades one step a second towards the one the server sends (Map::setLight). */
  let worldLight = $state(untrack(() => playerSettings.worldLight));
  $effect(() => {
    const target = playerSettings.worldLight;
    const fade = window.setInterval(() => {
      if (worldLight === target) return window.clearInterval(fade);
      worldLight += Math.sign(target - worldLight);
      requestDraw();
    }, PLAY.worldLightStepMs);
    return () => window.clearInterval(fade);
  });

  let raf = 0;
  let timer = 0;
  let shownFloors = { z: -1, firstFloor: 0 };
  function requestDraw() {
    if (!raf) raf = requestAnimationFrame(draw);
  }

  function mountLook(): number {
    return playerSettings.mounted ? playerSettings.mount : 0;
  }

  function applyOutfit(outfit: Outfit): void {
    Object.assign(playerSettings, outfit);
    outfitOpen = false;
  }

  /** Ctrl + right click on the player opens the outfit window, as in the
   * client. The player stands on the middle tile; its sprite reaches up and
   * left by the outfit's displacement. */
  function oncontextmenu(event: MouseEvent): void {
    event.preventDefault();
    if (!event.ctrlKey) return;
    const tile = canvasCss.width / PLAY.tilesWide;
    const look = looks.outfits.get(mountLook() || playerSettings.lookType);
    const reach = Math.max(look?.offsetX ?? 0, look?.offsetY ?? 0) * (tile / spriteSize);
    const left = Math.floor(PLAY.tilesWide / 2) * tile;
    const top = Math.floor(PLAY.tilesHigh / 2) * tile;
    if (event.offsetX >= left - reach && event.offsetX < left + tile && event.offsetY >= top - reach && event.offsetY < top + tile) {
      player.releaseAll();
      outfitOpen = true;
    }
  }

  const prefetched = new Set<number>();
  function prefetch(lookType: number): void {
    if (!lookType || prefetched.has(lookType)) return;
    const look = looks.outfits.get(lookType);
    if (look === undefined) return;
    prefetched.add(lookType);
    if (!look) return;
    void spriteCache.load([...look.sprites, ...(look.moving?.sprites ?? [])]).then(requestDraw);
  }

  /** Walk phases of the feet of the outfit, or of the mount when mounted (Creature::getFootAnimPhases). */
  function footPhases(): number {
    const lookType = mountLook() || playerSettings.lookType;
    const look = looks.outfits.get(lookType);
    if (!look) return 0;
    if (look.moving) return look.moving.frames;
    return look.frames > 2 ? look.frames - 1 : look.frames;
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
    if (position.x !== shown.x || position.y !== shown.y || position.z !== shown.z) {
      shown = { ...position };
      if (onpopout) map.player = { ...position };
    }
    const [offsetX, offsetY] = player.offset(now, spriteSize);
    // While the tiles over the player load, the floors stay as they were (only its own floor after a floor change).
    const firstFloor = firstVisibleFloor(floors, position) ?? (shownFloors.z === position.z ? shownFloors.firstFloor : position.z);
    shownFloors = { z: position.z, firstFloor };
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
          worldLight,
          names: playerSettings.showNames,
          healthBars: playerSettings.showHealth,
          manaBar: playerSettings.showMana,
          npcHealthBars: (app.project?.info.version.value ?? 0) < CLIENT_WITHOUT_NPC_BARS,
        },
      });
    } catch (err) {
      glError = errorMessage(err);
      return;
    }
    const context = overlay?.getContext("2d");
    if (context && overlay) drawLabels(context, overlay.width, overlay.height, result.labels, window.devicePixelRatio);
    clearTimeout(timer);
    if (moving || result.more) requestDraw();
    else if (result.pending) timer = window.setTimeout(requestDraw, REDRAW_DELAY.pending);
    else if (result.animated) timer = window.setTimeout(requestDraw, REDRAW_DELAY.animated);
    chunks.trim((key) => Math.abs((key & 15) - position.z) < 8);
    geo.trim((key) => Math.abs((key & 15) - position.z) < 8);
  }

  /** From this client version NPCs have no health bar (Creature::drawInformation). */
  const CLIENT_WITHOUT_NPC_BARS = 1100;

  void loadLabelFonts().then(() => requestDraw());

  /** PageUp / PageDown: the player goes a floor up or down at once (not in the client: to look around). */
  function changeFloor(step: number): void {
    const z = player.position.z + step;
    if (z < 0 || z > MAX_FLOOR) return;
    player.teleport({ ...player.position, z });
    requestDraw();
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

  function onkeydown(event: KeyboardEvent) {
    if (isTyping(event) || outfitOpen) return;
    if (event.key === "Escape") {
      event.preventDefault();
      onexit({ ...player.position });
      return;
    }
    if (event.ctrlKey && event.code === "KeyR") {
      event.preventDefault();
      if (!event.repeat && playerSettings.mount) playerSettings.mounted = !playerSettings.mounted;
      return;
    }
    if (event.key === "PageUp" || event.key === "PageDown") {
      event.preventDefault();
      changeFloor(event.key === "PageUp" ? -1 : 1);
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
    const resizeObserver = new ResizeObserver(() => {
      const box = host!.getBoundingClientRect();
      const tile = Math.max(1, Math.min(box.width / PLAY.tilesWide, box.height / PLAY.tilesHigh));
      canvasCss = { width: Math.floor(tile * PLAY.tilesWide), height: Math.floor(tile * PLAY.tilesHigh) };
      const dpr = window.devicePixelRatio || 1;
      width = Math.max(1, Math.round(canvasCss.width * dpr));
      height = Math.max(1, Math.round(canvasCss.height * dpr));
      canvas!.width = width;
      canvas!.height = height;
      overlay!.width = width;
      overlay!.height = height;
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
    <!-- No exit button: the editor's toolbar has Stop, the play window closes as any window. -->
    {#if onpopout}
      <button class="t-btn sm" title="Show the play view in its own window and go on editing here" onclick={() => onpopout({ ...player.position })}>Pop out</button>
      <span class="t-vsep"></span>
    {/if}
    <div class="m-seg" title="Light of the world the server sends">
      <button class:on={playerSettings.worldLight >= PLAY.dayLight} onclick={() => (playerSettings.worldLight = PLAY.dayLight)}>Day</button>
      <button class:on={playerSettings.worldLight <= PLAY.nightLight} onclick={() => (playerSettings.worldLight = PLAY.nightLight)}>Night</button>
    </div>
    <span class="t-label" title="Client option: the least light the map is shown with">Ambient</span>
    <Slider bind:value={map.view.ambientLight} min={0} max={100} width={70} />
    <span class="val">{map.view.ambientLight}%</span>
    <span class="t-vsep"></span>
    <div class="m-seg" title="Client options: what is shown over creatures">
      <button class:on={playerSettings.showNames} title="Names over creatures" onclick={() => (playerSettings.showNames = !playerSettings.showNames)}>Names</button>
      <button class:on={playerSettings.showHealth} title="Health bars over creatures" onclick={() => (playerSettings.showHealth = !playerSettings.showHealth)}>Health</button>
      <button class:on={playerSettings.showMana} title="Mana bar under the player's health bar" onclick={() => (playerSettings.showMana = !playerSettings.showMana)}>Mana</button>
    </div>
    <span class="grow"></span>
    <span class="t-label pos">{shown.x}, {shown.y}, {shown.z}</span>
    <span class="t-label keys">Arrows / numpad walk · Ctrl+arrows turn · PgUp/PgDn floors · Ctrl+R mount · Ctrl+right click outfit · Esc exits</span>
  </div>
  <div class="stage" bind:this={host}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="screen" {oncontextmenu} style="width:{canvasCss.width}px;height:{canvasCss.height}px">
      <canvas bind:this={canvas}></canvas>
      <canvas class="labels" bind:this={overlay}></canvas>
    </div>
    {#if glError}<div class="error">{glError}</div>{/if}
  </div>
</div>

{#if outfitOpen}
  <OutfitWindow outfit={playerSettings} onapply={applyOutfit} onclose={() => (outfitOpen = false)} />
{/if}

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
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .bar :global(.m-seg) {
    flex: none;
  }
  .bar :global(.m-seg > button) {
    flex: none;
    min-width: max-content;
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
  .screen {
    position: relative;
    flex: none;
  }
  .labels {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }
  canvas {
    width: 100%;
    height: 100%;
    display: block;
    image-rendering: pixelated;
  }
  .error {
    position: absolute;
    color: #f66;
  }
</style>
