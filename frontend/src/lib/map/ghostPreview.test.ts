import { describe, expect, it } from "vitest";
import type { Clip, CreatureType, DoodadBuffer } from "./api";
import { MAX_RECT_PREVIEW_TILES, PREVIEW_COLORS } from "./constants";
import { computePreview, creatureGhost, lookOf, type MapDrag, type PreviewInput } from "./ghostPreview";
import type { BrushSel } from "./state.svelte";

const at = (x: number, y: number, z = 7) => ({ x, y, z });
const ground: BrushSel = { kind: "ground", item: 100, label: "grass" };

function input(change: Partial<PreviewInput> = {}): PreviewInput {
  return {
    drag: null,
    hover: at(10, 10),
    floor: 7,
    tool: "brush",
    brush: ground,
    erase: false,
    pasting: false,
    clip: null,
    doodad: null,
    size: 0,
    shape: "square",
    fill: false,
    creatures: [],
    direction: 2,
    ...change,
  };
}

const rectDrag = (start: ReturnType<typeof at>, cur: ReturnType<typeof at>, erase = false): MapDrag => ({ kind: "rect", start, cur, erase, alt: false });

describe("lookOf", () => {
  it("prefers the look over the item", () => {
    expect(lookOf({ kind: "raw", item: 5, look: 6, label: "" })).toBe(6);
    expect(lookOf({ kind: "raw", item: 5, label: "" })).toBe(5);
    expect(lookOf({ kind: "raw", label: "" })).toBe(0);
  });
});

describe("creatureGhost", () => {
  it("dresses the creature of the brush", () => {
    const creatures = [{ name: "Rat", npc: false, outfit: { lookType: 21, head: 1 } }] as CreatureType[];
    const [ghost] = creatureGhost({ kind: "creature", creature: "Rat", label: "" }, creatures, 3);
    expect(ghost).toMatchObject({ name: "Rat", lookType: 21, head: 1, body: 0, dir: 3, npc: false, selected: false });
  });

  it("is empty for an unknown creature", () => {
    expect(creatureGhost({ kind: "creature", creature: "Dragon", label: "" }, [], 0)).toEqual([]);
  });
});

describe("computePreview", () => {
  it("shows nothing without a hovered tile", () => {
    expect(computePreview(input({ hover: null }))).toEqual({ ghosts: [], rects: [] });
  });

  it("shows nothing while panning", () => {
    const drag: MapDrag = { kind: "pan", x: 0, y: 0, camX: 0, camY: 0, button: 1, moved: true };
    expect(computePreview(input({ drag }))).toEqual({ ghosts: [], rects: [] });
  });

  it("shows nothing in selection mode", () => {
    expect(computePreview(input({ tool: "select" }))).toEqual({ ghosts: [], rects: [] });
  });

  it("draws a ghost of the brush item on each tile", () => {
    const { ghosts, rects } = computePreview(input({ size: 1 }));
    expect(ghosts).toHaveLength(9);
    expect(ghosts[0]).toEqual({ x: 9, y: 9, z: 7, ids: [100] });
    expect(rects).toEqual([{ x0: 9, y0: 9, x1: 11, y1: 11, z: 7, stroke: PREVIEW_COLORS.brushBounds }]);
  });

  it("marks erased tiles in red", () => {
    const { ghosts, rects } = computePreview(input({ erase: true }));
    expect(ghosts).toEqual([]);
    expect(rects).toEqual([{ x0: 10, y0: 10, x1: 10, y1: 10, z: 7, fill: PREVIEW_COLORS.erasedTile }]);
  });

  it("draws only the outline of a wall brush", () => {
    const { ghosts } = computePreview(input({ brush: { kind: "wall", item: 7, label: "" }, size: 1 }));
    expect(ghosts).toHaveLength(8);
  });

  it("tints house tiles", () => {
    const { rects } = computePreview(input({ brush: { kind: "house", house: 1, label: "" } }));
    expect(rects).toEqual([{ x0: 10, y0: 10, x1: 10, y1: 10, z: 7, fill: PREVIEW_COLORS.houseTile }]);
  });

  it("frames the tile a fill starts from", () => {
    const { rects } = computePreview(input({ fill: true }));
    expect(rects).toEqual([{ x0: 10, y0: 10, x1: 10, y1: 10, z: 7, ...PREVIEW_COLORS.fill }]);
  });

  it("frames a spawn's radius", () => {
    const { rects } = computePreview(input({ brush: { kind: "spawn", label: "" }, size: 0 }));
    expect(rects).toEqual([{ x0: 9, y0: 9, x1: 11, y1: 11, z: 7, stroke: PREVIEW_COLORS.spawn.stroke, fill: PREVIEW_COLORS.spawn.fill }]);
  });

  it("places a creature ghost", () => {
    const creatures = [{ name: "Rat", outfit: { lookType: 21 } }] as CreatureType[];
    const { ghosts } = computePreview(input({ brush: { kind: "creature", creature: "Rat", label: "" }, creatures }));
    expect(ghosts).toHaveLength(1);
    expect(ghosts[0].creatures?.[0].lookType).toBe(21);
  });

  it("offsets doodad tiles from the hovered tile", () => {
    const doodad = { tiles: [{ x: 1, y: -1, z: 0, items: [{ id: 50 }, { id: 51 }] }] } as unknown as DoodadBuffer;
    const { ghosts } = computePreview(input({ brush: { kind: "doodad", name: "tree", label: "" }, doodad }));
    expect(ghosts).toEqual([{ x: 11, y: 9, z: 7, ids: [50, 51] }]);
  });

  it("previews the clipboard while pasting", () => {
    const clip = { width: 2, height: 1, tiles: [{ dx: 1, dy: 0, dz: 0, ground: { id: 4, subtype: 0 }, items: [{ id: 5, subtype: 2 }] }] } as unknown as Clip;
    const { ghosts, rects } = computePreview(input({ pasting: true, clip }));
    expect(ghosts).toEqual([{ x: 11, y: 10, z: 7, ids: [4, 5], subs: [0, 2] }]);
    expect(rects).toEqual([{ x0: 10, y0: 10, x1: 11, y1: 10, z: 7, stroke: PREVIEW_COLORS.pasteBounds }]);
  });

  it("frames a selection box", () => {
    const drag: MapDrag = { kind: "box", start: at(1, 1), cur: at(3, 4), add: false, remove: false, px: { x: 0, y: 0 }, started: true };
    const { rects } = computePreview(input({ drag, tool: "select" }));
    expect(rects).toEqual([{ x0: 1, y0: 1, x1: 3, y1: 4, z: 7, ...PREVIEW_COLORS.selectionBox }]);
  });

  it("fills a dragged rectangle with ghosts", () => {
    const { ghosts, rects } = computePreview(input({ drag: rectDrag(at(3, 3), at(1, 1)) }));
    expect(ghosts).toHaveLength(9);
    expect(rects).toHaveLength(1);
  });

  it("skips ghosts of huge rectangles", () => {
    const side = Math.ceil(Math.sqrt(MAX_RECT_PREVIEW_TILES)) + 1;
    expect(computePreview(input({ drag: rectDrag(at(0, 0), at(side, side)) })).ghosts).toEqual([]);
  });

  it("does not ghost an erasing rectangle", () => {
    const { ghosts, rects } = computePreview(input({ drag: rectDrag(at(0, 0), at(2, 2), true) }));
    expect(ghosts).toEqual([]);
    expect(rects[0].fill).toEqual(PREVIEW_COLORS.rect.eraseFill);
  });
});
