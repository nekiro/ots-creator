// Port of the client/ObjectBuilder animator. Time is in milliseconds and is
// always passed in, so the animator is deterministic and testable.

export enum AnimationMode {
  Async = 0,
  Sync = 1,
}

export interface Duration {
  min: number;
  max: number;
}

export const FRAME_AUTOMATIC = -1;

export class Animator {
  readonly frames: number;
  readonly mode: AnimationMode;
  readonly loopCount: number;
  readonly startFrame: number;
  private readonly durations: number[];
  private current = 0;
  private remaining = 0;
  private lastTime = 0;
  private loop = 0;
  private forward = true;
  private complete = false;
  private readonly random: () => number;

  /**
   * @param loopCount 0 = infinite, n > 0 = play n times, < 0 = ping-pong
   * @param startFrame -1 = random start frame
   */
  constructor(
    mode: AnimationMode,
    loopCount: number,
    startFrame: number,
    durations: Duration[],
    now: number,
    random: () => number = Math.random,
  ) {
    if (durations.length === 0) throw new Error("animator needs at least one frame");
    if (startFrame < -1 || startFrame >= durations.length) throw new Error(`invalid start frame ${startFrame}`);
    this.frames = durations.length;
    this.mode = mode;
    this.loopCount = loopCount;
    this.startFrame = startFrame;
    this.random = random;
    // Duration of a frame is picked once, between min and max.
    this.durations = durations.map((d) => (d.max > d.min ? d.min + Math.floor(random() * (d.max - d.min + 1)) : d.min));
    this.reset(now);
  }

  get frame(): number {
    return this.current;
  }

  get isComplete(): boolean {
    return this.complete;
  }

  get totalDuration(): number {
    return this.durations.reduce((a, b) => a + b, 0);
  }

  reset(now: number): void {
    this.loop = 0;
    this.forward = true;
    this.complete = false;
    this.lastTime = now;
    if (this.mode === AnimationMode.Sync) {
      this.syncTo(now);
    } else {
      this.current = this.startFrame >= 0 ? this.startFrame : Math.floor(this.random() * this.frames);
      this.remaining = this.durations[this.current];
    }
  }

  /** Jump to a frame (used when stepping manually). */
  setFrame(frame: number, now: number): void {
    this.current = ((frame % this.frames) + this.frames) % this.frames;
    this.remaining = this.durations[this.current];
    this.lastTime = now;
    this.complete = false;
  }

  /** Synchronous animations are driven by absolute time so all instances match. */
  private syncTo(now: number): void {
    const total = this.totalDuration;
    let elapsed = total > 0 ? now % total : 0;
    for (let i = 0; i < this.frames; i++) {
      if (elapsed < this.durations[i]) {
        this.current = i;
        this.remaining = this.durations[i] - elapsed;
        return;
      }
      elapsed -= this.durations[i];
    }
    this.current = this.frames - 1;
    this.remaining = 0;
  }

  update(now: number): number {
    if (this.complete || this.frames < 2) {
      this.lastTime = now;
      return this.current;
    }
    let elapsed = now - this.lastTime;
    this.lastTime = now;
    // Advance as many frames as fit in the elapsed time.
    let guard = 0;
    while (elapsed >= this.remaining && !this.complete && guard++ < 10000) {
      elapsed -= this.remaining;
      const next = this.loopCount < 0 ? this.pingPongNext() : this.loopNext();
      if (next === this.current && this.complete) break;
      this.current = next;
      this.remaining = this.durations[next];
    }
    if (!this.complete) this.remaining -= elapsed;
    return this.current;
  }

  private loopNext(): number {
    const next = this.current + 1;
    if (next < this.frames) return next;
    if (this.loopCount === 0) return 0;
    if (this.loop < this.loopCount - 1) {
      this.loop++;
      return 0;
    }
    this.complete = true;
    return this.current;
  }

  private pingPongNext(): number {
    const step = this.forward ? 1 : -1;
    const next = this.current + step;
    if (next < 0 || next >= this.frames) {
      this.forward = !this.forward;
      return this.current - step;
    }
    return next;
  }
}

/**
 * Step duration the outfit preview walks at: a level 1 player (speed 220)
 * on grass (ground speed 150), rounded up to the 50 ms server beat like
 * the client does: 1000 * 150 / 220 = 682 -> 700.
 */
export const WALK_STEP_MS = 700;

/**
 * Delay between walking frames in the client (OTClient / voidcores
 * Creature::updateWalkAnimation): one walk cycle takes a bit longer than a
 * step, and outfits with more than 2 walking frames show a whole stride per
 * cycle, so they cycle 1.5x slower.
 */
export function footDelay(walkFrames: number, step = WALK_STEP_MS): number {
  const cycle = walkFrames > 2 ? Math.floor(((step + 20) * 3) / 2) : step + 20;
  return Math.max(Math.floor(cycle / walkFrames) + 10, 20);
}

/** What previewDurations needs to know about the thing and its client. */
export interface PreviewContext {
  outfit: boolean;
  /** Number of frame groups of the thing. */
  groups: number;
  /** The animate always flag of the thing. */
  animateAlways: boolean;
  /**
   * The client stores frame durations (improved animations); without them
   * the stored durations are made-up defaults.
   */
  improved: boolean;
}

const fixed = (frames: number, ms: number): Duration[] => Array.from({ length: frames }, () => ({ min: ms, max: ms }));

/**
 * Frame durations used to preview a frame group, timed like the client:
 * - outfit walking frames advance at footDelay, paced by the walk speed,
 *   not by stored durations (the official files hold a 300 ms placeholder);
 * - an outfit of an old client (one group, no improved animations) walks
 *   over frames 1..n-1, frame 0 is standing (it gets 0 ms so it is skipped),
 *   unless it animates always: then a whole cycle takes one second;
 * - everything else uses the stored durations, or 100 ms without them.
 */
export function previewDurations(g: { frames: number; durations: Duration[] | null; type: number }, ctx: PreviewContext): Duration[] {
  if (ctx.outfit && g.type === 1) return fixed(g.frames, footDelay(g.frames));
  if (ctx.outfit && ctx.groups === 1 && !ctx.improved) {
    if (ctx.animateAlways) return fixed(g.frames, Math.round(1000 / g.frames));
    if (g.frames > 2) return [{ min: 0, max: 0 }, ...fixed(g.frames - 1, footDelay(g.frames - 1))];
    return fixed(g.frames, footDelay(g.frames));
  }
  if (g.durations?.length === g.frames) return g.durations;
  return fixed(g.frames, 100);
}
