// Item table of the map editor: server ids with their client ids, flags,
// names and minimap colors, in typed arrays for fast lookups while drawing.
import type { ItemTable } from "./api";

export class ItemIndex {
  readonly maxId: number;
  readonly client: Uint16Array;
  readonly flags: Uint32Array;
  readonly top: Uint8Array;
  readonly minimap: Uint16Array;
  readonly kind: Uint8Array;
  readonly exists: Uint8Array;
  readonly names: string[];
  readonly ids: number[];
  readonly bits: Record<string, number>;
  readonly mode: string;
  /** Lowercase names for searching. */
  private lower: string[];

  constructor(table: ItemTable) {
    this.maxId = table.maxId ?? 0;
    const count = this.maxId + 1;
    this.client = new Uint16Array(count);
    this.flags = new Uint32Array(count);
    this.top = new Uint8Array(count);
    this.minimap = new Uint16Array(count);
    this.kind = new Uint8Array(count);
    this.exists = new Uint8Array(count);
    this.names = new Array(count);
    this.lower = new Array(count);
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
      this.exists[id] = 1;
      const name = table.names?.[i] ?? "";
      this.names[id] = name;
      this.lower[id] = name.toLowerCase();
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
    const num = /^\d+$/.test(text) ? Number(text) : NaN;
    if (this.has(num)) out.push(num);
    // "100-200" lists an id range.
    const range = text.match(/^(\d+)\text*-\text*(\d+)$/);
    if (range) {
      for (let id = Number(range[1]); id <= Math.min(Number(range[2]), this.maxId) && out.length < limit; id++) {
        if (this.has(id) && id !== num) out.push(id);
      }
      return out;
    }
    for (const id of this.ids) {
      if (out.length >= limit) break;
      if (id !== num && this.lower[id]?.includes(text)) out.push(id);
    }
    return out;
  }
}

/** Tibia minimap color index to CSS rgb. */
export function minimapCss(color: number): string {
  if (color >= 216) return "#000";
  return `rgb(${Math.floor(color / 36) % 6 * 51},${Math.floor(color / 6) % 6 * 51},${(color % 6) * 51})`;
}

/** Tibia minimap color index to 0xRRGGBB. */
export function minimapRgb(color: number): [number, number, number] {
  if (color >= 216 || color <= 0) return [0, 0, 0];
  return [(Math.floor(color / 36) % 6) * 51, (Math.floor(color / 6) % 6) * 51, (color % 6) * 51];
}
