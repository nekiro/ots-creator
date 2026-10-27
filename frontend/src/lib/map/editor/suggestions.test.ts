import { describe, expect, it } from "vitest";
import type { Suggestion } from "../data/api";
import { notePick, rerank, selectionOf, suggestionKey } from "./suggestions";

const suggestion = (item: number, score: number, brush?: string): Suggestion => ({ item, score, reason: "", brush, kind: brush ? "doodad" : undefined });

describe("suggestions", () => {
  it("keys brushes by brush and items by id", () => {
    expect(suggestionKey(suggestion(5, 1, "trees"))).toBe("doodad:trees");
    expect(suggestionKey(suggestion(5, 1))).toBe("item:5");
  });

  it("selects the brush of a suggestion, else the raw item", () => {
    expect(selectionOf(suggestion(5, 1, "trees"))).toMatchObject({ kind: "doodad", name: "trees", look: 5 });
    expect(selectionOf({ ...suggestion(6, 1, "stone wall"), kind: "decoration" })).toMatchObject({ kind: "wall", name: "stone wall" });
    expect(selectionOf(suggestion(7, 1))).toMatchObject({ kind: "raw", item: 7 });
  });

  it("raises picks and sinks the suggestions passed over", () => {
    const list = [suggestion(1, 1.0), suggestion(2, 0.9), suggestion(3, 0.8)];
    expect(rerank(list).map((entry) => entry.item)).toEqual([1, 2, 3]);
    notePick(list, list[2]);
    expect(rerank(list).map((entry) => entry.item)).toEqual([3, 1, 2]);
  });
});
