import { describe, expect, it } from "vitest";
import { fitZoom } from "./zoom";

describe("fitZoom", () => {
  it("takes the largest whole zoom that fits", () => {
    expect(fitZoom(330, 330, 100, 50, 30, 8)).toBe(3);
  });

  it("never goes below 1 or above the limit", () => {
    expect(fitZoom(50, 50, 100, 100, 0, 4)).toBe(1);
    expect(fitZoom(10000, 10000, 32, 32, 0, 4)).toBe(4);
  });

  it("keeps the margin free", () => {
    expect(fitZoom(200, 200, 100, 100, 0, 8)).toBe(2);
    expect(fitZoom(200, 200, 100, 100, 1, 8)).toBe(1);
  });
});
