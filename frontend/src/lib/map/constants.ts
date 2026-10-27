// Every constant of the map renderer, by topic: zoom levels, limits, the
// GPU data format and colors. Colors are rgba with channels from 0 to 1,
// label colors are CSS colors.

/** Tile sizes in device pixels from which more of the map is drawn. */
export const ZOOM = {
  /** Sprites at all (below: overlays only). */
  sprites: 1,
  /** Every item, creatures and animation (below: chunks come in blocks without them). */
  detail: 4,
  /** Tile data for overlays and previews is loaded. */
  tileData: 8,
  grid: 8,
  markerNames: 12,
  labels: 24,
} as const;

export const LIMITS = {
  /** Tiles drawn over all floors before the farthest lower floors are left out. */
  floorTiles: 3_000_000,
  /** Tiles of one floor in view above which sprites are not drawn. */
  visibleTiles: 6_000_000,
  /** Bytes of chunk quads uploaded per frame; the rest wait for the next frames. */
  uploadBytes: 32 << 20,
  /** Time spent uploading chunk quads per frame (ms); the rest wait for the next frames. */
  uploadMs: 6,
  /** Chunk buffers kept before the least recently drawn are freed. */
  chunkBuffers: 12000,
  /** Frames a chunk buffer is kept after it was last drawn, once there are chunkBuffers. */
  keepFrames: 60,
  /** Tiles around the view searched for spawns whose radius reaches into it. */
  spawnSearch: 30,
} as const;

export const ATLAS = {
  size: 1024,
  startLayers: 4,
  /** Past this many layers the least recently used cells are reused. */
  maxLayers: 64,
  /** Uploads per frame; more wait for the next frame so panning stays smooth. */
  uploadsPerFrame: 1500,
} as const;

/** Floors run from 0 (highest) to MAX_FLOOR; GROUND_FLOOR is the ground level. */
export const GROUND_FLOOR = 7;
export const MAX_FLOOR = 15;

/** Largest brush size (tiles from the center). */
export const MAX_BRUSH_SIZE = 15;

/** Tiles around the view whose item lights are taken in (lights reach as many tiles as their level; item lights go up to about 10). */
export const LIGHT_REACH = 16;

/** Chunk quads are ranked in depth from -DEPTH_RANGE to DEPTH_RANGE (also in chunk.vert). */
export const DEPTH_RANGE = 524288;

/** The play view, as the Tibia client shows the game. */
export const PLAY = {
  /** Tiles in view (client zoom 13 at a 15:11 aspect: 17 x 13). */
  tilesWide: 17,
  tilesHigh: 13,
  /** Over the floor under the player (client option shadowFloorIntensity 70 %). */
  lowerFloorShade: [0, 0, 0, 0.3],
  /** The player's rank among the things of its tile: after items (0), before top items (0.5, chunk.vert). */
  playerRank: 0.25,
  /** Most a tile raises the things on it (setup.otml max-elevation). */
  maxElevation: 24,
  /** The white light the player has when its own is weaker (Creature::drawLight). */
  playerLight: 2,
  /** The shown world light moves one step towards the server's every this many ms (Map::fadeAmbientLight). */
  worldLightStepMs: 1000,
  /** World light of the server by day and by night (TFS LIGHT_LEVEL_DAY / NIGHT). */
  dayLight: 250,
  nightLight: 40,
} as const;

/** Chunks a side of the blocks drawn far out. */
export const BLOCK_CHUNKS = 4;

export const GPU = {
  /** Floats per screen quad: rectangle (4), atlas cell (2), color (4). */
  floatsPerQuad: 10,
  /** Cell index value of an id that is not in the atlas (also in chunk.vert). */
  missingCell: 0xffffffff,
} as const;

export const COLORS = {
  /** Over the lower floors, under the floor in view. */
  lowerFloorsShade: [0, 0, 0, 0.45],
  grid: [1, 1, 1, 0.12],
  spawn: {
    selected: [1, 1, 0.4, 0.9],
    npc: [0.4, 0.7, 1, 0.7],
    monster: [1, 0.45, 0.45, 0.7],
  },
  clientBox: [1, 1, 1, 0.6],
  hoverTile: [1, 1, 1, 0.12],
  positionTag: [0, 0, 0, 0.75],
  movedTile: [0.4, 0.7, 1, 0.15],
  /** Things that cannot be drawn: alpha is multiplied by the thing's alpha. */
  unknownItem: [0.8, 0, 0, 0.6],
  itemWithoutLook: [0.8, 0, 0, 0.5],
  unknownNpc: [0.3, 0.6, 1, 0.8],
  unknownMonster: [1, 0.5, 0.2, 0.8],
} as const;

