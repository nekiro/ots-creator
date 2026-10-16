// What the map renderer draws in a frame and what it reports back.
import type { SpriteCache } from "../../render/sprites";
import type { Appearances } from "../data/appearances";
import type { ChunkStore, CreatureData } from "../data/chunks";
import type { GeoStore } from "./geo";
import type { ItemIndex } from "../data/items";

export interface ViewOptions {
  grid: boolean;
  lowerFloors: boolean;
  ghostHigher: boolean;
  shade: boolean;
  creatures: boolean;
  /** Names over creatures, houses, temples and waypoints. */
  labels: boolean;
  spawns: boolean;
  houses: boolean;
  special: boolean;
  zones: boolean;
  pathing: boolean;
  animation: boolean;
  onlyModified: boolean;
  highlightItems: boolean;
  wallHooks: boolean;
  markers: boolean;
  tooltips: boolean;
  /** x, y, z written on every tile. */
  coords: boolean;
  ghostItems: boolean;
  clientBox: boolean;
  /** Darken the map to the ambient light and light it with item lights, as the client does. */
  lights: boolean;
  /** Ambient light with lights on, 0 (dark) to 100 % (client: "Ambient light"). */
  ambientLight: number;
}

export interface GhostTile {
  x: number;
  y: number;
  z: number;
  /** Server ids, bottom to top. */
  ids: number[];
  subs?: number[];
  creatures?: CreatureData[];
}

export interface RectMark {
  /** Tile rectangle on a floor (inclusive). */
  x0: number;
  y0: number;
  x1: number;
  y1: number;
  z: number;
  fill?: [number, number, number, number];
  stroke?: [number, number, number, number];
}

export interface Marker {
  x: number;
  y: number;
  z: number;
  label: string;
  /** Whole name, shown on hover when the label is shortened. */
  full?: string;
  color: [number, number, number];
}

export interface Label {
  x: number;
  y: number;
  text: string;
  color: string;
  /** Small font, centered on y (coordinate rulers). */
  small?: boolean;
  align?: "center" | "left";
  /** Left out where it would cover another label (item texts). */
  cull?: boolean;
  /** Drawn over the others, which keep clear of it (labels under the mouse). */
  top?: boolean;
}

/** The play view: the player walking on the map as the client shows it. */
export interface PlayScene {
  /** The player's outfit; dir is the outfit direction. */
  player: CreatureData;
  /** The player's tile (the camera follows it). */
  x: number;
  y: number;
  z: number;
  /** Walk offset from the tile, in world pixels (also moves the camera). */
  offsetX: number;
  offsetY: number;
  /** Walk phase of the feet: 0 standing, from 1 walking. */
  footPhase: number;
  /** Floors drawn: from lastFloor (lowest) up to firstFloor. */
  firstFloor: number;
  lastFloor: number;
  /** Light of the world the server sends (250 day, 40 night). */
  worldLight: number;
}

export interface Scene {
  width: number;
  height: number;
  /** World pixels per tile (the sprite size). */
  spriteSize: number;
  /** Camera center in world pixels (ground floor space). */
  camX: number;
  camY: number;
  /** Device pixels per world pixel. */
  scale: number;
  floor: number;
  /** Tiles, for overlays and previews (only the floor in view is loaded). */
  chunks: ChunkStore;
  /** Chunk quads. */
  geo: GeoStore;
  items: ItemIndex;
  looks: Appearances;
  sprites: SpriteCache;
  opts: ViewOptions;
  time: number;
  /** House highlighted by the house brush (0: none). */
  house: number;
  ghosts: GhostTile[];
  ghostAlpha: number;
  rects: RectMark[];
  markers: Marker[];
  /** Selected things drawn again at this offset (dragging a selection). */
  moveOffset: { dx: number; dy: number; dz: number } | null;
  /** Tile under the mouse (labelled with its position when coords is on). */
  hover: { x: number; y: number; z: number } | null;
  /** Set in the play view. */
  play?: PlayScene | null;
}

export interface FrameResult {
  /** Animated things are visible: draw again soon. */
  animated: boolean;
  /** Data is still loading: draw again when it arrives. */
  pending: boolean;
  labels: Label[];
  /** Zoomed out past drawing sprites: only overlays show. */
  far: boolean;
  /** Chunks are still being built: draw again on the next frame. */
  more: boolean;
}
