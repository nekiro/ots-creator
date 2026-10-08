// Property editor metadata: groups, labels and payload fields.
import type { Properties } from "./api";

export type FieldKind = "u16" | "i16" | "text" | "select";

export interface Field {
  key: string; // path inside Properties, e.g. "groundSpeed" or "market.name"
  label: string;
  kind: FieldKind;
  options?: [number, string][];
}

export interface Flag {
  key: keyof Properties & string;
  label: string;
  hint?: string;
  fields?: Field[];
}

export interface FlagGroup {
  title: string;
  flags: Flag[];
}

export const CLOTH_SLOTS: [number, string][] = [
  [0, "None"], [1, "Two-handed"], [2, "Helmet"], [3, "Amulet"], [4, "Backpack"], [5, "Armor"],
  [6, "Shield"], [7, "One-handed"], [8, "Legs"], [9, "Boots"], [10, "Ring"], [11, "Ammunition"],
];

export const MARKET_CATEGORIES: [number, string][] = [
  [1, "Armors"], [2, "Amulets"], [3, "Boots"], [4, "Containers"], [5, "Decoration"], [6, "Food"],
  [7, "Helmets & Hats"], [8, "Legs"], [9, "Others"], [10, "Potions"], [11, "Rings"], [12, "Runes"],
  [13, "Shields"], [14, "Tools"], [15, "Valuables"], [16, "Ammunition"], [17, "Axes"], [18, "Clubs"],
  [19, "Distance Weapons"], [20, "Swords"], [21, "Wands & Rods"], [22, "Premium Scrolls"], [255, "Meta Weapons"],
];

export const PROFESSIONS: [number, string][] = [
  [0, "Any"], [1, "Knight"], [2, "Paladin"], [4, "Sorcerer"], [8, "Druid"], [16, "Monk"],
];

export const FLAG_GROUPS: FlagGroup[] = [
  {
    title: "Ground & Stacking",
    flags: [
      { key: "ground", label: "Ground", hint: "Walkable tile; speed affects walking", fields: [{ key: "groundSpeed", label: "Speed", kind: "u16" }] },
      { key: "groundBorder", label: "Ground border", hint: "Drawn above ground (borders)" },
      { key: "onBottom", label: "On bottom", hint: "Walls, drawn under items" },
      { key: "onTop", label: "On top", hint: "Drawn above creatures (doors, arches)" },
      { key: "fullGround", label: "Full ground", hint: "Ground covers the whole tile" },
    ],
  },
  {
    title: "Behaviour",
    flags: [
      { key: "container", label: "Container" },
      { key: "stackable", label: "Stackable" },
      { key: "forceUse", label: "Force use" },
      { key: "multiUse", label: "Multi use" },
      { key: "hasCharges", label: "Has charges" },
      { key: "fluidContainer", label: "Fluid container" },
      { key: "fluid", label: "Splash" },
      { key: "pickupable", label: "Pickupable" },
      { key: "rotatable", label: "Rotatable" },
      { key: "hangable", label: "Hangable" },
      { key: "hookSouth", label: "Hook south", hint: "Vertical wall hook" },
      { key: "hookEast", label: "Hook east", hint: "Horizontal wall hook" },
      { key: "usable", label: "Usable" },
      { key: "wrappable", label: "Wrappable" },
      { key: "unwrappable", label: "Unwrappable" },
      { key: "hasDefaultAction", label: "Default action", fields: [{ key: "defaultAction", label: "Action", kind: "select", options: [[0, "None"], [1, "Look"], [2, "Use"], [3, "Open"], [4, "Autowalk highlight"]] }] },
    ],
  },
  {
    title: "Blocking",
    flags: [
      { key: "unpassable", label: "Unpassable" },
      { key: "unmoveable", label: "Unmoveable" },
      { key: "blockMissile", label: "Block missiles" },
      { key: "blockPathfind", label: "Block pathfinding" },
      { key: "noMoveAnimation", label: "No move animation" },
      { key: "floorChange", label: "Floor change" },
    ],
  },
  {
    title: "Text",
    flags: [
      { key: "writable", label: "Writable", fields: [{ key: "maxTextLength", label: "Max length", kind: "u16" }] },
      { key: "writableOnce", label: "Writable once", fields: [{ key: "maxReadLength", label: "Max length", kind: "u16" }] },
    ],
  },
  {
    title: "Appearance",
    flags: [
      { key: "hasLight", label: "Light", fields: [{ key: "lightLevel", label: "Level", kind: "u16" }, { key: "lightColor", label: "Color", kind: "u16" }] },
      { key: "hasOffset", label: "Offset", fields: [{ key: "offsetX", label: "X", kind: "i16" }, { key: "offsetY", label: "Y", kind: "i16" }] },
      { key: "hasElevation", label: "Elevation", fields: [{ key: "elevation", label: "Height", kind: "u16" }] },
      { key: "miniMap", label: "Minimap", fields: [{ key: "miniMapColor", label: "Color", kind: "u16" }] },
      { key: "lyingObject", label: "Lying object" },
      { key: "animateAlways", label: "Animate always" },
      { key: "dontHide", label: "Don't hide" },
      { key: "translucent", label: "Translucent" },
      { key: "topEffect", label: "Top effect" },
      { key: "ignoreLook", label: "Ignore look" },
      { key: "lensHelp", label: "Lens help", fields: [{ key: "lensHelpValue", label: "Value", kind: "u16" }] },
    ],
  },
  {
    title: "Equipment & Market",
    flags: [
      { key: "cloth", label: "Cloth", fields: [{ key: "clothSlot", label: "Slot", kind: "select", options: CLOTH_SLOTS }] },
      {
        key: "isMarket",
        label: "Market",
        fields: [
          { key: "market.name", label: "Name", kind: "text" },
          { key: "market.category", label: "Category", kind: "select", options: MARKET_CATEGORIES },
          { key: "market.tradeAs", label: "Trade as", kind: "u16" },
          { key: "market.showAs", label: "Show as", kind: "u16" },
          { key: "market.restrictProfession", label: "Vocation", kind: "select", options: PROFESSIONS },
          { key: "market.restrictLevel", label: "Min level", kind: "u16" },
        ],
      },
    ],
  },
  {
    title: "Outfit",
    flags: [{ key: "hasBones", label: "Bones", hint: "Per-direction attachment offsets" }],
  },
];

