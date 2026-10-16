import { describe, expect, it } from "vitest";
import type { MapInfo } from "../data/api";
import { MARKER_COLORS } from "../constants";
import { houseLabel, markersOf, shortLabel } from "./markers";

describe("houseLabel", () => {
  it("keeps the part after the last comma", () => {
    expect(houseLabel("Paupers Palace, Flat 07")).toBe("Flat 07");
  });

  it("keeps names without a comma", () => {
    expect(houseLabel("Castle")).toBe("Castle");
    expect(houseLabel("Odd,")).toBe("Odd,");
  });
});

describe("shortLabel", () => {
  it("keeps short names", () => {
    expect(shortLabel("Thais")).toBe("Thais");
    expect(shortLabel("a".repeat(18))).toBe("a".repeat(18));
  });

  it("cuts long names with an ellipsis", () => {
    expect(shortLabel("Abcdefghijklmnop qrstuv")).toBe("Abcdefghijklmnop…");
    expect(shortLabel("a".repeat(19))).toBe("a".repeat(17) + "…");
  });
});

describe("markersOf", () => {
  it("is empty without a map", () => {
    expect(markersOf(null)).toEqual([]);
  });

  it("marks waypoints, house exits and temples", () => {
    const info = {
      waypoints: [{ name: "depot", pos: { x: 1, y: 2, z: 7 } }, null],
      houses: [
        { name: "Street, Flat 1", exit: { x: 3, y: 4, z: 7 } },
        { name: "No exit", exit: { x: 0, y: 0, z: 0 } },
      ],
      towns: [{ name: "Thais", temple: { x: 5, y: 6, z: 7 } }, { name: "Nowhere", temple: { x: 0, y: 0, z: 7 } }],
    } as unknown as MapInfo;
    expect(markersOf(info)).toEqual([
      { x: 1, y: 2, z: 7, label: "depot", full: "depot", color: MARKER_COLORS.waypoint },
      { x: 3, y: 4, z: 7, label: "Flat 1", full: "Street, Flat 1", color: MARKER_COLORS.houseExit },
      { x: 5, y: 6, z: 7, label: "Thais", full: "Thais", color: MARKER_COLORS.temple },
    ]);
  });
});
