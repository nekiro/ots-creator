import { describe, expect, it } from "vitest";
import { firstVisibleFloor, lastVisibleFloor, type FloorItemFlags, type FloorWorld } from "./floors";
import { Player, type WalkWorld } from "./player";
import { calculatedStepSpeed, combineDirections, Direction, footPhaseDelay, outfitDirection, playerSpeed, stepDuration, walkingTileOffset, walkOffset } from "./walk";

describe("walk rules", () => {
  it("times steps as the server does", () => {
    expect(calculatedStepSpeed(playerSpeed(1))).toBe(278);
    expect(stepDuration(150, playerSpeed(1))).toBe(550);
    expect(stepDuration(100, playerSpeed(1))).toBe(400);
    expect(stepDuration(0, playerSpeed(1))).toBe(550);
    expect(stepDuration(150, playerSpeed(50))).toBe(400);
    expect(stepDuration(100, playerSpeed(8))).toBe(350);
  });

  it("offsets a walking creature back to the tile it left", () => {
    expect(walkOffset(Direction.north, 0, 32)).toEqual([0, 32]);
    expect(walkOffset(Direction.southWest, 8, 32)).toEqual([24, -24]);
    expect(walkOffset(Direction.east, 32, 32)).toEqual([0, 0]);
  });

  it("draws a walking creature on the tile its bottom right corner is over", () => {
    // Walking south from the tile above: the old tile until about halfway, sooner with displacement.
    expect(walkingTileOffset(0, -32, 0, 0, 32)).toEqual([0, -1]);
    expect(walkingTileOffset(0, -1, 0, 0, 32)).toEqual([0, 0]);
    // Displacement moves the corner up and left: the switch comes later.
    expect(walkingTileOffset(0, -24, 8, 8, 32)).toEqual([0, -1]);
    expect(walkingTileOffset(0, -23, 8, 8, 32)).toEqual([0, 0]);
  });

  it("faces diagonals east or west and combines held keys", () => {
    expect(outfitDirection(Direction.northEast)).toBe(Direction.east);
    expect(outfitDirection(Direction.southWest)).toBe(Direction.west);
    expect(combineDirections(Direction.north, Direction.east)).toBe(Direction.northEast);
    expect(combineDirections(Direction.north, Direction.south)).toBe(Direction.north);
  });

  it("paces the feet by the step", () => {
    expect(footPhaseDelay(8, 550)).toBe(116);
    expect(footPhaseDelay(2, 550)).toBe(295);
  });
});

