import { describe, expect, it } from "vitest";
import type { HouseInfo, MapInfo } from "./api";
import { map } from "./state.svelte";
import { houses, townName, towns, viewCenter } from "./world";

function openMap(fields: Partial<MapInfo>) {
  map.maps = [{ id: 9, ...fields } as MapInfo];
  map.activeId = 9;
}

describe("world", () => {
  it("lists towns and houses without holes", () => {
    openMap({
      towns: [{ id: 1, name: "Thais", temple: { x: 1, y: 1, z: 7 } }, null],
      houses: [{ id: 3, name: "Flat", townId: 1 } as HouseInfo],
    });
    expect(towns().map((town) => town.name)).toEqual(["Thais"]);
    expect(houses().map((house) => house.id)).toEqual([3]);
    expect(townName(1)).toBe("Thais");
    expect(townName(2)).toBe("No town");
  });

  it("finds the tile at the center of the view", () => {
    openMap({});
    map.cams[9] = { x: 100.5 * 32, y: 200.5 * 32, zoom: 1, floor: 7 };
    expect(viewCenter()).toEqual({ x: 100, y: 200, z: 7 });
    map.cams[9] = { x: -50, y: 10, zoom: 1, floor: 7 };
    expect(viewCenter()).toEqual({ x: 0, y: 0, z: 7 });
  });
});
