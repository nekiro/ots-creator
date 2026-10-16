import { describe, expect, it } from "vitest";
import { FLUID_OPTIONS, FLUIDS, parseCount } from "./itemAttrs";

describe("parseCount", () => {
  it("keeps whole numbers in range", () => {
    expect(parseCount("12")).toBe(12);
    expect(parseCount("7.9")).toBe(7);
    expect(parseCount("70000")).toBe(65535);
  });

  it("turns invalid and negative input into 0", () => {
    expect(parseCount("abc")).toBe(0);
    expect(parseCount("-3")).toBe(0);
    expect(parseCount("Infinity")).toBe(0);
  });
});

describe("FLUID_OPTIONS", () => {
  it("lists every fluid as a menu option", () => {
    expect(FLUID_OPTIONS).toHaveLength(FLUIDS.length);
    expect(FLUID_OPTIONS[1]).toEqual({ value: 1, label: "Water" });
  });
});