describe("Player", () => {
  const open: WalkWorld = { isWalkable: (x) => x < 105, groundSpeed: () => 150 };

  it("steps, waits for the step and keeps walking while the key is held", () => {
    const player = new Player({ x: 100, y: 100, z: 7 }, open, playerSpeed(1), 8);
    player.pressWalk(Direction.east, 0, false);
    player.update(0, 32);
    expect(player.position).toEqual({ x: 101, y: 100, z: 7 });
    expect(player.offset(275, 32)).toEqual([-16, 0]);
    player.update(275, 32);
    expect(player.footPhase).toBeGreaterThan(0);
    // Held past the auto repeat delay: the next step starts as soon as the first ends.
    player.update(550, 32);
    expect(player.position.x).toBe(102);
    player.releaseWalk(Direction.east);
    player.update(1100, 32);
    expect(player.walking).toBe(false);
    player.update(1200, 32);
    expect(player.position.x).toBe(102);
    expect(player.footPhase).toBe(0);
  });

  it("does not step onto tiles that cannot be walked", () => {
    const player = new Player({ x: 104, y: 100, z: 7 }, open, playerSpeed(1));
    player.pressWalk(Direction.east, 0, false);
    player.update(0, 32);
    expect(player.position.x).toBe(104);
    expect(player.walking).toBe(false);
  });

  it("waits twice as long after a diagonal step", () => {
    const player = new Player({ x: 100, y: 100, z: 7 }, open, playerSpeed(1));
    player.pressWalk(Direction.north, 0, false);
    player.pressWalk(Direction.east, 0, false);
    player.update(0, 32);
    expect(player.position).toEqual({ x: 101, y: 99, z: 7 });
    player.releaseAll();
    player.update(560, 32);
    expect(player.walking).toBe(false);
    player.pressWalk(Direction.east, 600, false);
    player.update(600, 32);
    expect(player.position.x).toBe(101);
    player.releaseAll();
    player.pressWalk(Direction.east, 1100, false);
    player.update(1100, 32);
    expect(player.position.x).toBe(102);
  });

  it("teleports to another floor without a step", () => {
    const player = new Player({ x: 100, y: 100, z: 7 }, open, playerSpeed(1));
    player.pressWalk(Direction.east, 0, false);
    player.update(0, 32);
    player.teleport({ x: 101, y: 100, z: 6 });
    expect(player.position).toEqual({ x: 101, y: 100, z: 6 });
    expect(player.walking).toBe(false);
    expect(player.offset(100, 32)).toEqual([0, 0]);
  });

  it("turns with Ctrl and holds walking for a moment", () => {
    const player = new Player({ x: 100, y: 100, z: 7 }, open, playerSpeed(1));
    player.turn(Direction.west, 1000, false);
    expect(player.direction).toBe(Direction.west);
    player.pressWalk(Direction.east, 1050, false);
    player.update(1050, 32);
    expect(player.position.x).toBe(100);
  });
});

describe("visible floors", () => {
  const none: FloorItemFlags = { ground: false, border: false, bottom: false, top: false, dontHide: false, blockProjectile: false };
  const GROUND = 1;
  const WALL = 2;
  const flags = (id: number): FloorItemFlags => (id === GROUND ? { ...none, ground: true } : id === WALL ? { ...none, bottom: true, blockProjectile: true } : none);
  function world(tiles: Record<string, number[]>): FloorWorld {
    return { items: (x, y, z) => tiles[`${x},${y},${z}`], flags };
  }

  it("shows every floor above an open sky", () => {
    expect(firstVisibleFloor(world({ "100,100,7": [GROUND] }), { x: 100, y: 100, z: 7 })).toBe(0);
  });

  it("stops at a roof right above the player", () => {
    const map = world({ "100,100,7": [GROUND], "100,100,6": [GROUND] });
    expect(firstVisibleFloor(map, { x: 100, y: 100, z: 7 })).toBe(7);
  });

  it("stops at a floor drawn over the player along the diagonal", () => {
    const map = world({ "100,100,7": [GROUND], "101,101,6": [GROUND] });
    expect(firstVisibleFloor(map, { x: 100, y: 100, z: 7 })).toBe(7);
  });

  it("looks past roofs next to the player only where it can see", () => {
    const open = world({ "100,100,7": [GROUND], "101,100,7": [GROUND], "101,100,5": [GROUND] });
    expect(firstVisibleFloor(open, { x: 100, y: 100, z: 7 })).toBe(6);
    const walled = world({ "100,100,7": [GROUND], "101,100,7": [GROUND, WALL], "101,100,5": [GROUND] });
    expect(firstVisibleFloor(walled, { x: 100, y: 100, z: 7 })).toBe(0);
  });

  it("waits for tiles that are not loaded yet", () => {
    const loading: FloorWorld = { items: (x, y, z) => (z === 6 ? null : x === 100 && y === 100 && z === 7 ? [GROUND] : undefined), flags };
    expect(firstVisibleFloor(loading, { x: 100, y: 100, z: 7 })).toBeNull();
  });

  it("keeps under ground floors near the player", () => {
    expect(firstVisibleFloor(world({ "100,100,10": [GROUND] }), { x: 100, y: 100, z: 10 })).toBe(8);
    expect(lastVisibleFloor(10)).toBe(12);
    expect(lastVisibleFloor(5)).toBe(7);
  });
});
