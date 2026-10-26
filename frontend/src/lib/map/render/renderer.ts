// WebGL2 map renderer. Tiles are drawn as instanced quads (one per sprite
// piece or colored rectangle). The backend builds the quads of each 32x32
// chunk (geo.ts, app/map_http.go) with sprite ids; they go to a GPU buffer
// as they come and are redrawn with a camera offset (chunkBuffers.ts). The
// shader finds the atlas cell of a sprite in a cell index texture, so atlas
// changes never rebuild chunks; only animated pieces are patched per frame.
// Previews (things.ts) and overlays (overlays.ts) are built every frame in
// screen pixels (quadBatch.ts). Sprites live in a texture array atlas.
import { Atlas } from "./atlas";
import { ChunkBuffers } from "./chunkBuffers";
import { chunkKey, chunkOfTile } from "../data/chunks";
import { TOP_ORDER } from "../data/items";
import { ALPHA, BLOCK_CHUNKS, COLORS, DEPTH_RANGE, GROUND_FLOOR, LIGHT_REACH, LIMITS, MAX_FLOOR, PLAY, ZOOM } from "../constants";
import { GEO, GEO_BLOCK, encodeGeoSignature, geoBits, type GeoChunk } from "./geo";
import { ambientIntensity, DAYLIGHT_INTENSITY, WHITE_LIGHT_COLOR, type ExtraLight } from "./lightMap";
import { walkingTileOffset } from "../play/walk";
import { BAR_COLORS, healthRect, INFO_COLORS, infoLayout, litInfoColor, manaBar, NAME_BASELINE } from "./creatureInfo";
import { wholePixelRatio } from "./labels";
import { AF } from "../data/appearances";
import { TileFlags, type CreatureData } from "../data/chunks";
import type { DrawCreature } from "../data/things";
import { LightPass } from "./lightPass";
import { drawOverlays } from "./overlays";
import { QuadBatch } from "./quadBatch";
import type { FrameResult, Scene } from "./scene";
import { ThingDrawer } from "../data/things";
import { Camera, floorsToDraw, type TileRange } from "../view/view";

/** Floors of the play view, bottom up. */
function playFloors(play: { firstFloor: number; lastFloor: number }): number[] {
  const floors: number[] = [];
  for (let z = Math.max(play.lastFloor, play.firstFloor); z >= play.firstFloor; z--) floors.push(z);
  return floors;
}

/** The chunks a tile range covers (tiles before 0 count as tile 0). */
function chunkRange(range: TileRange): TileRange {
  return { x0: chunkOfTile(Math.max(0, range.x0)), y0: chunkOfTile(Math.max(0, range.y0)), x1: chunkOfTile(Math.max(0, range.x1)), y1: chunkOfTile(Math.max(0, range.y1)) };
}

/** The chunks a frame draws, and how many of them are still loading. */
interface ChunkPlan {
  /** What the chunk quads are built for (view options, house, block size). */
  signature: string;
  /** Chunks a side of what is drawn: 1, or BLOCK_CHUNKS far out. */
  blockSize: number;
  /** Chunks of the floor in view still loading or not uploaded yet. */
  notReady: number;
  /** Per floor: blocks loaded before, drawn under chunks still loading (right after zooming in). */
  underneath: Map<number, GeoChunk[]>;
}

export class MapRenderer {
  readonly gl: WebGL2RenderingContext;
  private batch: QuadBatch;
  private chunks: ChunkBuffers;
  private things: ThingDrawer;
  private lights: LightPass;
  atlas: Atlas | null = null;
  /** Sprites drawn this frame that are not loaded yet. */
  private missing = new Set<number>();

  constructor(readonly canvas: HTMLCanvasElement) {
    const gl = canvas.getContext("webgl2", { alpha: false, antialias: false, premultipliedAlpha: false, preserveDrawingBuffer: true });
    if (!gl) throw new Error("WebGL 2 is not available");
    this.gl = gl;
    // The unit square every quad is drawn from.
    const corners = gl.createBuffer()!;
    gl.bindBuffer(gl.ARRAY_BUFFER, corners);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
    this.batch = new QuadBatch(gl, corners);
    this.chunks = new ChunkBuffers(gl, corners, this.missing);
    this.things = new ThingDrawer(this.batch, this.missing);
    this.lights = new LightPass(gl, corners);
    // Chunk quads of later tiles have greater depth (see chunk.vert).
    gl.clearDepth(0);
    gl.depthFunc(gl.GEQUAL);
  }

