// Walking rules of the voidcores client and server (OTClient Creature,
// LocalPlayer, game_walk; TFS Creature, Player): step durations from the
// player's speed and the ground, the walk offset while a step is drawn and
// the foot animation.

/** Directions in the client's order (Otc::Direction). */
export const Direction = { north: 0, east: 1, south: 2, west: 3, northEast: 4, southEast: 5, southWest: 6, northWest: 7 } as const;
export type Direction = (typeof Direction)[keyof typeof Direction];

/** Tile steps of each direction. */
const OFFSETS: Record<Direction, [number, number]> = {
  0: [0, -1],
  1: [1, 0],
  2: [0, 1],
  3: [-1, 0],
  4: [1, -1],
  5: [1, 1],
  6: [-1, 1],
  7: [-1, -1],
};

export function directionOffset(direction: Direction): [number, number] {
  return OFFSETS[direction];
}

export function isDiagonal(direction: Direction): boolean {
  return direction >= Direction.northEast;
}

/** The outfit pattern of a direction: diagonals face east or west (Creature::getDrawDirection). */
export function outfitDirection(direction: Direction): number {
  if (direction === Direction.northEast || direction === Direction.southEast) return Direction.east;
  if (direction === Direction.northWest || direction === Direction.southWest) return Direction.west;
  return direction;
}

/** The diagonal two straight directions make, or the newest when they do not (game_walk smartWalk). */
export function combineDirections(newest: Direction, other: Direction | undefined): Direction {
  if (other === undefined) return newest;
  const pair = new Set([newest, other]);
  if (pair.has(Direction.north) && pair.has(Direction.east)) return Direction.northEast;
  if (pair.has(Direction.south) && pair.has(Direction.east)) return Direction.southEast;
  if (pair.has(Direction.south) && pair.has(Direction.west)) return Direction.southWest;
  if (pair.has(Direction.north) && pair.has(Direction.west)) return Direction.northWest;
  return newest;
}

/** Speed formula constants the server sends at login (TFS Creature::speedA/B/C). */
const SPEED_A = 857.36;
const SPEED_B = 261.29;
const SPEED_C = -4795.01;
/** Base speed of every vocation, plus this much per level above 1 (TFS Player::updateBaseSpeed). */
export const BASE_SPEED = 220;
const SPEED_PER_LEVEL = 2;
const MIN_SPEED = 80;
const MAX_SPEED = 6000;
/** Step durations are rounded up to whole server beats (ms). */
const SERVER_BEAT = 50;
/** Ground speed of a tile without a ground speed. */
const DEFAULT_GROUND_SPEED = 150;
/** Ground speed of a tile without a ground (client Tile::getGroundSpeed). */
const NO_GROUND_SPEED = 100;
/** A player's diagonal step takes this many straight steps (setup.otml player diagonal-walk-speed). */
export const PLAYER_DIAGONAL_COST = 2;

/** Step speed of a player of a level (Player::getStepSpeed). */
export function playerSpeed(level: number): number {
  return Math.min(Math.max(BASE_SPEED + SPEED_PER_LEVEL * (Math.max(level, 1) - 1), MIN_SPEED), MAX_SPEED);
}

/** The speed the step formula works with: the client gets half the step speed (Creature::setSpeed). */
export function calculatedStepSpeed(speed: number): number {
  return Math.floor(SPEED_A * Math.log(Math.floor(speed / 2) + SPEED_B) + SPEED_C + 0.5) || 1;
}

/** Duration of a straight step onto a ground (ms; groundSpeed null: no ground). */
export function stepDuration(groundSpeed: number | null, speed: number): number {
  const ground = groundSpeed === null ? NO_GROUND_SPEED : groundSpeed || DEFAULT_GROUND_SPEED;
  const duration = Math.floor((1000 * ground) / calculatedStepSpeed(speed));
  return Math.ceil(duration / SERVER_BEAT) * SERVER_BEAT;
}

/** Offset (world pixels at a sprite size) of a creature from its tile while it walks: it points back to the tile it left. */
export function walkOffset(direction: Direction, walked: number, spriteSize: number): [number, number] {
  let x = 0;
  let y = 0;
  if (direction === Direction.north || direction === Direction.northEast || direction === Direction.northWest) y = spriteSize - walked;
  else if (direction === Direction.south || direction === Direction.southEast || direction === Direction.southWest) y = walked - spriteSize;
  if (direction === Direction.east || direction === Direction.northEast || direction === Direction.southEast) x = walked - spriteSize;
  else if (direction === Direction.west || direction === Direction.northWest || direction === Direction.southWest) x = spriteSize - walked;
  return [x, y];
}

/**
 * The tile a walking creature is drawn with, from its position: the tile of
 * the 3x3 around it holding the bottom-right corner of its sprite, moved by
 * its walk offset and displacement (Creature::updateWalkingTile).
 */
export function walkingTileOffset(offsetX: number, offsetY: number, displacementX: number, displacementY: number, spriteSize: number): [number, number] {
  const right = 2 * spriteSize - 1 + offsetX - displacementX;
  const bottom = 2 * spriteSize - 1 + offsetY - displacementY;
  return [Math.min(Math.max(Math.floor(right / spriteSize), 0), 2) - 1, Math.min(Math.max(Math.floor(bottom / spriteSize), 0), 2) - 1];
}

/** How long each foot animation phase lasts while walking (Creature::updateWalkAnimation). */
export function footPhaseDelay(phases: number, straightStepDuration: number): number {
  const walkSpeed = straightStepDuration;
  const cycle = phases > 2 ? Math.floor(((walkSpeed + 20) * 3) / 2) : walkSpeed + 20;
  return Math.max(Math.floor(cycle / phases) + 10, 20);
}
