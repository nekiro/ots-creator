import { describe, expect, it } from "vitest";
import { Animator, AnimationMode, footDelay, previewDurations } from "./animator";
import { clampPos, resizeSprites, spriteIndex, textureSlots, totalSprites, type Layout } from "./layout";
import { colorize, hsiToRgb, PALETTE_SIZE } from "./outfit";

const d = (ms: number) => ({ min: ms, max: ms });

describe("Animator", () => {
  it("loops forever with loopCount 0", () => {
    const a = new Animator(AnimationMode.Async, 0, 0, [d(100), d(100), d(100)], 0);
    expect(a.update(99)).toBe(0);
    expect(a.update(100)).toBe(1);
    expect(a.update(250)).toBe(2);
    expect(a.update(300)).toBe(0);
    expect(a.isComplete).toBe(false);
  });

  it("skips several frames when much time passed", () => {
    const a = new Animator(AnimationMode.Async, 0, 0, [d(10), d(10), d(10), d(10)], 0);
    expect(a.update(35)).toBe(3);
  });

  it("stops after loopCount loops", () => {
    const a = new Animator(AnimationMode.Async, 2, 0, [d(10), d(10)], 0);
    a.update(10); // 1
    a.update(20); // 0 (second loop)
    a.update(30); // 1
    expect(a.update(40)).toBe(1);
    expect(a.isComplete).toBe(true);
    expect(a.update(1000)).toBe(1);
  });

  it("ping-pongs with negative loopCount", () => {
    const a = new Animator(AnimationMode.Async, -1, 0, [d(10), d(10), d(10)], 0);
    const seq = [10, 20, 30, 40, 50].map((t) => a.update(t));
    expect(seq).toEqual([1, 2, 1, 0, 1]);
  });

  it("sync mode depends only on absolute time", () => {
    const a = new Animator(AnimationMode.Sync, 0, 0, [d(100), d(200)], 1150);
    const b = new Animator(AnimationMode.Sync, 0, 0, [d(100), d(200)], 1150);
    expect(a.frame).toBe(b.frame);
    expect(a.frame).toBe(1); // 1150 % 300 = 250 -> frame 1
  });

  it("random start frame uses the random source", () => {
    const a = new Animator(AnimationMode.Async, 0, -1, [d(1), d(1), d(1), d(1)], 0, () => 0.6);
    expect(a.frame).toBe(2);
  });

  it("picks durations between min and max", () => {
    const a = new Animator(AnimationMode.Async, 0, 0, [{ min: 100, max: 200 }], 0, () => 0.5);
    expect(a.totalDuration).toBe(150);
  });

  it("validates start frame", () => {
    expect(() => new Animator(AnimationMode.Async, 0, 5, [d(1)], 0)).toThrow();
  });

  it("setFrame wraps", () => {
    const a = new Animator(AnimationMode.Async, 0, 0, [d(10), d(10), d(10)], 0);
    a.setFrame(-1, 0);
    expect(a.frame).toBe(2);
  });
});

const L: Layout = { width: 2, height: 2, layers: 2, patternX: 4, patternY: 3, patternZ: 2, frames: 3 };

describe("layout", () => {
  it("enumerates slots in client order", () => {
    let next = 0;
    for (let f = 0; f < L.frames; f++)
      for (let z = 0; z < L.patternZ; z++)
        for (let y = 0; y < L.patternY; y++)
          for (let x = 0; x < L.patternX; x++)
            for (let l = 0; l < L.layers; l++)
              for (let h = 0; h < L.height; h++)
                for (let w = 0; w < L.width; w++) expect(spriteIndex(L, w, h, { layer: l, x, y, z, frame: f })).toBe(next++);
    expect(next).toBe(totalSprites(L));
  });

  it("puts tile (0,0) bottom-right", () => {
    const slots = textureSlots(L, { layer: 0, x: 0, y: 0, z: 0, frame: 0 }, 32);
    expect(slots[0]).toEqual({ slot: 0, x: 32, y: 32 });
    expect(slots[3]).toEqual({ slot: 3, x: 0, y: 0 });
  });

  it("clamps positions", () => {
    expect(clampPos(L, { layer: 9, x: -1, y: 1, z: 5, frame: 3 })).toEqual({ layer: 1, x: 0, y: 1, z: 1, frame: 2 });
  });

  it("resize keeps sprites", () => {
    const sprites = Array.from({ length: totalSprites(L) }, (_, i) => i + 1);
    const next = { ...L, patternY: 1, frames: 2 };
    const out = resizeSprites(L, sprites, next);
    expect(out.length).toBe(totalSprites(next));
    const p = { layer: 1, x: 3, y: 0, z: 1, frame: 1 };
    expect(out[spriteIndex(next, 1, 0, p)]).toBe(sprites[spriteIndex(L, 1, 0, p)]);
  });
});

