import { decodeBytes, normalizeThing, type ObjectFile, type Thing } from "./api";

export interface DecodedObject {
  file: ObjectFile;
  thing: Thing;
  /** Sprite pixels by sprite id - 1. */
  pixels: Uint8ClampedArray[];
}

export function decodeObjectFile(file: ObjectFile | null | undefined, emptyMessage: string): DecodedObject {
  if (!file?.thing) throw new Error(emptyMessage);
  return { file, thing: normalizeThing(file.thing), pixels: (file.sprites ?? []).map((sprite) => decodeBytes(sprite as unknown as string)) };
}
