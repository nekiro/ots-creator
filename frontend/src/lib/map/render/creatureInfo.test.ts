import { describe, expect, it } from "vitest";
import { healthRect, infoLayout, manaBar } from "./creatureInfo";

describe("creature info", () => {
  it("puts the bar at the point and the name 12 pixels over it", () => {
    const layout = infoLayout(100, 50, 1, 800, 600, false);
    expect(layout.bar).toEqual({ x: 86, y: 50, width: 27, height: 4 });
    expect(layout.nameTop).toBe(38);
    expect(healthRect(layout.bar, 1, 100)).toEqual({ x: 87, y: 51, width: 25, height: 2 });
    expect(healthRect(layout.bar, 1, 40).width).toBe(10);
    expect(manaBar(layout.bar, 1)).toEqual({ x: 86, y: 53, width: 27, height: 4 });
  });

  it("keeps bar and name inside the view, apart when the top edge pushes the name", () => {
    const top = infoLayout(5, 4, 1, 800, 600, false);
    expect(top.bar.x).toBe(0);
    expect(top.nameTop).toBe(0);
    expect(top.bar.y).toBe(12);
    expect(infoLayout(400, 4, 1, 800, 600, true).bar.y).toBe(24);
    const bottom = infoLayout(400, 599, 1, 800, 600, false);
    expect(bottom.bar.y).toBe(596);
    expect(bottom.nameTop).toBe(584);
  });

  it("scales with the screen pixel", () => {
    const layout = infoLayout(200, 100, 2, 800, 600, false);
    expect(layout.bar).toEqual({ x: 173, y: 100, width: 54, height: 8 });
    expect(layout.nameTop).toBe(76);
  });
});
