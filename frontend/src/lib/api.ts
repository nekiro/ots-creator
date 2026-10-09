// Thin, typed access to the Go services and the /res resource endpoints.
export * as ProjectService from "../../bindings/github.com/nekiro/ots-creator/internal/app/projectservice";
export * as ThingService from "../../bindings/github.com/nekiro/ots-creator/internal/app/thingservice";
export * as SpriteService from "../../bindings/github.com/nekiro/ots-creator/internal/app/spriteservice";
export * as DialogService from "../../bindings/github.com/nekiro/ots-creator/internal/app/dialogservice";
export * as UpdateService from "../../bindings/github.com/nekiro/ots-creator/internal/app/updateservice";
export * as WindowService from "../../bindings/github.com/nekiro/ots-creator/internal/app/windowservice";
export * as ViewerService from "../../bindings/github.com/nekiro/ots-creator/internal/app/viewerservice";
export * as SettingsService from "../../bindings/github.com/nekiro/ots-creator/internal/app/settingsservice";
export * as MarketService from "../../bindings/github.com/nekiro/ots-creator/internal/app/marketservice";
export * as CompareService from "../../bindings/github.com/nekiro/ots-creator/internal/app/compareservice";
export type {
  State,
  ClientFiles,
  OpenRequest,
  CompileRequest,
  FileFilter,
  ImportResult,
  UpdateInfo,
  OBDFile,
  ImageFile,
  ViewerEntry,
  MarketIndex,
  MarketConfig,
  MarketImport,
  ShareRequest,
} from "../../bindings/github.com/nekiro/ots-creator/internal/app/models";
export type { Settings, Recent } from "../../bindings/github.com/nekiro/ots-creator/internal/settings/models";
export type { OptimizeOptions, OptimizeResult, ConvertResult, PropsPatch, DiffEntry, DiffResult, TransferResult } from "../../bindings/github.com/nekiro/ots-creator/internal/project/models";
export { Format } from "../../bindings/github.com/nekiro/ots-creator/internal/project/models";
export type { Version, Features } from "../../bindings/github.com/nekiro/ots-creator/internal/client/models";
export type { FrameDuration, Properties } from "../../bindings/github.com/nekiro/ots-creator/internal/thing/models";
export { Category, AnimationMode } from "../../bindings/github.com/nekiro/ots-creator/internal/thing/models";

import {
  Category,
  type FrameDuration,
  type FrameGroup as GoFrameGroup,
  type Thing as GoThing,
} from "../../bindings/github.com/nekiro/ots-creator/internal/thing/models";

/** Payload of the "app:files-dropped" event (app.FilesDropped). */
export interface FilesDropped {
  paths: string[] | null;
  /** Id of the drop target element. */
  target: string;
}

// Go nil slices arrive as null; the UI works on normalized copies.
export type FrameGroup = Omit<GoFrameGroup, "sprites" | "durations"> & { sprites: number[]; durations: FrameDuration[] };
export type Thing = Omit<GoThing, "frameGroups"> & { frameGroups: FrameGroup[] };

export function normalizeThing(t: GoThing): Thing {
  const plain = JSON.parse(JSON.stringify(t)) as GoThing;
  return {
    ...plain,
    frameGroups: (plain.frameGroups ?? []).filter((g): g is GoFrameGroup => !!g).map((g) => ({ ...g, sprites: g.sprites ?? [], durations: g.durations ?? [] })),
  };
}

export const CATEGORY_NAMES: Record<number, string> = {
  [Category.CategoryItem]: "item",
  [Category.CategoryOutfit]: "outfit",
  [Category.CategoryEffect]: "effect",
  [Category.CategoryMissile]: "missile",
};

export const CATEGORY_LABELS: Record<number, string> = {
  [Category.CategoryItem]: "Items",
  [Category.CategoryOutfit]: "Outfits",
  [Category.CategoryEffect]: "Effects",
  [Category.CategoryMissile]: "Missiles",
};

export const CATEGORIES = [Category.CategoryItem, Category.CategoryOutfit, Category.CategoryEffect, Category.CategoryMissile];

export function minId(c: Category): number {
  return c === Category.CategoryItem ? 100 : 1;
}

export const res = {
  thumb: (c: Category, id: number, ver: string | number) => `/res/thumb/${CATEGORY_NAMES[c]}/${id}?v=${ver}`,
  /** Thumbnail of the second client of a comparison. */
  otherThumb: (c: Category, id: number, ver: string | number) => `/res/b/thumb/${CATEGORY_NAMES[c]}/${id}?v=${ver}`,
  spritePng: (id: number, ver: string | number) => `/res/spritepng/${id}?v=${ver}`,
  sheet: (c: Category, id: number, group: number, transparent: boolean, ver: string | number) =>
    `/res/sheet/${CATEGORY_NAMES[c]}/${id}?g=${group}&bg=${transparent ? "transparent" : "magenta"}&v=${ver}`,
  sprites: (ids: number[], rev: number) => `/res/sprites?ids=${ids.join(",")}&r=${rev}`,
};

/** Decodes a Go []byte (base64 in JSON) into bytes. */
export function decodeBytes(b64: string | null | undefined): Uint8ClampedArray {
  const bin = atob(b64 ?? "");
  const out = new Uint8ClampedArray(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

/** Encodes bytes as base64 for a Go []byte parameter. */
export function encodeBytes(bytes: Uint8Array | Uint8ClampedArray): string {
  let bin = "";
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(bin);
}

/** Normalizes Go errors (strings or Error objects) into a message. */
export function errorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  try {
    return JSON.stringify(e);
  } catch {
    return String(e);
  }
}
