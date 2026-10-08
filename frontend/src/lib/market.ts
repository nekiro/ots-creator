// Filtering and sorting of market entries (index.json rows).
import type { Entry } from "../../bindings/github.com/nekiro/ots-creator/internal/market/models";

export type MarketEntry = Entry;

/** "all", "sprites" or an object category name ("item", "outfit"...). */
export type MarketSection = string;

export type MarketSort = "new" | "old" | "name";

export interface MarketFilter {
  section: MarketSection;
  tags: string[];
  query: string;
  sort: MarketSort;
  /** Only entries with this sprite size (0 = any). */
  spriteSize: number;
}

export const LICENSES = [
  { value: "CC0-1.0", label: "CC0 (public domain)" },
  { value: "CC-BY-4.0", label: "CC BY 4.0 (credit me)" },
  { value: "CC-BY-SA-4.0", label: "CC BY-SA 4.0 (credit, share alike)" },
  { value: "OTS-free", label: "Free for OT servers" },
];

/** What a tag group applies to: an object category or "sprites". */
export type TagKind = "item" | "outfit" | "effect" | "missile" | "sprites";

export interface TagGroup {
  label: string;
  tags: string[];
}

// Curated tags in Tibia terms (bestiary classes, market categories, damage
// types, cities), so entries are described the same way. Authors can still
// add their own.
const ROLE = [
  "monster",
  "boss",
  "npc",
  "player",
  "addon",
  "mount",
  "familiar",
  "summon",
];
const CLASS = [
  "amphibic",
  "aquatic",
  "bird",
  "construct",
  "demon",
  "dragon",
  "elemental",
  "extra-dimensional",
  "fey",
  "giant",
  "human",
  "humanoid",
  "lycanthrope",
  "magical",
  "mammal",
  "plant",
  "reptile",
  "slime",
  "undead",
  "vermin",
];
// Item groups follow items.xml: slotType / weaponType (equipment), type
// and fluidSource (usable), house items and map pieces (floorchange,
// magic fields, the most common words of item names).
const EQUIPMENT = [
  "helmet",
  "armor",
  "legs",
  "boots",
  "shield",
  "spellbook",
  "quiver",
  "amulet",
  "ring",
  "backpack",
  "sword",
  "axe",
  "club",
  "two-handed",
  "bow",
  "crossbow",
  "distance",
  "ammunition",
  "wand",
  "rod",
];
const USABLE = [
  "rune",
  "potion",
  "fluid",
  "food",
  "key",
  "container",
  "chest",
  "book",
  "tool",
  "valuable",
  "creature-product",
  "trophy",
  "quest",
  "decoration",
];
const HOUSE = [
  "door",
  "bed",
  "furniture",
  "chair",
  "table",
  "carpet",
  "tapestry",
  "window",
  "lamp",
  "sign",
  "mailbox",
  "depot",
  "trash",
];
const MAP = [
  "ground",
  "border",
  "wall",
  "roof",
  "railing",
  "archway",
  "gate",
  "palisade",
  "stairs",
  "ramp",
  "ladder",
  "hole",
  "teleport",
  "magic-field",
  "tree",
  "bush",
  "plant",
  "rock",
  "mountain",
  "water",
  "lava",
  "swamp",
  "snow",
  "sand",
  "grass",
  "corpse",
  "bones",
  "debris",
  "statue",
  "ship",
];
const STYLE = [
  "wooden",
  "stone",
  "sandstone",
  "marble",
  "gold",
  "frozen",
  "elven",
  "dwarven",
  "orcish",
  "oriental",
  "bamboo",
  "jungle",
  "desert",
  "nordic",
  "demonic",
  "holy-place",
];
// Effects and missiles named like the client's magic effects and
// shoot types.
const EFFECT = [
  "hit",
  "block",
  "blood",
  "puff",
  "explosion",
  "area",
  "wave",
  "beam",
  "strike",
  "aura",
  "rings",
  "sparkles",
  "bubbles",
  "sound",
  "teleport",
  "healing",
];
const MISSILE = [
  "arrow",
  "bolt",
  "spear",
  "star",
  "knife",
  "stone",
  "snowball",
  "rune",
  "spell",
];
const SPRITE = ["creature", "item", "effect", "interface"];
const ELEMENT = [
  "physical",
  "fire",
  "ice",
  "energy",
  "earth",
  "death",
  "holy",
  "drown",
  "lifedrain",
  "manadrain",
];
const REGION = [
  "rookgaard",
  "thais",
  "carlin",
  "venore",
  "abdendriel",
  "kazordoon",
  "edron",
  "darashia",
  "ankrahmun",
  "port-hope",
  "liberty-bay",
  "svargrond",
  "yalahar",
  "zao",
  "farmine",
  "roshamuul",
  "oramond",
  "feyrist",
  "issavi",
];
const ORIGIN = ["custom", "edited", "remake", "animated", "oldschool", "event"];

