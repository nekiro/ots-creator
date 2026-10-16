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
  /** Grid lines. */
  grid: 8,
  /** Marker names. */
  markerNames: 12,
  /** Creature names, item texts and tooltips. */
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

/** The sprite atlas: a texture array of square cells. */
export const ATLAS = {
  /** Width and height of each layer in pixels. */
  size: 1024,
  startLayers: 4,
  /** Past this many layers the least recently used cells are reused. */
  maxLayers: 64,
  /** Uploads per frame; more wait for the next frame so panning stays smooth. */
  uploadsPerFrame: 1500,
} as const;

/** Chunks a side of the blocks drawn far out. */
export const BLOCK_CHUNKS = 4;

/** The data the shaders read. */
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
  /** Over tiles of a dragged selection. */
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
  /** The tile a spawn is on. */
  spawnCenter: 0.25,
  /** Marker outlines. */
  marker: 0.9,
  /** Things of a dragged selection. */
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
