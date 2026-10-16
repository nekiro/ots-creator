// Pure parts of the brush editor: what a brush still needs before it can
// be saved, the shape the form expects and the cleanup before saving.
import type { Brush, Chance, Composite, PaletteTileset } from "../data/api";

export const MAIN_WALLS = ["horizontal", "vertical", "corner", "pole"];
export const MORE_WALLS = ["north end", "south end", "east end", "west end", "north T", "south T", "east T", "west T", "northeast diagonal", "southwest diagonal", "southeast diagonal", "intersection", "untouchable"];

/** The chances of a list without holes. */
export function chances(list: (Chance | null)[] | null | undefined): Chance[] {
  return (list ?? []).filter((chance): chance is Chance => !!chance);
}

/** The first item of a brush: the library icon when none is chosen. */
export function firstItem(brush: Brush): number {
  const ids = [
    ...chances(brush.items),
    ...Object.values(brush.walls ?? {}).flatMap((wall) => chances(wall?.items)),
    ...Object.values(brush.parts ?? {}).flatMap((list) => chances(list)),
    ...(brush.alternatives ?? []).flatMap((alternative) => (alternative?.singles ?? []).map((single) => single?.item)),
  ];
  return ids.find((item) => item?.id)?.id ?? 0;
}

/** What still stops saving a brush, as a hint; empty when it can be saved. */
export function missingForSave(brush: Brush, fromSelection: boolean): string {
  if (!brush.name?.trim()) return "Give the brush a name.";
  if (fromSelection) return "";
  const has = (list: (Chance | null)[] | null | undefined) => chances(list).some((chance) => chance.id);
  switch (brush.type) {
    case "ground":
      return has(brush.items) ? "" : "Add at least one ground item.";
    case "wall":
    case "decoration":
      return ["horizontal", "vertical"].every((align) => has(brush.walls?.[align]?.items)) ? "" : "Give the horizontal and vertical pieces.";
    case "doodad":
      return (brush.alternatives ?? []).some((alternative) => (alternative?.singles ?? []).some((single) => single?.item?.id) || (alternative?.composites ?? []).length)
        ? ""
        : "Add at least one item.";
    case "carpet":
      return has(brush.parts?.c) ? "" : "Pick the center piece.";
    case "table":
      return has(brush.parts?.alone) ? "" : "Pick the piece of a single table.";
  }
  return "";
}

/** The one group of tiles of a brush made from a selection; null for other brushes. */
export function stampOf(brush: Brush): Composite | null {
  const alternatives = (brush.alternatives ?? []).filter((alternative) => !!alternative);
  if (brush.type !== "doodad" || alternatives.length !== 1 || (alternatives[0]!.singles ?? []).length) return null;
  const composites = (alternatives[0]!.composites ?? []).filter((composite) => !!composite);
  return composites.length === 1 ? composites[0]! : null;
}

/** Fills in the look and drops what does not belong to the brush type (changes brush). */
export function normalizeBrushForSave(brush: Brush): Brush {
  if (!brush.look) brush.look = firstItem(brush);
  if (brush.walls) {
    for (const [align, wall] of Object.entries(brush.walls)) {
      const empty = !wall || (!(wall.items ?? []).some((chance) => chance?.id) && !(wall.doors ?? []).some((door) => door?.id));
      if (empty) delete brush.walls[align];
    }
  }
  if (brush.type !== "wall" && brush.type !== "decoration") brush.walls = {};
  if (brush.type !== "doodad") {
    brush.doodad = null;
    brush.alternatives = [];
  }
  return brush;
}

/** Walls and doodads edit nested objects: creates the missing ones (changes brush). */
export function ensureBrushShape(brush: Brush): void {
  if (brush.type === "wall" || brush.type === "decoration") {
    const walls = { ...(brush.walls ?? {}) };
    let added = false;
    for (const align of [...MAIN_WALLS, ...MORE_WALLS]) {
      if (!walls[align]) {
        walls[align] = { items: [], doors: [] };
        added = true;
      }
    }
    if (added) brush.walls = walls;
  }
  if (brush.type === "doodad" && !brush.doodad) brush.doodad = { draggable: true } as NonNullable<Brush["doodad"]>;
  if (brush.type === "doodad" && !(brush.alternatives ?? []).length) brush.alternatives = [{ singles: [], composites: [] }];
}

/** Names of the hand-made collections (tilesets) a brush is in. */
export function collectionsContaining(tilesets: (PaletteTileset | null)[] | null | undefined, brushName: string): string[] {
  return (tilesets ?? [])
    .filter((tileset) => !!tileset && !tileset.auto && [...(tileset.terrain ?? []), ...(tileset.doodads ?? [])].some((entry) => entry?.name === brushName))
    .map((tileset) => tileset!.name);
}