const GROUPS: Record<TagKind, TagGroup[]> = {
  outfit: [
    { label: "Role", tags: ROLE },
    { label: "Class", tags: CLASS },
    { label: "Element", tags: ELEMENT },
    { label: "Region", tags: REGION },
    { label: "Origin", tags: ORIGIN },
  ],
  item: [
    { label: "Equip", tags: EQUIPMENT },
    { label: "Usable", tags: USABLE },
    { label: "House", tags: HOUSE },
    { label: "Map", tags: MAP },
    { label: "Style", tags: STYLE },
    { label: "Element", tags: ELEMENT },
    { label: "Region", tags: REGION },
    { label: "Origin", tags: ORIGIN },
  ],
  effect: [
    { label: "Effect", tags: EFFECT },
    { label: "Element", tags: ELEMENT },
    { label: "Origin", tags: ORIGIN },
  ],
  missile: [
    { label: "Missile", tags: MISSILE },
    { label: "Element", tags: ELEMENT },
    { label: "Origin", tags: ORIGIN },
  ],
  sprites: [
    { label: "Type", tags: SPRITE },
    { label: "Map", tags: MAP },
    { label: "House", tags: HOUSE },
    { label: "Style", tags: STYLE },
    { label: "Region", tags: REGION },
    { label: "Origin", tags: ORIGIN },
  ],
};

/** Tag groups offered when sharing an entry of a kind. */
export function tagGroups(kind: TagKind): TagGroup[] {
  return GROUPS[kind];
}

/** Maximum number of tags of an entry. */
export const MAX_TAGS = 8;

/** The section of an entry: "sprites" for packs, else its category. */
export const sectionOf = (e: MarketEntry): string =>
  e.kind === "sprites" ? "sprites" : (e.category ?? "");

function matches(e: MarketEntry, words: string[]): boolean {
  if (words.length === 0) return true;
  const text = [e.name, e.description ?? "", e.author, ...(e.tags ?? [])]
    .join(" ")
    .toLowerCase();
  return words.every((w) => text.includes(w));
}

/** Entries matching the filter, sorted. */
export function filterEntries(
  entries: MarketEntry[],
  f: MarketFilter,
): MarketEntry[] {
  const words = f.query.toLowerCase().split(/\s+/).filter(Boolean);
  const out = entries.filter(
    (e) =>
      (f.section === "all" || sectionOf(e) === f.section) &&
      (f.spriteSize === 0 || !e.spriteSize || e.spriteSize === f.spriteSize) &&
      f.tags.every((t) => e.tags?.includes(t)) &&
      matches(e, words),
  );
  const byName = (a: MarketEntry, b: MarketEntry) =>
    a.name.localeCompare(b.name, undefined, {
      sensitivity: "base",
      numeric: true,
    });
  switch (f.sort) {
    case "name":
      return out.sort(byName);
    case "old":
      return out.sort(
        (a, b) => a.created.localeCompare(b.created) || byName(a, b),
      );
    default:
      return out.sort(
        (a, b) => b.created.localeCompare(a.created) || byName(a, b),
      );
  }
}