  /** Sprites changed in the client: drop them from the atlas (the cell index follows). */
  invalidateSprites(ids: number[] | null): void {
    this.atlas?.invalidate(ids);
  }

  dispose(): void {
    this.chunks.dispose();
    this.lights.dispose();
    this.atlas?.dispose();
    this.gl.getExtension("WEBGL_lose_context")?.loseContext();
  }

  render(scene: Scene): FrameResult {
    const gl = this.gl;
    const result: FrameResult = { animated: false, pending: false, labels: [], far: false, more: false };
    if (!this.atlas || this.atlas.cell !== scene.spriteSize) {
      this.atlas?.dispose();
      this.atlas = new Atlas(gl, scene.spriteSize);
      this.batch.atlas = this.atlas;
      this.chunks.useAtlas(this.atlas);
    }
    const atlas = this.atlas;
    atlas.begin();
    this.chunks.beginFrame();
    this.batch.clear();
    this.missing.clear();
    gl.viewport(0, 0, scene.width, scene.height);
    gl.clearColor(0, 0, 0, 1);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

    const floor = scene.floor;
    const camera = new Camera(scene);
    const tilePx = camera.tilePx;
    const floors = scene.play ? playFloors(scene.play) : floorsToDraw(scene, camera.visibleTiles);
    result.far = tilePx < ZOOM.sprites || camera.visibleTiles > LIMITS.visibleTiles;
    if (result.far) {
      // Overlays only.
      drawOverlays(scene, result, camera, this.batch);
      this.flushScreen(scene);
      return result;
    }

    const { drawItem, drawCreature } = this.things.drawers(scene, scene.scale, tilePx, result);
    const plan = this.planChunks(scene, tilePx);
    // The floor in view requests first.
    const chunksByFloor = new Map<number, GeoChunk[]>();
    chunksByFloor.set(floor, this.chunksInView(scene, camera, floor, plan));
    for (const z of floors) if (z !== floor) chunksByFloor.set(z, this.chunksInView(scene, camera, z, plan));
    const ghostFloor = scene.opts.ghostHigher && floor > 0 && floor !== 8 ? floor - 1 : -1;
    if (ghostFloor >= 0) chunksByFloor.set(ghostFloor, this.chunksInView(scene, camera, ghostFloor, plan));
    // The current floor uploads first.
    for (const chunk of chunksByFloor.get(floor) ?? []) if (!this.chunks.upload(chunk)) plan.notReady++;
    // What is in view has upload time first; the floors above and below take what is left.
    for (const z of floors) for (const chunk of chunksByFloor.get(z)!) this.chunks.upload(chunk);
    // Only while nothing else loads: those requests would hold up what comes into view.
    if (!plan.notReady && scene.geo.idle()) {
      for (const z of [floor - 1, floor + 1]) {
        if (z < 0 || z > MAX_FLOOR || chunksByFloor.has(z)) continue;
        for (const chunk of this.chunksNearby(scene, camera, z, plan)) this.chunks.prefetch(chunk);
      }
    }

    // Draw floors from the bottom up.
    for (const z of floors) {
      const isFloorInView = z === floor;
      // Over the floor right under the one in view and what is under it (client: shadowFloorIntensity).
      if (isFloorInView && floors.includes(floor + 1) && scene.opts.shade) {
        this.batch.rect(0, 0, scene.width, scene.height, scene.play ? PLAY.lowerFloorShade : COLORS.lowerFloorsShade);
        this.flushScreen(scene);
      }
      this.drawChunks(scene, plan.underneath.get(z) ?? [], 1, camera.shift(z), result);
      this.drawChunks(scene, chunksByFloor.get(z)!, 1, camera.shift(z), result);
      if (isFloorInView && scene.play) this.drawPlayer(scene, camera, drawCreature);
      if (isFloorInView && scene.moveOffset) {
        this.things.drawMoved(scene, z, camera.tiles(z, 1, 3), camera, drawItem, drawCreature);
        this.flushScreen(scene);
      }
    }
    if (ghostFloor >= 0) this.drawChunks(scene, chunksByFloor.get(ghostFloor)!, ALPHA.ghostFloor, camera.shift(ghostFloor), result);
    if (scene.play) {
      this.drawPlayLights(scene, camera, floors, plan);
      this.drawCreatureInfo(scene, camera, result);
    }
    else if (scene.opts.lights && tilePx >= ZOOM.detail) this.drawLights(scene, camera, floors, plan, ambientIntensity(scene.opts.ambientLight), []);
    this.things.drawGhosts(scene, camera, drawItem, drawCreature);
    drawOverlays(scene, result, camera, this.batch);
    if (this.missing.size) {
      result.pending = true;
      void scene.sprites.load([...this.missing]);
    }
    if (atlas.pending) result.pending = true;
    if (plan.notReady) result.pending = true;
    this.flushScreen(scene);
    this.chunks.sweep(scene);
    return result;
  }

