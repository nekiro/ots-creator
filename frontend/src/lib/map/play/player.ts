// The local player of the play view: walking and turning from the keys as
// the voidcores client does (modules/game_walk, LocalPlayer, Creature).
// The map is local, so every step the client would send succeeds.
import { combineDirections, Direction, directionOffset, footPhaseDelay, isDiagonal, PLAYER_DIAGONAL_COST, stepDuration, walkOffset } from "./walk";

/** What the player walks on. */
export interface WalkWorld {
  /** The tile can be walked onto; false also while it is not loaded (client: Tile::isWalkable). */
  isWalkable(x: number, y: number, z: number): boolean;
  /** Ground speed of a tile (null: no ground). */
  groundSpeed(x: number, y: number, z: number): number | null;
}

export interface Position {
  x: number;
  y: number;
  z: number;
}

/** Keys held this long start repeating (client root panel auto repeat delay, ms). */
const AUTO_REPEAT_DELAY = 200;
/** A direction pressed while stepping is walked this long after the step ends (game_walk onWalkFinish, ms). */
const NEXT_WALK_DELAY = 50;
/** The feet stop this long after the last step (one server beat, ms). */
const FOOT_RESET_DELAY = 50;
/** Walking waits this long after a turn (walkTurnDelay, ms), and after its key is let go. */
const TURN_LOCK = 100;
const TURN_RELEASE_LOCK = 200;
/** Repeated turns of a held key wait this long; a first press this long (game_walk turn, ms). */
const TURN_DELAY_REPEATED = 150;
const TURN_DELAY_FIRST = 50;

/** A step being walked. */
interface Step {
  direction: Direction;
  start: number;
  /** The walk animation ends after a straight step. */
  animationEnd: number;
  /** Diagonal steps take longer: the next step waits until then. */
  stepEnd: number;
  /** Straight step duration (foot animation pace). */
  straight: number;
}

export class Player {
  position: Position;
  direction: Direction = Direction.south;
  /** Walk phase of the feet: 0 standing, 1 to footPhases while walking. */
  footPhase = 0;
  private step: Step | null = null;
  private lastStep: Step | null = null;
  /** Pixels of the step walked, in whole pixels (never back). */
  private walkedPixels = 0;
  private footTimer = -Infinity;
  private footResetAt = Infinity;
  private turnAfterWalk: Direction | null = null;
  private walkLockUntil = 0;
  private lastTurn = -Infinity;
  /** Held walk keys, newest first (game_walk smartWalkDirs), with when they went down. */
  private heldDirections: Direction[] = [];
  private pressedAt = new Map<Direction, number>();
  private lastWalkDirection: Direction | null = null;
  private nextWalkDirection: Direction | null = null;
  private nextWalkAt = Infinity;
  /** A walk key went down: walk on the next update, after the other keys of the frame (game_walk addWalkEvent). */
  private walkQueued = false;

  constructor(
    start: Position,
    private world: WalkWorld,
    /** Step speed (see playerSpeed). */
    public speed: number,
    /** Walk phases of the outfit's feet (0: no walk animation). */
    public footPhases = 0,
  ) {
    this.position = { ...start };
  }

  /** Puts the player on another tile at once (no step), facing as it did. */
  teleport(position: Position): void {
    this.position = { ...position };
    this.step = null;
    this.lastStep = null;
    this.walkedPixels = 0;
    this.footPhase = 0;
    this.nextWalkDirection = null;
    this.nextWalkAt = Infinity;
  }

  /** The step is being drawn (the walk animation runs). */
  get walking(): boolean {
    return this.step !== null;
  }

  /** Offset of the drawn player from its tile, in world pixels. */
  offset(now: number, spriteSize: number): [number, number] {
    const step = this.step;
    if (!step) return [0, 0];
    const progress = Math.min(Math.max((now - step.start) / (step.animationEnd - step.start), 0), 1);
    return walkOffset(step.direction, progress * spriteSize, spriteSize);
  }

  /** A walk key went down; it walks on the next update (none with a modifier held). */
  pressWalk(direction: Direction, now: number, modifiers: boolean): void {
    if (!this.pressedAt.has(direction)) {
      this.pressedAt.set(direction, now);
      this.heldDirections = [direction, ...this.heldDirections.filter((held) => held !== direction)];
    }
    if (!modifiers) this.walkQueued = true;
  }

  releaseWalk(direction: Direction): void {
    this.pressedAt.delete(direction);
    this.heldDirections = this.heldDirections.filter((held) => held !== direction);
  }

  /** Every walk key let go (the view lost focus). */
  releaseAll(): void {
    this.pressedAt.clear();
    this.heldDirections = [];
  }

