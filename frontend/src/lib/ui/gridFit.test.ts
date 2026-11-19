import { describe, expect, it } from "vitest";
import { fitCells } from "./gridFit";

describe("fitCells", () => {
	it("grows the cells to fill the width", () => {
		// Four cells of 138 fit in 570 and grow to 140; the 4 pixels left are less than one per column.
		expect(fitCells(570, 138, 2)).toEqual({ columns: 4, size: 140 });
		expect(fitCells(560, 138, 2)).toEqual({ columns: 4, size: 138 });
		expect(fitCells(500, 138, 2)).toEqual({ columns: 3, size: 164 });
	});
	it("keeps the parity of the smallest size", () => {
		const { columns, size } = fitCells(501, 36, 2);
		expect((size - 36) % 2).toBe(0);
		expect(columns * size + (columns - 1) * 2).toBeLessThanOrEqual(501);
	});
	it("never shrinks below the smallest size", () => {
		expect(fitCells(0, 36, 2)).toEqual({ columns: 1, size: 36 });
		expect(fitCells(20, 36, 2)).toEqual({ columns: 1, size: 36 });
	});
});
