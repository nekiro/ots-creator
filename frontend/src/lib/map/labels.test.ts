import { describe, expect, it } from "vitest";
import { boxesOverlap, drawLabels } from "./labels";
import type { Label } from "./scene";

describe("boxesOverlap", () => {
  it("finds overlapping boxes", () => {
    expect(boxesOverlap([0, 0, 10, 10], [5, 5, 15, 15])).toBe(true);
    expect(boxesOverlap([0, 0, 10, 10], [2, 2, 3, 3])).toBe(true);
  });

  it("does not count touching edges", () => {
    expect(boxesOverlap([0, 0, 10, 10], [10, 0, 20, 10])).toBe(false);
    expect(boxesOverlap([0, 0, 10, 10], [0, 11, 10, 20])).toBe(false);
  });
});

/** A 2D context that logs filled texts; every glyph is 6 px wide. */
function fakeContext() {
  const filled: { text: string; x: number; y: number; color: string }[] = [];
  const ctx = {
    fillStyle: "",
    font: "",
    textAlign: "",
    textBaseline: "",
    lineWidth: 0,
    lineJoin: "",
    strokeStyle: "",
    clearRect() {},
    measureText: (text: string) => ({ width: text.length * 6 }),
    strokeText() {},
    fillText(text: string, x: number, y: number) {
      filled.push({ text, x, y, color: ctx.fillStyle });
    },
  };
  return { ctx: ctx as unknown as CanvasRenderingContext2D, filled };
}

describe("drawLabels", () => {
  it("draws names over an eight copy outline", () => {
    const { ctx, filled } = fakeContext();
    drawLabels(ctx, 100, 100, [{ x: 10.4, y: 20.6, text: "Rat", color: "#0f0" }], 1);
    expect(filled).toHaveLength(9);
    expect(filled.slice(0, 8).every((call) => call.color === "#000")).toBe(true);
    expect(filled[8]).toEqual({ text: "Rat", x: 10, y: 21, color: "#0f0" });
  });

  it("leaves out culled texts that would cover others", () => {
    const { ctx, filled } = fakeContext();
    const labels: Label[] = [
      { x: 50, y: 50, text: "first", color: "#fff", small: true, cull: true },
      { x: 52, y: 51, text: "second", color: "#fff", small: true, cull: true },
      { x: 50, y: 90, text: "third", color: "#fff", small: true, cull: true },
    ];
    drawLabels(ctx, 100, 100, labels, 1);
    expect(filled.map((call) => call.text)).toEqual(["first", "third"]);
  });

  it("draws labels under the mouse last", () => {
    const { ctx, filled } = fakeContext();
    const labels: Label[] = [
      { x: 0, y: 0, text: "top", color: "#fff", top: true },
      { x: 0, y: 50, text: "other", color: "#fff" },
    ];
    drawLabels(ctx, 100, 100, labels, 1);
    expect(filled.filter((call) => call.color === "#fff").map((call) => call.text)).toEqual(["other", "top"]);
  });
});
