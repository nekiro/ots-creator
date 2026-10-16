import { describe, expect, it } from "vitest";
import { fitZoom, stepZoom, zoomLabel } from "./zoom";

describe("zoom steps", () => {
  it("steps between levels from any zoom", () => {
    expect(stepZoom(1, 1)).toBe(1.5);
    expect(stepZoom(1, -1)).toBe(0.75);
    expect(stepZoom(0.4, -1)).toBe(0.33);
    expect(stepZoom(0.4, 1)).toBe(0.5);
    expect(stepZoom(16, 1)).toBe(16);
    expect(stepZoom(0.1, -1)).toBe(0.1);
  });
  it("labels", () => {
    expect(zoomLabel(3)).toBe("3×");
    expect(zoomLabel(0.5)).toBe("50%");
  });
});

describe("fitZoom", () => {
  it("fits whole steps above 1x and fractions below", () => {
    expect(fitZoom(100, 50, 516, 516)).toBe(5);
    expect(fitZoom(1000, 100, 516, 516)).toBeCloseTo(0.5);
  });

  it("takes the largest whole zoom that fits", () => {
    expect(fitZoom(100, 50, 330, 330, 8, 30, 1)).toBe(3);
  });

  it("never goes below min or above the limit", () => {
    expect(fitZoom(100, 100, 50, 50, 4, 0, 1)).toBe(1);
    expect(fitZoom(32, 32, 10000, 10000, 4, 0, 1)).toBe(4);
  });

  it("keeps the margin free", () => {
    expect(fitZoom(100, 100, 200, 200, 8, 0, 1)).toBe(2);
    expect(fitZoom(100, 100, 200, 200, 8, 1, 1)).toBe(1);
  });
});
