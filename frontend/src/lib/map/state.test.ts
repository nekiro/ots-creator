import { describe, expect, it } from "vitest";
import { isBrushActive, map, selectBrush, setCam, warningCount } from "./state.svelte";

describe("warningCount", () => {
  it("counts listed warnings and the ones summed up", () => {
    expect(warningCount(["tile 1", "tile 2", "40 more not listed: unknown item"])).toBe(42);
    expect(warningCount(undefined)).toBe(0);
  });
});

describe("setCam", () => {
  it("changes only the given camera values", () => {
    setCam(77, { x: 5 });
    expect(map.cams[77]).toEqual({ x: 5, y: 0, zoom: 1, floor: 7 });
    setCam(77, { zoom: 2 });
    expect(map.cams[77]).toEqual({ x: 5, y: 0, zoom: 2, floor: 7 });
  });
});

describe("selectBrush", () => {
  it("switches to the brush tool and remembers recent brushes once", () => {
    const grass = { kind: "ground", name: "grass", label: "Grass" };
    selectBrush(grass);
    selectBrush({ kind: "ground", name: "sand", label: "Sand" });
    selectBrush({ ...grass });
    expect(map.tool).toBe("brush");
    expect(isBrushActive(grass)).toBe(true);
    expect(map.recentBrushes.filter((brush) => brush.name === "grass")).toHaveLength(1);
    expect(map.recentBrushes[0].name).toBe("grass");
  });

  it("does not remember raw brushes without an item", () => {
    const before = map.recentBrushes.length;
    selectBrush({ kind: "raw", label: "nothing" });
    expect(map.recentBrushes).toHaveLength(before);
  });
});
