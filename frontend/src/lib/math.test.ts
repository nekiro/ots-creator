import { describe, expect, it } from "vitest";
import { clamp } from "./math";

describe("clamp", () => {
  it("keeps values inside the range", () => {
    expect(clamp(5, 0, 10)).toBe(5);
    expect(clamp(-1, 0, 10)).toBe(0);
    expect(clamp(11, 0, 10)).toBe(10);
  });

  it("prefers max when the range is empty", () => {
    expect(clamp(3, 0, -1)).toBe(-1);
  });
});
