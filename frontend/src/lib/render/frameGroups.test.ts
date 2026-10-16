import { describe, expect, it } from "vitest";
import type { FrameGroup } from "../api";
import { DEFAULT_DURATION, setAllDurations, setLayout, syncDurations } from "./frameGroups";

function frameGroup(fields: Partial<FrameGroup> = {}): FrameGroup {
  return {
    type: 0,
    width: 1,
    height: 1,
    exactSize: 32,
    layers: 1,
    patternX: 1,
    patternY: 1,
    patternZ: 1,
    frames: 1,
    mode: 0,
    loopCount: 0,
    startFrame: 0,
    durations: [],
    sprites: [1],
    ...fields,
  } as FrameGroup;
}

describe("syncDurations", () => {
  it("clears durations of a still group", () => {
    const group = frameGroup({ frames: 1, durations: [{ min: 1, max: 2 }] });
    syncDurations(group, 1);
    expect(group.durations).toEqual([]);
  });

  it("adds the category's default for new frames", () => {
    const group = frameGroup({ frames: 3, durations: [{ min: 7, max: 8 }] });
    syncDurations(group, 2);
    const fallback = DEFAULT_DURATION[2];
    expect(group.durations).toEqual([
      { min: 7, max: 8 },
      { min: fallback, max: fallback },
      { min: fallback, max: fallback },
    ]);
  });

  it("drops durations past the last frame and resets the start frame", () => {
    const group = frameGroup({ frames: 2, startFrame: 4, durations: [1, 2, 3, 4].map((ms) => ({ min: ms, max: ms })) });
    syncDurations(group, 1);
    expect(group.durations).toHaveLength(2);
    expect(group.startFrame).toBe(0);
  });

  it("uses 100 ms for unknown categories", () => {
    const group = frameGroup({ frames: 2 });
    syncDurations(group, 99);
    expect(group.durations[0]).toEqual({ min: 100, max: 100 });
  });
});

describe("setAllDurations", () => {
  it("sets every frame, max at least min", () => {
    const group = frameGroup({ frames: 2, durations: [{ min: 1, max: 1 }, { min: 2, max: 2 }] });
    setAllDurations(group, 50, 20);
    expect(group.durations).toEqual([
      { min: 50, max: 50 },
      { min: 50, max: 50 },
    ]);
  });
});

describe("setLayout", () => {
  it("sets the exact size alone", () => {
    const group = frameGroup();
    setLayout(group, "exactSize", 64, 1);
    expect(group.exactSize).toBe(64);
    expect(group.sprites).toEqual([1]);
  });

  it("resizes the sprite slots with the layout", () => {
    const group = frameGroup({ sprites: [5] });
    setLayout(group, "width", 2, 1);
    expect(group.width).toBe(2);
    expect(group.sprites).toHaveLength(2);
  });

  it("adds durations for added frames", () => {
    const group = frameGroup({ sprites: [5] });
    setLayout(group, "frames", 3, 1);
    expect(group.frames).toBe(3);
    expect(group.sprites).toHaveLength(3);
    expect(group.durations).toHaveLength(3);
  });
});
