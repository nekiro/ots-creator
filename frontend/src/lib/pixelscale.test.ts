import { describe, expect, it } from "vitest";
import { fittingScale, splitZoom, zoomFor } from "./pixelscale";

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

describe("fittingScale", () => {
	const minimum = { width: 1034, height: 600 };

	it("gives the largest whole scale whose layout fits the screen", () => {
		expect(fittingScale({ width: 1920, height: 1040 }, minimum)).toBe(1);
		expect(fittingScale({ width: 2560, height: 1400 }, minimum)).toBe(2);
		expect(fittingScale({ width: 3840, height: 2100 }, minimum)).toBe(3);
	});

	it("never goes below 1 and is unbounded without a screen", () => {
		expect(fittingScale({ width: 800, height: 500 }, minimum)).toBe(1);
		expect(fittingScale(null, minimum)).toBe(Infinity);
	});

	it("lowers the auto scale that would cut the window off, keeps a chosen one", () => {
		const fullHd = fittingScale({ width: 1920, height: 1040 }, minimum);
		expect(1.5 * zoomFor(1.5, "auto", fullHd)).toBeCloseTo(1);
		expect(1.5 * zoomFor(1.5, 2, fullHd)).toBeCloseTo(2);
		expect(zoomFor(1.5, "system", fullHd)).toBe(1);
	});
});

describe("splitZoom", () => {
	it("uses the webview for zooming in and CSS zoom for shrinking", () => {
		expect(splitZoom(1.5)).toEqual({ webview: 1.5, css: 1 });
		expect(splitZoom(0.5)).toEqual({ webview: 1, css: 0.5 });
		expect(splitZoom(1)).toEqual({ webview: 1, css: 1 });
	});
});
