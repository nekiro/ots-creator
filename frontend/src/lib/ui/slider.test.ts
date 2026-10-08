import { describe, expect, it } from "vitest";
import { thumbOffset, thumbWidth, valueAt } from "./slider";

describe("slider geometry", () => {
  it("sizes the thumb per step with a minimum", () => {
    expect(thumbWidth(80, 0, 3)).toBe(20);
    expect(thumbWidth(80, 0, 100)).toBe(12);
    expect(thumbWidth(80, 0, 0)).toBe(80);
  });

  it("places the thumb at the ends", () => {
    expect(thumbOffset(0, 80, 0, 3)).toBe(0);
    expect(thumbOffset(3, 80, 0, 3)).toBe(60);
    expect(thumbOffset(9, 80, 0, 3)).toBe(60);
  });

  it("maps pointer positions back to values", () => {
    expect(valueAt(0, 80, 0, 3)).toBe(0);
    expect(valueAt(80, 80, 0, 3)).toBe(3);
    expect(valueAt(40, 80, 0, 3)).toBe(2);
    expect(valueAt(10, 80, 2, 2)).toBe(2);
    for (let v = 0; v <= 3; v++) expect(valueAt(thumbOffset(v, 80, 0, 3) + 10, 80, 0, 3)).toBe(v);
  });
});