  /** The player among the things of its floor: drawn after the items of
   * the tile it walks over and before that tile's top items, hidden by
   * what later tiles draw (Tile::drawCreature, Creature::updateWalkingTile). */
  private drawPlayer(scene: Scene, camera: Camera, drawCreature: DrawCreature): void {
    const play = scene.play!;
    const size = scene.spriteSize;
    this.flushScreen(scene);
    const look = this.displacedLook(scene, play.player);
    const [dx, dy] = walkingTileOffset(Math.trunc(play.offsetX), Math.trunc(play.offsetY), look?.offsetX ?? 0, look?.offsetY ?? 0, size);
    const walkX = play.x + dx;
    const walkY = play.y + dy;
    // A diagonal corner tile takes the elevation of the player's own tile.
    const elevation = dx && dy ? this.tileElevation(scene, play.x, play.y, play.z) : this.tileElevation(scene, walkX, walkY, play.z);
    const screenX = camera.tileX(play.x, play.z) + Math.round(play.offsetX * scene.scale) - elevation * scene.scale;
    const screenY = camera.tileY(play.y, play.z) + Math.round(play.offsetY * scene.scale) - elevation * scene.scale;
    drawCreature(play.player, screenX, screenY, play.x, play.y, play.z, 1, play.footPhase);
    // The rank chunk.vert gives the walking tile, between its items and its top items.
    const tileX = walkX - Math.round(scene.camX / size);
    const tileY = walkY - Math.round(scene.camY / size);
    const rank = 8 * (tileX + tileY) + tileX + PLAY.playerRank;
    const gl = this.gl;
    gl.enable(gl.DEPTH_TEST);
    if (this.batch.flush(scene, Math.min(Math.max(rank / DEPTH_RANGE, -1), 1))) this.chunks.unbind();
    gl.disable(gl.DEPTH_TEST);
  }

