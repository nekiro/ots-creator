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
  ObjectFile,
  ImageFile,
  ViewerEntry,
  MarketIndex,
  MarketConfig,
  MarketImport,
  ShareRequest,
  TargetPlan,
} from "../../bindings/github.com/nekiro/ots-creator/internal/app/models";
export type { Settings, Recent } from "../../bindings/github.com/nekiro/ots-creator/internal/settings/models";
export type { OptimizeOptions, OptimizeResult, CompileStats, OTBInfo, OTBReport, ConvertResult, PropsPatch, DiffEntry, DiffResult, TransferResult } from "../../bindings/github.com/nekiro/ots-creator/internal/project/models";
export { Format } from "../../bindings/github.com/nekiro/ots-creator/internal/project/models";
export type { Version, Features } from "../../bindings/github.com/nekiro/ots-creator/internal/client/models";
export type { FrameDuration, NpcSale, Properties } from "../../bindings/github.com/nekiro/ots-creator/internal/thing/models";
export { Category, AnimationMode } from "../../bindings/github.com/nekiro/ots-creator/internal/thing/models";

import {
  Category,
  type FrameDuration,
  type FrameGroup as GoFrameGroup,
  type Thing as GoThing,
  type NpcSale,
} from "../../bindings/github.com/nekiro/ots-creator/internal/thing/models";

/** Object files: the native .otobj and ObjectBuilder .obd. */
export const OBJECT_EXT = /\.(otobj|obd)$/i;

/** Payload of the "app:files-dropped" event (app.FilesDropped). */
export interface FilesDropped {
  paths: string[] | null;
  /** Id of the drop target element. */
  target: string;
}

// Go nil slices arrive as null; the UI works on normalized copies.
export type FrameGroup = Omit<GoFrameGroup, "sprites" | "durations"> & { sprites: number[]; durations: FrameDuration[] };
export type Thing = Omit<GoThing, "frameGroups" | "npcSales"> & { frameGroups: FrameGroup[]; npcSales: NpcSale[] };

export function normalizeThing(thing: GoThing): Thing {
  const plain = JSON.parse(JSON.stringify(thing)) as GoThing;
  return {
    ...plain,
    npcSales: plain.npcSales ?? [],
    frameGroups: (plain.frameGroups ?? []).filter((group): group is GoFrameGroup => !!group).map((group) => ({ ...group, sprites: group.sprites ?? [], durations: group.durations ?? [] })),
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

export function minId(category: Category): number {
  return category === Category.CategoryItem ? 100 : 1;
}

export const res = {
  thumb: (c: Category, id: number, ver: string | number) => `/res/thumb/${CATEGORY_NAMES[c]}/${id}?v=${ver}`,
  /** Thumbnail of the second client of a comparison. */
  otherThumb: (c: Category, id: number, ver: string | number) => `/res/b/thumb/${CATEGORY_NAMES[c]}/${id}?v=${ver}`,
  /** Full (uncropped) texture of a thing as the client draws it at map position x, y, z. */
  textureAt: (c: Category, id: number, x: number, y: number, z: number, ver: string | number) => `/res/texture/${CATEGORY_NAMES[c]}/${id}?x=${x}&y=${y}&z=${z}&wrap=1&v=${ver}`,
  spritePng:(id: number, ver: string | number) => `/res/spritepng/${id}?v=${ver}`,
  sheet: (c: Category, id: number, group: number, transparent: boolean, ver: string | number) =>
    `/res/sheet/${CATEGORY_NAMES[c]}/${id}?g=${group}&bg=${transparent ? "transparent" : "magenta"}&v=${ver}`,
  sprites: (ids: number[], rev: number) => `/res/sprites?ids=${ids.join(",")}&r=${rev}`,
  /** Raw sprites of the second client of a comparison. */
  otherSprites: (ids: number[], rev: number) => `/res/b/sprites?ids=${ids.join(",")}&r=${rev}`,
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

/** Formats a byte count as KB or MB. */
export function formatSize(bytes: number): string {
  return bytes < 1048576 ? `${(bytes / 1024).toFixed(1)} KB` : `${(bytes / 1048576).toFixed(1)} MB`;
}

/** Normalizes Go errors (strings or Error objects) into a message. */
export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === "string") return err;
  try {
    return JSON.stringify(err);
  } catch {
    return String(err);
  }
}
