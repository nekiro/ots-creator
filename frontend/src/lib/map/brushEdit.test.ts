import { describe, expect, it } from "vitest";
import type { Brush, PaletteTileset } from "./api";
import { MAIN_WALLS, MORE_WALLS, chances, collectionsContaining, ensureBrushShape, firstItem, missingForSave, normalizeBrushForSave, stampOf } from "./brushEdit";

const brushOf = (fields: Partial<Brush>): Brush => ({ name: "test", type: "ground", ...fields }) as Brush;

describe("chances", () => {
  it("drops holes", () => {
    expect(chances([{ id: 1, chance: 5 }, null, { id: 2, chance: 1 }])).toEqual([
      { id: 1, chance: 5 },
      { id: 2, chance: 1 },
    ]);
    expect(chances(null)).toEqual([]);
  });
});

describe("firstItem", () => {
  it("takes the first item with an id", () => {
    expect(firstItem(brushOf({ items: [{ id: 0, chance: 1 }, { id: 7, chance: 1 }] }))).toBe(7);
  });

  it("looks into walls, parts and doodad items", () => {
    expect(firstItem(brushOf({ walls: { horizontal: { items: [{ id: 3, chance: 1 }], doors: [] } } }))).toBe(3);
    expect(firstItem(brushOf({ parts: { c: [{ id: 4, chance: 1 }] } }))).toBe(4);
    expect(firstItem(brushOf({ alternatives: [{ singles: [{ chance: 1, item: { id: 5 } }], composites: [] }] } as Partial<Brush>))).toBe(5);
  });

  it("is 0 without items", () => {
    expect(firstItem(brushOf({}))).toBe(0);
  });
});

describe("missingForSave", () => {
  it("needs a name", () => {
    expect(missingForSave(brushOf({ name: "  " }), false)).toBe("Give the brush a name.");
  });

  it("needs only a name for a doodad from a selection", () => {
    expect(missingForSave(brushOf({ type: "doodad" }), true)).toBe("");
  });

  it("needs a ground item", () => {
    expect(missingForSave(brushOf({ items: [] }), false)).toBe("Add at least one ground item.");
    expect(missingForSave(brushOf({ items: [{ id: 1, chance: 1 }] }), false)).toBe("");
  });

  it("needs horizontal and vertical wall pieces", () => {
    const horizontal = { items: [{ id: 1, chance: 1 }], doors: [] };
    expect(missingForSave(brushOf({ type: "wall", walls: { horizontal } }), false)).toBe("Give the horizontal and vertical pieces.");
    expect(missingForSave(brushOf({ type: "decoration", walls: { horizontal, vertical: horizontal } }), false)).toBe("");
  });

  it("needs a doodad item or group", () => {
    expect(missingForSave(brushOf({ type: "doodad", alternatives: [{ singles: [], composites: [] }] }), false)).toBe("Add at least one item.");
    expect(missingForSave(brushOf({ type: "doodad", alternatives: [{ singles: [], composites: [{ chance: 1, tiles: [] }] }] } as Partial<Brush>), false)).toBe("");
  });

  it("needs the carpet center and the single table", () => {
    expect(missingForSave(brushOf({ type: "carpet" }), false)).toBe("Pick the center piece.");
    expect(missingForSave(brushOf({ type: "table", parts: { alone: [{ id: 1, chance: 1 }] } }), false)).toBe("");
  });
});

describe("stampOf", () => {
  const composite = { chance: 1, tiles: [] };

  it("finds the one group of a doodad", () => {
    expect(stampOf(brushOf({ type: "doodad", alternatives: [{ singles: [], composites: [composite] }] } as Partial<Brush>))).toBe(composite);
  });

  it("is null for doodads with single items, several groups or variations", () => {
    expect(stampOf(brushOf({ type: "doodad", alternatives: [{ singles: [{ chance: 1, item: { id: 1 } }], composites: [composite] }] } as Partial<Brush>))).toBeNull();
    expect(stampOf(brushOf({ type: "doodad", alternatives: [{ singles: [], composites: [composite, composite] }] } as Partial<Brush>))).toBeNull();
    expect(stampOf(brushOf({ type: "ground" }))).toBeNull();
  });
});

describe("normalizeBrushForSave", () => {
  it("sets the look to the first item", () => {
    expect(normalizeBrushForSave(brushOf({ items: [{ id: 9, chance: 1 }] })).look).toBe(9);
    expect(normalizeBrushForSave(brushOf({ look: 2, items: [{ id: 9, chance: 1 }] })).look).toBe(2);
  });

  it("drops empty wall pieces", () => {
    const saved = normalizeBrushForSave(
      brushOf({
        type: "wall",
        walls: {
          horizontal: { items: [{ id: 1, chance: 1 }], doors: [] },
          pole: { items: [], doors: [] },
          corner: { items: [], doors: [{ id: 5, type: "normal", open: false }] },
        },
      } as Partial<Brush>),
    );
    expect(Object.keys(saved.walls ?? {})).toEqual(["horizontal", "corner"]);
  });

  it("drops parts of other types", () => {
    const saved = normalizeBrushForSave(brushOf({ type: "ground", items: [{ id: 1, chance: 1 }], walls: { pole: { items: [], doors: [] } }, alternatives: [] }));
    expect(saved.walls).toEqual({});
    expect(saved.doodad).toBeNull();
    expect(saved.alternatives).toEqual([]);
  });
});

describe("ensureBrushShape", () => {
  it("adds every wall piece", () => {
    const brush = brushOf({ type: "wall", walls: { horizontal: { items: [{ id: 1, chance: 1 }], doors: [] } } });
    ensureBrushShape(brush);
    expect(Object.keys(brush.walls ?? {}).sort()).toEqual([...MAIN_WALLS, ...MORE_WALLS].sort());
    expect(brush.walls?.horizontal?.items).toEqual([{ id: 1, chance: 1 }]);
  });

  it("keeps complete walls as they are", () => {
    const walls = Object.fromEntries([...MAIN_WALLS, ...MORE_WALLS].map((align) => [align, { items: [], doors: [] }]));
    const brush = brushOf({ type: "wall", walls });
    ensureBrushShape(brush);
    expect(brush.walls).toBe(walls);
  });

  it("gives doodads options and a variation", () => {
    const brush = brushOf({ type: "doodad" });
    ensureBrushShape(brush);
    expect(brush.doodad).toEqual({ draggable: true });
    expect(brush.alternatives).toEqual([{ singles: [], composites: [] }]);
  });
});

describe("collectionsContaining", () => {
  it("lists hand-made collections with the brush", () => {
    const tilesets = [
      { name: "Nature", auto: false, terrain: [{ name: "grass" }], doodads: [] },
      { name: "Trees", auto: false, terrain: [], doodads: [{ name: "grass" }] },
      { name: "Auto", auto: true, terrain: [{ name: "grass" }] },
      { name: "Other", auto: false, terrain: [{ name: "sand" }] },
      null,
    ] as unknown as PaletteTileset[];
    expect(collectionsContaining(tilesets, "grass")).toEqual(["Nature", "Trees"]);
    expect(collectionsContaining(null, "grass")).toEqual([]);
  });
});