// Flags that make sense outside items (category 1). Items get everything but bones.
const CATEGORY_FLAGS: Record<number, ReadonlySet<string>> = {
  2: new Set(["hasLight", "hasOffset", "animateAlways", "hasBones"]),
  3: new Set(["hasLight", "hasOffset", "animateAlways", "topEffect"]),
  4: new Set(["hasLight", "hasOffset"]),
};

/** Whether the editor shows a flag for a category. A flag that is already set
 * always shows, so stray flags can still be cleared. */
export function flagVisible(key: string, category: number, set: boolean): boolean {
  if (set) return true;
  const allowed = CATEGORY_FLAGS[category];
  return allowed ? allowed.has(key) : key !== "hasBones";
}

export function getPath(obj: any, path: string): any {
  return path.split(".").reduce((o, k) => (o == null ? o : o[k]), obj);
}

export function setPath(obj: any, path: string, value: any): void {
  const keys = path.split(".");
  const last = keys.pop()!;
  const target = keys.reduce((o, k) => o[k], obj);
  target[last] = value;
}

export function clampField(kind: FieldKind, v: number): number {
  if (!Number.isFinite(v)) return 0;
  v = Math.trunc(v);
  if (kind === "i16") return Math.min(Math.max(v, -32768), 32767);
  return Math.min(Math.max(v, 0), 65535);
}

/** Field metadata by key, e.g. "lightLevel" or "market.name". */
export function flagFields(key: string): Field[] {
  for (const g of FLAG_GROUPS) for (const f of g.flags) if (f.key === key) return f.fields ?? [];
  return [];
}

/**
 * Builds a bulk edit patch: flags set to true or false are written, fields
 * only for flags switched on (values default to 0 / ""). Nested fields such
 * as "market.name" become nested objects.
 */
export function buildPatch(states: Record<string, boolean | undefined>, values: Record<string, number | string>): Record<string, unknown> {
  const patch: Record<string, any> = {};
  for (const [key, on] of Object.entries(states)) {
    if (on === undefined) continue;
    patch[key] = on;
    if (!on) continue;
    for (const fd of flagFields(key)) {
      const v = values[fd.key] ?? (fd.kind === "text" ? "" : 0);
      const parts = fd.key.split(".");
      if (parts.length === 1) patch[fd.key] = v;
      else (patch[parts[0]] ??= {})[parts[1]] = v;
    }
  }
  return patch;
}