describe("outfit colors", () => {
  it("has 133 colors with white and black ends", () => {
    expect(PALETTE_SIZE).toBe(133);
    expect(hsiToRgb(0)).toBe(0xffffff);
    expect(hsiToRgb(4 * 19 + 0)).toBe(hsiToRgb(76)); // grey column
    expect(hsiToRgb(999)).toBe(0xffffff); // out of range -> 0
  });

  it("pure hue at full saturation is red", () => {
    // color 76 = row 4 (S=1, I=1), column 0 -> grey; column 1 is first hue
    expect(hsiToRgb(4 * 19 + 18) >> 16).toBe(0xff);
  });

  it("multiplies only template areas", () => {
    const base = new Uint8ClampedArray([200, 200, 200, 255, 200, 200, 200, 255]);
    const tpl = new Uint8ClampedArray([255, 0, 0, 255, 0, 0, 0, 0]); // body, untouched
    colorize(base, tpl, { head: 0, body: 0, legs: 0, feet: 0 }); // white multiplier
    expect(Array.from(base)).toEqual([200, 200, 200, 255, 200, 200, 200, 255]);
    const tpl2 = new Uint8ClampedArray([255, 0, 0, 255, 0, 0, 0, 0]);
    colorize(base, tpl2, { head: 0, body: 132, legs: 0, feet: 0 }); // last color, darkest row
    expect(base[0]).toBeLessThan(200);
    expect(base[4]).toBe(200);
  });
});

describe("previewDurations", () => {
  const durations = [
    { min: 300, max: 300 },
    { min: 300, max: 300 },
    { min: 300, max: 300 },
  ];
  const outfit = { outfit: true, groups: 2, animateAlways: false, improved: true };
  const ms = (d: { min: number }[]) => d.map((x) => x.min);

  it("paces walking like the client", () => {
    // 2 frames: (700 + 20) / 2 + 10; 8 frames: (720 * 1.5) / 8 + 10.
    expect(footDelay(2)).toBe(370);
    expect(footDelay(8)).toBe(145);
    expect(ms(previewDurations({ frames: 3, durations, type: 1 }, outfit))).toEqual([370, 370, 370]);
  });
  it("walks old single group outfits over frames 1..n-1", () => {
    const old = { ...outfit, groups: 1, improved: false };
    expect(ms(previewDurations({ frames: 3, durations, type: 0 }, old))).toEqual([0, footDelay(2), footDelay(2)]);
    expect(ms(previewDurations({ frames: 2, durations: durations.slice(1), type: 0 }, old))).toEqual([370, 370]);
    expect(ms(previewDurations({ frames: 4, durations: null, type: 0 }, { ...old, animateAlways: true }))).toEqual([250, 250, 250, 250]);
  });
  it("keeps stored durations elsewhere", () => {
    expect(previewDurations({ frames: 3, durations, type: 0 }, outfit)).toBe(durations);
    expect(previewDurations({ frames: 3, durations, type: 0 }, { ...outfit, groups: 1 })).toBe(durations);
    expect(previewDurations({ frames: 3, durations, type: 1 }, { ...outfit, outfit: false })).toBe(durations);
    expect(ms(previewDurations({ frames: 2, durations: null, type: 0 }, { ...outfit, outfit: false }))).toEqual([100, 100]);
  });
});
