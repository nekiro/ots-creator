import { describe, expect, it } from "vitest";
import { healthRect, infoLayout, litInfoColor, manaBar, nameLeft } from "./creatureInfo";

const ALL = { names: true, bars: true, manaBar: true };

describe("creature info", () => {
  it("puts the bar at the point and the name 12 pixels over it", () => {
    const layout = infoLayout(100, 50, 1, 800, 600, false, ALL);
    expect(layout.bar).toEqual({ x: 86, y: 50, width: 27, height: 4 });
    expect(layout.nameTop).toBe(38);
    expect(layout.nameCenter).toBe(100);
    expect(healthRect(layout.bar, 1, 100)).toEqual({ x: 87, y: 51, width: 25, height: 2 });
    expect(healthRect(layout.bar, 1, 40).width).toBe(10);
    expect(manaBar(layout.bar, 1)).toEqual({ x: 86, y: 53, width: 27, height: 4 });
  });

  it("keeps bar and name together a pixel inside the view", () => {
    const top = infoLayout(5, 4, 1, 800, 600, false, ALL);
    expect(top.bar.x).toBe(1);
    expect(top.bar.y).toBe(13);
    expect(top.nameTop).toBe(1);
    expect(infoLayout(5, -5, 1, 800, 600, false, { ...ALL, names: false }).bar.y).toBe(1);
    expect(infoLayout(799, 300, 1, 800, 600, false, ALL).bar.x).toBe(772);
    const bottom = infoLayout(400, 599, 1, 800, 600, false, ALL);
    expect(bottom.bar.y).toBe(583);
    expect(bottom.nameTop).toBe(571);
    // The player's mana bar keeps room under the health bar.
    expect(infoLayout(400, 599, 1, 800, 600, true, ALL).bar.y).toBe(577);
  });

  it("keeps the name a pixel inside the view", () => {
    expect(nameLeft(100, 40, 1, 800)).toBe(80);
    expect(nameLeft(10, 40, 1, 800)).toBe(1);
    expect(nameLeft(790, 40, 1, 800)).toBe(759);
  });

  it("scales with the screen pixel", () => {
    const layout = infoLayout(200, 100, 2, 800, 600, false, ALL);
    expect(layout.bar).toEqual({ x: 172, y: 100, width: 54, height: 8 });
    expect(layout.nameTop).toBe(76);
  });

  it("dims the colors by the light of the field", () => {
    expect(litInfoColor([0, 188, 0], 255)).toEqual([0, 188, 0]);
    expect(litInfoColor([192, 192, 192], 127)).toEqual([94, 94, 94]);
    expect(litInfoColor([0, 188, 0], 0)).toEqual([0, 0, 0]);
    // The player sees itself in at least a light of 2 levels.
    expect(litInfoColor([0, 188, 0], 0, 0)).toEqual([0, 73, 0]);
    expect(litInfoColor([0, 188, 0], 0, 9)).toEqual([0, 188, 0]);
  });
});
