import { describe, expect, it } from "vitest";
import { arrowAt, needsFakeButtons } from "./scrollbuttons";

// 100x200 client area with a 1px border and both 12px scrollbars.
const el = {
  clientLeft: 1,
  clientTop: 1,
  clientWidth: 100,
  clientHeight: 200,
  scrollWidth: 300,
  scrollHeight: 900,
};

describe("scrollbar arrows", () => {
  it("only fakes buttons on WebKit", () => {
    expect(
      needsFakeButtons("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)"),
    ).toBe(true);
    expect(
      needsFakeButtons(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0",
      ),
    ).toBe(false);
  });

  it("maps presses at the scrollbar ends to steps", () => {
    expect(arrowAt(el, 105, 5)).toEqual({ dx: 0, dy: -40 });
    expect(arrowAt(el, 105, 195)).toEqual({ dx: 0, dy: 40 });
    expect(arrowAt(el, 5, 205)).toEqual({ dx: -40, dy: 0 });
    expect(arrowAt(el, 95, 205)).toEqual({ dx: 40, dy: 0 });
  });

  it("ignores the track, the content and the corner", () => {
    expect(arrowAt(el, 105, 100)).toBeNull();
    expect(arrowAt(el, 50, 205)).toBeNull();
    expect(arrowAt(el, 50, 5)).toBeNull();
    expect(arrowAt(el, 105, 205)).toBeNull();
  });

  it("ignores scrollbars with nothing to scroll", () => {
    expect(arrowAt({ ...el, scrollHeight: 200 }, 105, 5)).toBeNull();
    expect(arrowAt({ ...el, scrollWidth: 100 }, 5, 205)).toBeNull();
  });
});
