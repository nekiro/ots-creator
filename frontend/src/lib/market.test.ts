import { describe, expect, it } from "vitest";
import { filterEntries, formatIds, groupOptions, matchTags, MAX_TAGS, parseIds, parseTags, tagGroups, sectionCounts, tagProblem, topTags, type MarketEntry } from "./market";

const entry = (id: string, over: Partial<MarketEntry>): MarketEntry => ({
  id,
  name: id,
  kind: "object",
  category: "item",
  tags: [],
  license: "CC0-1.0",
  author: "alice",
  spriteSize: 32,
  created: "2026-01-01T00:00:00Z",
  width: 1,
  height: 1,
  sprites: 1,
  file: "",
  preview: "",
  bytes: 0,
  ...over,
});

const entries = [
  entry("rat", { name: "Rat", category: "outfit", tags: ["monster", "sewer"], created: "2026-03-01T00:00:00Z", spriteSize: 32 }),
  entry("sword", { name: "Fire sword", tags: ["weapon", "magic"], description: "Burns", created: "2026-02-01T00:00:00Z", spriteSize: 32 }),
  entry("walls", { name: "Walls", kind: "sprites", category: undefined, tags: ["wall"], author: "bob", created: "2026-04-01T00:00:00Z", spriteSize: 64 }),
  entry("demon", { name: "Demon", category: "outfit", tags: ["monster"], created: "2026-01-15T00:00:00Z", spriteSize: 32 }),
];

const base = { section: "all", tags: [], query: "", sort: "new" as const, spriteSize: 0 };
const ids = (es: MarketEntry[]) => es.map((e) => e.id);

describe("market filter", () => {
  it("sorts newest first by default", () => {
    expect(ids(filterEntries(entries, base))).toEqual(["walls", "rat", "sword", "demon"]);
    expect(ids(filterEntries(entries, { ...base, sort: "name" }))).toEqual(["demon", "sword", "rat", "walls"]);
  });
  it("filters by section, tags, sprite size and words", () => {
    expect(ids(filterEntries(entries, { ...base, section: "outfit" }))).toEqual(["rat", "demon"]);
    expect(ids(filterEntries(entries, { ...base, section: "sprites" }))).toEqual(["walls"]);
    expect(ids(filterEntries(entries, { ...base, tags: ["monster", "sewer"] }))).toEqual(["rat"]);
    expect(ids(filterEntries(entries, { ...base, spriteSize: 32 }))).toEqual(["rat", "sword", "demon"]);
    expect(ids(filterEntries(entries, { ...base, query: "BURNS fire" }))).toEqual(["sword"]);
    expect(ids(filterEntries(entries, { ...base, query: "bob" }))).toEqual(["walls"]);
  });
  it("counts sections and tags", () => {
    expect(sectionCounts(entries)).toEqual({ all: 4, outfit: 2, item: 1, sprites: 1 });
    expect(topTags(entries, 2)).toEqual([
      { tag: "monster", count: 2 },
      { tag: "magic", count: 1 },
    ]);
  });
  it("parses and checks tags like the Go side", () => {
    expect(parseTags(" Big  Rat , big-rat,, Sewer")).toEqual(["big-rat", "sewer"]);
    expect(tagProblem(["ok", "fine-2"])).toBe("");
    expect(tagProblem(["żaba"])).not.toBe("");
    expect(tagProblem(Array.from({ length: 9 }, (_, i) => `t${i}`))).not.toBe("");
  });
  it("offers valid tags for every kind", () => {
    for (const kind of ["item", "outfit", "effect", "missile", "sprites"] as const) {
      const all = tagGroups(kind).flatMap((g) => g.tags);
      expect(tagProblem(all.slice(0, MAX_TAGS))).toBe("");
      for (const t of all) expect(tagProblem([t])).toBe("");
      expect(new Set(all).size).toBe(all.length);
    }
    expect(tagGroups("outfit")[1].tags).toContain("lycanthrope");
    expect(tagGroups("item")[0].tags).toContain("spellbook");
    expect(tagGroups("item").map((g) => g.label)).toContain("House");
  });
  it("parses sprite id lists", () => {
    expect(parseIds("5, 1-3 ,2; 9-7 x 0 200", 100)).toEqual([1, 2, 3, 5, 7, 8, 9]);
    expect(parseIds("1-5000", 100, 10)).toHaveLength(10);
    expect(formatIds([1, 2, 3, 5, 7, 8])).toBe("1-3, 5, 7-8");
    expect(parseIds(formatIds([4, 5, 6, 10]), 100)).toEqual([4, 5, 6, 10]);
  });
  it("matches tags while typing", () => {
    const opts = groupOptions([
      { label: "Equip", tags: ["sword", "shield", "two-handed"] },
      { label: "Map", tags: ["swamp", "water"] },
    ]);
    const tags = (q: string, picked: string[] = []) => matchTags(opts, q, picked).map((o) => o.tag);
    expect(tags("")).toEqual(["sword", "shield", "two-handed", "swamp", "water"]);
    expect(tags("sw")).toEqual(["sword", "swamp"]);
    expect(tags("a")).toEqual(["two-handed", "swamp", "water"]);
    expect(tags("two handed")).toEqual(["two-handed"]);
    expect(tags("map")).toEqual(["swamp", "water"]);
    expect(tags("s", ["sword"])).toEqual(["shield", "swamp"]);
  });
});