  /** Names and health bars over the creatures of the player's floor and the
   * player, over the light (Creature::drawInformation). */
  private drawCreatureInfo(scene: Scene, camera: Camera, result: FrameResult): void {
    const play = scene.play!;
    if (!play.names && !play.healthBars) return;
    const px = wholePixelRatio(globalThis.devicePixelRatio);
    const half = scene.spriteSize / 2;
    const parts = { names: play.names, bars: play.healthBars, manaBar: play.manaBar };
    const shift = camera.shift(play.z);
    const info = (creature: CreatureData, x: number, y: number, z: number, offsetX: number, offsetY: number, elevation: number, isPlayer: boolean) => {
      const look = this.displacedLook(scene, creature);
      const pointX = camera.tileX(x, z) + Math.round(offsetX * scene.scale) + (half - (look?.offsetX ?? 0) - elevation) * scene.scale;
      const pointY = camera.tileY(y, z) + Math.round(offsetY * scene.scale) + (-(look?.offsetY ?? 0) - 2 - elevation) * scene.scale;
      const layout = infoLayout(pointX, pointY, px, scene.width, scene.height, isPlayer, parts);
      const covered = !isPlayer && this.isCovered(scene, x, y, z, play.firstFloor);
      // The light of the creature's field (its position while it walks).
      const brightness = this.lights.brightness(x - shift, y - shift);
      const [red, green, blue] = litInfoColor(covered ? INFO_COLORS.covered : INFO_COLORS.fullHealth, brightness, isPlayer ? 0 : undefined);
      if (play.healthBars && (!creature.npc || play.npcHealthBars)) {
        this.batch.rect(layout.bar.x, layout.bar.y, layout.bar.width, layout.bar.height, BAR_COLORS.background);
        const bar = healthRect(layout.bar, px, 100);
        this.batch.rect(bar.x, bar.y, bar.width, bar.height, [red / 255, green / 255, blue / 255, 1]);
        if (isPlayer && play.manaBar) {
          const mana = manaBar(layout.bar, px);
          this.batch.rect(mana.x, mana.y, mana.width, mana.height, BAR_COLORS.background);
          const fill = healthRect(mana, px, 100);
          this.batch.rect(fill.x, fill.y, fill.width, fill.height, BAR_COLORS.mana);
        }
      }
      if (play.names) {
        result.labels.push({ x: layout.nameCenter, y: layout.nameTop + NAME_BASELINE * px, text: creature.name, color: `rgb(${red}, ${green}, ${blue})`, clampX: true, baseline: true });
      }
    };
    // Only the camera floor: MapPosInfo::isInRange rejects any other z.
    const z = play.z;
    const range = camera.tiles(z, 1, 1);
    for (let y = Math.max(0, range.y0); y <= range.y1; y++) {
      for (let x = Math.max(0, range.x0); x <= range.x1; x++) {
        // Requested: the play view loads tile data only around the player.
        const tile = scene.chunks.tile(x, y, z);
        if (!tile?.creatures) continue;
        const elevation = this.tileElevation(scene, x, y, z);
        for (const creature of tile.creatures) info(creature, x, y, z, 0, 0, elevation, false);
      }
    }
    const look = this.displacedLook(scene, play.player);
    const [dx, dy] = walkingTileOffset(Math.trunc(play.offsetX), Math.trunc(play.offsetY), look?.offsetX ?? 0, look?.offsetY ?? 0, scene.spriteSize);
    const elevation = dx && dy ? this.tileElevation(scene, play.x, play.y, z) : this.tileElevation(scene, play.x + dx, play.y + dy, z);
    info(play.player, play.x, play.y, z, play.offsetX, play.offsetY, elevation, true);
    this.flushScreen(scene);
  }

  /** The look whose displacement places a creature (Creature::getDisplacementX): its item, its mount while mounted, else its outfit. */
  private displacedLook(scene: Scene, creature: CreatureData) {
    if (creature.lookItem) return scene.looks.items.get(scene.items.client[creature.lookItem]);
    const outfit = scene.looks.outfits.get(creature.lookType);
    if (creature.mount && outfit && outfit.patternZ > 1) return scene.looks.outfits.get(creature.mount) ?? outfit;
    return outfit;
  }

  /** A floor between the player's and a creature covers it (Map::isCovered): a fully opaque tile over it, or a two tiles wide ground right of that. */
  private isCovered(scene: Scene, x: number, y: number, z: number, firstFloor: number): boolean {
    if (z === 0 || z === firstFloor) return false;
    for (let coveredZ = z - 1, coveredX = x + 1, coveredY = y + 1; coveredZ >= firstFloor; coveredZ--, coveredX++, coveredY++) {
      const tile = scene.chunks.tile(coveredX, coveredY, coveredZ);
      if (tile && tile.flags & TileFlags.opaque) return true;
      const next = scene.chunks.tile(coveredX + 1, coveredY + 1, coveredZ);
      if (next && this.hasTopGround(scene, next.ids)) return true;
    }
    return false;
  }

  /** Tile::hasTopGround: a ground or ground border two tiles a side. */
  private hasTopGround(scene: Scene, ids: Uint16Array): boolean {
    for (const id of ids) {
      const look = scene.looks.items.get(scene.items.client[id]);
      if (look && look.width + look.height === 4 && look.flags & (AF.ground | AF.border)) return true;
    }
    return false;
  }

