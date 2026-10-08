import { describe, expect, it } from "vitest";
import { fillSlots, sheetCellPos, sheetGrid, spriteIndex, type Layout } from "./layout";

const g: Layout = { width: 1, height: 1, layers: 2, patternX: 4, patternY: 2, patternZ: 1, frames: 3 };

describe("sheet layout", () => {
  it("uses the ObjectBuilder sheet grid", () => {
    expect(sheetGrid(g)).toEqual({ columns: 8, rows: 6 });
  });

  it("maps every cell to a distinct texture", () => {
    const seen = new Set<number>();
    const { columns, rows } = sheetGrid(g);
    for (let r = 0; r < rows; r++)
      for (let c = 0; c < columns; c++) seen.add(spriteIndex(g, 0, 0, sheetCellPos(g, c, r)));
    expect(seen.size).toBe(columns * rows);
    expect(sheetCellPos(g, 3, 5)).toEqual({ layer: 1, x: 1, z: 0, y: 1, frame: 2 });
  });

  it("fills consecutive slots and stops at the end", () => {
    expect(fillSlots([0, 0, 0, 0], 1, [7, 8])).toEqual([0, 7, 8, 0]);
    expect(fillSlots([0, 0, 0], 2, [7, 8, 9])).toEqual([0, 0, 7]);
  });
});
