// Item table of the map editor: server ids with their client ids, flags,
// names and minimap colors, in typed arrays for fast lookups while drawing.
import type { ItemTable } from "./api";

/** Top orders of ItemIndex.top: common items, borders, bottom items (walls) and top items. */
export const TOP_ORDER = { common: 0, border: 1, bottom: 2, top: 3 } as const;

export class ItemIndex {
  readonly maxId: number;
  readonly client: Uint16Array;
  readonly flags: Uint32Array;
  readonly top: Uint8Array;
  readonly minimap: Uint16Array;
  readonly kind: Uint8Array;
  /** Ground speed (0: not a ground or no speed). */
  readonly speed: Uint16Array;
  readonly exists: Uint8Array;
  readonly names: string[];
  readonly ids: number[];
  readonly bits: Record<string, number>;
  readonly mode: string;
  /** Lowercase names for searching. */
  private lowercaseNames: string[];

  constructor(table: ItemTable) {
    this.maxId = table.maxId ?? 0;
    const count = this.maxId + 1;
    this.client = new Uint16Array(count);
    this.flags = new Uint32Array(count);
    this.top = new Uint8Array(count);
    this.minimap = new Uint16Array(count);
    this.kind = new Uint8Array(count);
    this.speed = new Uint16Array(count);
    this.exists = new Uint8Array(count);
    this.names = new Array(count);
    this.lowercaseNames = new Array(count);
    this.ids = table.ids ?? [];
    this.bits = (table.flagbits ?? {}) as Record<string, number>;
    this.mode = table.mode;
    const ids = table.ids ?? [];
    for (let i = 0; i < ids.length; i++) {
      const id = ids[i];
      this.client[id] = table.clients?.[i] ?? 0;
      this.flags[id] = table.flags?.[i] ?? 0;
      this.top[id] = table.tops?.[i] ?? 0;
      this.minimap[id] = table.minimap?.[i] ?? 0;
      this.kind[id] = table.kinds?.[i] ?? 0;
      this.speed[id] = table.speeds?.[i] ?? 0;
      this.exists[id] = 1;
      const name = table.names?.[i] ?? "";
      this.names[id] = name;
      this.lowercaseNames[id] = name.toLowerCase();
    }
  }

  has(id: number): boolean {
    return id > 0 && id <= this.maxId && this.exists[id] === 1;
  }

  is(id: number, flag: string): boolean {
    return id <= this.maxId && (this.flags[id] & (this.bits[flag] ?? 0)) !== 0;
  }

  name(id: number): string {
    const name = id <= this.maxId ? this.names[id] : "";
    return name || `item ${id}`;
  }

  /** Items whose name contains the query or whose id it is, ids first. */
  search(query: string, limit = 500): number[] {
    const text = query.trim().toLowerCase();
    if (!text) return [];
    const out: number[] = [];
    const queryId = /^\d+$/.test(text) ? Number(text) : NaN;
    if (this.has(queryId)) out.push(queryId);
    // "100-200" lists an id range.
    const range = text.match(/^(\d+)\s*-\s*(\d+)$/);
    if (range) {
      for (let id = Number(range[1]); id <= Math.min(Number(range[2]), this.maxId) && out.length < limit; id++) {
        if (this.has(id) && id !== queryId) out.push(id);
      }
      return out;
    }
    for (const id of this.ids) {
      if (out.length >= limit) break;
      if (id !== queryId && this.lowercaseNames[id]?.includes(text)) out.push(id);
    }
    return out;
  }
}

/** Rgb of each 8-bit client color, three bytes each: a 6x6x6 cube, black at 0 and from 216 (client: Color::from8bit). */
export const CLIENT_COLOR_RGB: Uint8Array = (() => {
  const table = new Uint8Array(256 * 3);
  for (let color = 1; color < 216; color++) {
    table[color * 3] = Math.floor(color / 36) * 51;
    table[color * 3 + 1] = (Math.floor(color / 6) % 6) * 51;
    table[color * 3 + 2] = (color % 6) * 51;
  }
  return table;
})();

/** Offset of an 8-bit client color in CLIENT_COLOR_RGB; colors outside the cube are black. */
export function clientColorOffset(color: number): number {
  return color > 0 && color < 216 ? color * 3 : 0;
}

/** Tibia minimap color index to CSS rgb. */
export function minimapCss(color: number): string {
  if (color >= 216) return "#000";
  const offset = clientColorOffset(color);
  return `rgb(${CLIENT_COLOR_RGB[offset]},${CLIENT_COLOR_RGB[offset + 1]},${CLIENT_COLOR_RGB[offset + 2]})`;
}

/** Tibia minimap color index to 0xRRGGBB. */
export function minimapRgb(color: number): [number, number, number] {
  const offset = clientColorOffset(color);
  return [CLIENT_COLOR_RGB[offset], CLIENT_COLOR_RGB[offset + 1], CLIENT_COLOR_RGB[offset + 2]];
}
