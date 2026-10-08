import { describe, expect, it } from "vitest";
import { splitZoom, zoomFor } from "./pixelscale";

describe("zoomFor", () => {
  it("snaps fractional system scales to whole screen pixels", () => {
    expect(1.5 * zoomFor(1.5, "auto")).toBe(2);
    expect(1.25 * zoomFor(1.25, "auto")).toBe(1);
    expect(1.75 * zoomFor(1.75, "auto")).toBe(2);
    expect(zoomFor(1, "auto")).toBe(1);
    expect(zoomFor(2, "auto")).toBe(1);
  });

  it("forces an explicit scale", () => {
    expect(1.5 * zoomFor(1.5, 1)).toBeCloseTo(1);
    expect(1.25 * zoomFor(1.25, 3)).toBeCloseTo(3);
  });

  it("system mode and bad input keep zoom 1", () => {
    expect(zoomFor(1.5, "system")).toBe(1);
    expect(zoomFor(0, "auto")).toBe(1);
    expect(zoomFor(NaN, 2)).toBe(1);
  });
});

describe("splitZoom", () => {
  it("uses the webview for zooming in and CSS zoom for shrinking", () => {
    expect(splitZoom(1.5)).toEqual({ webview: 1.5, css: 1 });
    expect(splitZoom(0.5)).toEqual({ webview: 1, css: 0.5 });
    expect(splitZoom(1)).toEqual({ webview: 1, css: 1 });
  });
});