  /** A turn key (Ctrl+arrow) went down; repeated while held. */
  turn(direction: Direction, now: number, repeated: boolean): void {
    if (this.step && this.direction === direction) return;
    this.nextWalkDirection = null;
    this.nextWalkAt = Infinity;
    if (this.lastTurn + (repeated ? TURN_DELAY_REPEATED : TURN_DELAY_FIRST) >= now) return;
    if (this.step) this.turnAfterWalk = direction;
    else this.direction = direction;
    this.lastTurn = now;
    this.lockWalk(now, TURN_LOCK);
  }

  releaseTurn(now: number): void {
    this.lockWalk(now, TURN_RELEASE_LOCK);
  }

  /** Advances walking to now; true while something moves (draw again). */
  update(now: number, spriteSize: number): boolean {
    const step = this.step;
    if (step) {
      const progress = Math.min(Math.max((now - step.start) / (step.animationEnd - step.start), 0), 1);
      this.walkedPixels = Math.max(this.walkedPixels, Math.floor(progress * spriteSize));
      this.updateFeet(now, step, spriteSize);
      if (this.walkedPixels >= spriteSize) this.endWalk(now);
    }
    const smart = this.smartDirection();
    if (this.walkQueued) {
      this.walkQueued = false;
      if (smart !== undefined) this.walk(smart, now);
    } else if (smart !== undefined && now - (this.pressedAt.get(this.heldDirections[0]) ?? now) >= AUTO_REPEAT_DELAY) {
      // Held keys repeat after the auto repeat delay.
      this.walk(smart, now);
    }
    if (now >= this.nextWalkAt) {
      const direction = this.nextWalkDirection!;
      this.nextWalkAt = Infinity;
      this.nextWalkDirection = null;
      this.walk(this.smartDirection() ?? direction, now);
    }
    if (!this.step && now >= this.footResetAt) {
      this.footPhase = 0;
      this.footResetAt = Infinity;
    }
    return this.step !== null || this.walkQueued || this.footResetAt !== Infinity || this.nextWalkAt !== Infinity || this.heldDirections.length > 0;
  }

  /** game_walk walk(): steps when it may, or keeps a new direction for when the step ends. */
  private walk(direction: Direction, now: number): void {
    if (now < this.walkLockUntil) {
      this.nextWalkDirection = null;
      return;
    }
    if (!this.canWalk(now)) {
      if (this.lastWalkDirection !== direction) this.nextWalkDirection = direction;
      return;
    }
    this.nextWalkDirection = null;
    this.lastWalkDirection = direction;
    const [dx, dy] = directionOffset(direction);
    const { x, y, z } = this.position;
    if (!this.world.isWalkable(x + dx, y + dy, z)) return;
    const straight = stepDuration(this.world.groundSpeed(x + dx, y + dy, z), this.speed);
    this.position = { x: x + dx, y: y + dy, z };
    this.direction = direction;
    this.step = {
      direction,
      start: now,
      animationEnd: now + straight,
      stepEnd: now + (isDiagonal(direction) ? straight * PLAYER_DIAGONAL_COST : straight),
      straight,
    };
    this.lastStep = this.step;
    this.walkedPixels = 0;
    this.turnAfterWalk = null;
    this.footResetAt = Infinity;
  }

  /** LocalPlayer::canWalk: the last step, diagonal wait included, is over. */
  private canWalk(now: number): boolean {
    const last = this.lastStep;
    return !last || now >= Math.max(last.stepEnd, last.animationEnd);
  }

  private endWalk(now: number): void {
    this.step = null;
    this.walkedPixels = 0;
    if (this.turnAfterWalk !== null) {
      this.direction = this.turnAfterWalk;
      this.turnAfterWalk = null;
    }
    this.footResetAt = now + FOOT_RESET_DELAY;
    if (this.nextWalkDirection !== null) this.nextWalkAt = now + NEXT_WALK_DELAY;
  }

  /** Creature::updateWalkAnimation. */
  private updateFeet(now: number, step: Step, spriteSize: number): void {
    if (!this.footPhases) return;
    if (now - step.start < step.stepEnd - step.start && this.walkedPixels >= spriteSize) {
      this.footPhase = 0;
      return;
    }
    if (now - this.footTimer >= footPhaseDelay(this.footPhases, step.straight)) {
      this.footPhase = this.footPhase >= this.footPhases ? 1 : this.footPhase + 1;
      this.footTimer = now;
    }
  }

  private lockWalk(now: number, millis: number): void {
    this.walkLockUntil = Math.max(this.walkLockUntil, now + millis);
  }

  /** The newest held direction, made diagonal with another held one (smartWalk). */
  private smartDirection(): Direction | undefined {
    const [newest, ...others] = this.heldDirections;
    if (newest === undefined) return undefined;
    for (const other of others) {
      const combined = combineDirections(newest, other);
      if (combined !== newest) return combined;
    }
    return newest;
  }
}