/** Entry counts per section ("all" included). */
export function sectionCounts(entries: MarketEntry[]): Record<string, number> {
  const out: Record<string, number> = { all: entries.length };
  for (const e of entries) out[sectionOf(e)] = (out[sectionOf(e)] ?? 0) + 1;
  return out;
}

/** The most used tags of the entries, most used first. */
export function topTags(
  entries: MarketEntry[],
  limit = 24,
): { tag: string; count: number }[] {
  const counts = new Map<string, number>();
  for (const e of entries)
    for (const t of e.tags ?? []) counts.set(t, (counts.get(t) ?? 0) + 1);
  return [...counts]
    .map(([tag, count]) => ({ tag, count }))
    .sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag))
    .slice(0, limit);
}

/** Splits a typed tag list ("Big rat, sewer") like the Go side does. */
export function parseTags(text: string): string[] {
  const out: string[] = [];
  for (const raw of text.split(",")) {
    const t = raw.trim().toLowerCase().split(/\s+/).filter(Boolean).join("-");
    if (t && !out.includes(t)) out.push(t);
  }
  return out;
}

/**
 * Parses a sprite id list like "100-120, 130" into sorted unique ids
 * between 1 and max. At most limit ids are returned.
 */
export function parseIds(text: string, max: number, limit = 1024): number[] {
  const out = new Set<number>();
  for (const part of text.split(/[\s,;]+/)) {
    const m = part.match(/^(\d+)(?:-(\d+))?$/);
    if (!m) continue;
    let [a, b] = [+m[1], +(m[2] ?? m[1])];
    if (a > b) [a, b] = [b, a];
    for (
      let id = Math.max(a, 1);
      id <= Math.min(b, max) && out.size < limit;
      id++
    )
      out.add(id);
  }
  return [...out].sort((x, y) => x - y);
}

/** Formats ids as ranges ("100-120, 130"), the inverse of parseIds. */
export function formatIds(ids: number[]): string {
  const parts: string[] = [];
  for (let i = 0; i < ids.length;) {
    let j = i;
    while (j + 1 < ids.length && ids[j + 1] === ids[j] + 1) j++;
    parts.push(j > i ? `${ids[i]}-${ids[j]}` : `${ids[i]}`);
    i = j + 1;
  }
  return parts.join(", ");
}

/** Problems with a typed tag list, or "" when it is fine. */
export function tagProblem(tags: string[]): string {
  if (tags.length > 8) return "At most 8 tags.";
  const bad = tags.find(
    (t) => t.length > 24 || !/^[a-z0-9][a-z0-9-]*$/.test(t),
  );
  return bad
    ? `Tag "${bad}": letters, digits and dashes only, at most 24.`
    : "";
}

/** A tag the picker offers: its group (for headers) and an optional count. */
export interface TagOption {
  tag: string;
  group?: string;
  count?: number;
}

/** Flattens tag groups into picker options. */
export const groupOptions = (groups: TagGroup[]): TagOption[] => groups.flatMap((g) => g.tags.map((tag) => ({ tag, group: g.label })));

/**
 * Options matching a typed query, without the picked ones: tags starting
 * with the query first, then tags containing it; groups keep their order.
 * An empty query lists everything.
 */
export function matchTags(options: TagOption[], query: string, picked: string[], limit = 200): TagOption[] {
  const q = query.trim().toLowerCase().replace(/\s+/g, "-");
  const free = options.filter((o) => !picked.includes(o.tag));
  if (!q) return free.slice(0, limit);
  const starts = free.filter((o) => o.tag.startsWith(q));
  const contains = free.filter((o) => !o.tag.startsWith(q) && (o.tag.includes(q) || (o.group ?? "").toLowerCase().startsWith(q)));
  return [...starts, ...contains].slice(0, limit);
}