export const ALPHA = {
  /** The floor above drawn over the floor in view (ghostHigher). */
  ghostFloor: 0.3,
  spawnCenter: 0.25,
  marker: 0.9,
  moved: 0.6,
  /** Shade (rgb multiplier) of selected creatures. */
  selectedShade: 0.5,
} as const;

export const LABEL_COLORS = {
  creatureName: "#00c000",
  itemText: "#e0d29a",
  position: "#e8e8e8",
  tooltip: "#ffd75e",
} as const;

/** Brush kinds that paint the same item over an area (rectangle previews show it). */
export const AREA_BRUSH_KINDS: ReadonlySet<string> = new Set(["raw", "ground", "wall", "carpet", "table", "eraser", "house", "flag", "zone", "optionalBorder"]);
/** Brush kinds that paint along the mouse while the button is held. */
export const SMEAR_BRUSH_KINDS: ReadonlySet<string> = new Set([...AREA_BRUSH_KINDS, "creature"]);
export const RECT_BRUSH_KINDS: ReadonlySet<string> = AREA_BRUSH_KINDS;

type Rgb = [number, number, number];
type Rgba = [number, number, number, number];

/** Colors of the brush, paste and selection previews (rgba, 0 to 1). */
export const PREVIEW_COLORS: {
  selectionBox: { fill: Rgba; stroke: Rgba };
  rect: { fill: Rgba; eraseFill: Rgba; stroke: Rgba };
  pasteBounds: Rgba;
  erasedTile: Rgba;
  erasedCreature: Rgba;
  spawn: { stroke: Rgba; eraseStroke: Rgba; fill: Rgba };
  singleTile: { stroke: Rgba; fill: Rgba };
  fill: { stroke: Rgba; fill: Rgba };
  houseTile: Rgba;
  markTile: Rgba;
  brushBounds: Rgba;
} = {
  selectionBox: { fill: [0.4, 0.6, 1, 0.12], stroke: [0.6, 0.8, 1, 0.9] },
  rect: { fill: [1, 1, 1, 0.12], eraseFill: [1, 0.2, 0.2, 0.25], stroke: [1, 1, 1, 0.7] },
  pasteBounds: [1, 0.85, 0.3, 0.8],
  erasedTile: [1, 0.2, 0.2, 0.3],
  erasedCreature: [1, 0.2, 0.2, 0.35],
  spawn: { stroke: [1, 0.6, 0.6, 0.9], eraseStroke: [1, 0.3, 0.3, 0.9], fill: [1, 0.5, 0.5, 0.08] },
  singleTile: { stroke: [1, 1, 1, 0.9], fill: [1, 1, 1, 0.15] },
  fill: { stroke: [1, 0.85, 0.3, 1], fill: [1, 0.85, 0.3, 0.25] },
  houseTile: [0.4, 0.6, 1, 0.3],
  markTile: [0.3, 1, 0.4, 0.25],
  brushBounds: [1, 1, 1, 0.35],
};

export const GHOST_ALPHA = { paste: 0.65, brush: 0.55 } as const;

export const MAX_RECT_PREVIEW_TILES = 4000;

/** Marker colors (rgb): the color tells the kind, so labels carry a short name. */
export const MARKER_COLORS: { waypoint: Rgb; houseExit: Rgb; temple: Rgb } = {
  waypoint: [0.3, 1, 0.4],
  houseExit: [0.4, 0.7, 1],
  temple: [1, 0.85, 0.3],
};

export const LABEL_FONT = {
  /** Size of the client's name font in CSS pixels. */
  nameSize: 14,
  smallSize: 9,
  maxNames: 1000,
  maxMarkerChars: 18,
} as const;

/** Mouse travel in CSS pixels before a press counts as a drag. */
export const DRAG_THRESHOLD = { pan: 3, selectionBox: 4 } as const;

/** Delays before drawing again (ms). */
export const REDRAW_DELAY = { pending: 60, animated: 90 } as const;

export const WHEEL_PAN_TILES = 3;

export const FPS_POLL_MS = 250;
