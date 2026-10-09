import { describe, expect, it } from "vitest";
import { fitZoom, stepZoom, zoomLabel } from "./pan.svelte";

describe("zoom steps", () => {
  it("steps between levels from any zoom", () => {
    expect(stepZoom(1, 1)).toBe(1.5);
    expect(stepZoom(1, -1)).toBe(0.75);
    expect(stepZoom(0.4, -1)).toBe(0.33);
    expect(stepZoom(0.4, 1)).toBe(0.5);
    expect(stepZoom(16, 1)).toBe(16);
    expect(stepZoom(0.1, -1)).toBe(0.1);
  });
  it("fits whole steps above 1x and fractions below", () => {
    expect(fitZoom(100, 50, 516, 516)).toBe(5);
    expect(fitZoom(1000, 100, 516, 516)).toBeCloseTo(0.5);
  });
  it("labels", () => {
    expect(zoomLabel(3)).toBe("3×");
    expect(zoomLabel(0.5)).toBe("50%");
  });
});