  /** How far things on a tile are raised: the elevation of its items under the top ones (at most 24 pixels). */
  private tileElevation(scene: Scene, x: number, y: number, z: number): number {
    const tile = scene.chunks.peekTile(x, y, z);
    if (!tile) return 0;
    let elevation = 0;
    for (const id of tile.ids) {
      if (scene.items.top[id] === TOP_ORDER.top) continue;
      elevation += scene.looks.items.get(scene.items.client[id])?.elevation ?? 0;
    }
    return Math.min(elevation, PLAY.maxElevation);
  }

  /** The play view's light: the world light (none under ground), at least
   * the ambient light option, and the light the player always has, as it
   * has none of its own (MapView::updateLight, Creature::drawLight). */
  private drawPlayLights(scene: Scene, camera: Camera, floors: number[], plan: ChunkPlan): void {
    const play = scene.play!;
    const world = play.z > GROUND_FLOOR ? 0 : play.worldLight;
    const ambient = Math.max(ambientIntensity(scene.opts.ambientLight), world);
    const size = scene.spriteSize;
    const extra: ExtraLight[] = [{ x: play.x + 0.5 + play.offsetX / size, y: play.y + 0.5 + play.offsetY / size, intensity: PLAY.playerLight, color: WHITE_LIGHT_COLOR }];
    this.drawLights(scene, camera, floors, plan, ambient, extra);
  }

  /** Darkens the drawn floors to the ambient light, brightened by the
   * lights of their items (previews and overlays drawn later stay lit).
   * extra: lights of the floor in view besides the items'. */
  private drawLights(scene: Scene, camera: Camera, floors: number[], plan: ChunkPlan, ambient: number, extra: ExtraLight[]): void {
    this.lights.clear();
    if (ambient >= DAYLIGHT_INTENSITY) return;
    this.flushScreen(scene);
    const floor = scene.floor;
    const range = camera.tiles(floor, 1, 1);
    const shift = camera.shift(floor);
    const grid = { x: range.x0 - shift, y: range.y0 - shift, width: range.x1 - range.x0 + 1, height: range.y1 - range.y0 + 1 };
    const lightFloors = floors.map((z) => ({ z, chunks: this.chunksLighting(scene, camera, z, plan), shift: camera.shift(z), extra: z === floor ? extra : [] }));
    this.lights.draw(scene.width, scene.height, grid, camera.tileX(range.x0, floor), camera.tileY(range.y0, floor), camera.tilePx, lightFloors, ambient, scene.spriteSize, floor);
    this.chunks.unbind();
  }

  /** The chunks of floor z whose lights can reach into the view: the
   * chunks in view and those up to LIGHT_REACH tiles around it. */
  private chunksLighting(scene: Scene, camera: Camera, z: number, plan: ChunkPlan): GeoChunk[] {
    const span = chunkRange(camera.tiles(z, LIGHT_REACH, LIGHT_REACH));
    const out: GeoChunk[] = [...(plan.underneath.get(z) ?? [])];
    for (let chunkY = span.y0; chunkY <= span.y1; chunkY++) {
      for (let chunkX = span.x0; chunkX <= span.x1; chunkX++) {
        const chunk = scene.geo.get(chunkKey(chunkX, chunkY, z), plan.signature);
        if (chunk) out.push(chunk);
      }
    }
    return out;
  }

  /** Draws chunks over what is drawn, their tiles in drawing order among
   * themselves whatever order the chunks come in (see chunk.vert). */
  private drawChunks(scene: Scene, chunks: GeoChunk[], alpha: number, floorShift: number, result: FrameResult): void {
    if (!chunks.length) return;
    const gl = this.gl;
    gl.clear(gl.DEPTH_BUFFER_BIT);
    gl.enable(gl.DEPTH_TEST);
    for (const chunk of chunks) this.chunks.draw(scene, chunk, alpha, floorShift, result);
    gl.disable(gl.DEPTH_TEST);
  }

  /** What chunk quads are built for besides the chunk itself. Far out,
   * movable items and creatures are too small to see and animation (all
   * the water) would patch thousands of chunks per frame. Far out, chunks
   * also come in blocks of BLOCK_CHUNKS x BLOCK_CHUNKS: fewer draws. */
  private planChunks(scene: Scene, tilePx: number): ChunkPlan {
    const farDetail = tilePx < ZOOM.detail;
    const blockSize = farDetail ? BLOCK_CHUNKS : 1;
    let bits = geoBits(scene.opts);
    if (farDetail) bits = (bits & ~(GEO.animation | GEO.creatures | GEO.ghostItems)) | GEO.far;
    const signature = encodeGeoSignature({ bits, house: scene.opts.houses ? scene.house : 0, blockSize });
    return { signature, blockSize, notReady: 0, underneath: new Map() };
  }

  /** The chunks of floor z in view, requested from the store in drawing
   * order. Chunks still loading count in plan.notReady (on the floor in
   * view) and show the block loaded before under them, or the chunks
   * loaded before in place of a block. */
  private chunksInView(scene: Scene, camera: Camera, z: number, plan: ChunkPlan): GeoChunk[] {
    const { signature, blockSize, underneath } = plan;
    // Sprites reach up and left of their tile: take the chunks right and below too.
    const range = camera.tiles(z, 1, 3);
    const out: GeoChunk[] = [];
    if (range.x1 < 0 || range.y1 < 0) return out;
    const span = chunkRange(range);
    const alignToBlock = (chunk: number) => chunk - (chunk % blockSize);
    // Row by row: a sprite only overlaps tiles up and left of it, which
    // this order has already drawn, as a tile by tile walk would.
    for (let chunkY = alignToBlock(span.y0); chunkY <= span.y1; chunkY += blockSize) {
      for (let chunkX = alignToBlock(span.x0); chunkX <= span.x1; chunkX += blockSize) {
        const key = chunkKey(chunkX, chunkY, z);
        if (blockSize === 1) {
          // Tiles of the floor in view feed the overlays and previews.
          if (z === scene.floor && (camera.tilePx >= ZOOM.tileData || scene.moveOffset)) scene.chunks.get(key);
          const chunk = scene.geo.get(key, signature);
          if (chunk) out.push(chunk);
          else if (chunk === undefined) {
            const blockKey = chunkKey(chunkX - (chunkX % BLOCK_CHUNKS), chunkY - (chunkY % BLOCK_CHUNKS), z) + GEO_BLOCK;
            const block = scene.geo.peek(blockKey);
            if (block) {
              let list = underneath.get(z);
              if (!list) underneath.set(z, (list = []));
              if (!list.includes(block)) list.push(block);
            }
            if (z === scene.floor) plan.notReady++;
          }
          continue;
        }
        const block = scene.geo.get(key + GEO_BLOCK, signature);
        if (block) out.push(block);
        else if (block === undefined) {
          // Until the block arrives, its chunks loaded before show.
          for (let y = chunkY; y < chunkY + blockSize; y++) {
            for (let x = chunkX; x < chunkX + blockSize; x++) {
              const chunk = scene.geo.peek(chunkKey(x, y, z));
              if (chunk) out.push(chunk);
            }
          }
          if (z === scene.floor) plan.notReady++;
        }
      }
    }
    return out;
  }

  /** The chunks of floor z under the view that are built already: floors
   * next to the view, loaded while idle so changing floors finds them. */
  private chunksNearby(scene: Scene, camera: Camera, z: number, plan: ChunkPlan): GeoChunk[] {
    const { signature, blockSize } = plan;
    const range = camera.tiles(z, 1, 3);
    const out: GeoChunk[] = [];
    if (range.x1 < 0 || range.y1 < 0) return out;
    const span = chunkRange(range);
    for (let chunkY = span.y0; chunkY <= span.y1; chunkY += blockSize) {
      for (let chunkX = span.x0; chunkX <= span.x1; chunkX += blockSize) {
        const key = chunkKey(chunkX - (chunkX % blockSize), chunkY - (chunkY % blockSize), z) + (blockSize > 1 ? GEO_BLOCK : 0);
        const chunk = scene.geo.get(key, signature);
        if (chunk) out.push(chunk);
      }
    }
    return out;
  }

  /** Draws the screen-space quads pushed so far. */
  private flushScreen(scene: Scene): void {
    if (!this.batch.flush(scene)) return;
    this.chunks.unbind();
  }
}
