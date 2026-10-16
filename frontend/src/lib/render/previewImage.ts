// The image of the object preview: the texture at a position with addons
// and outfit colors, and a rider stacked on its mount.
import type { FrameGroup } from "../api";
import { compose, stackBottomRight, type PixelSource } from "./compose";
import type { TexturePos } from "./layout";
import type { OutfitColors } from "./outfit";

export interface PreviewImageInput {
  frameGroup: FrameGroup;
  /** The mount's frame group when the outfit rides one. */
  mountGroup: FrameGroup | null;
  size: number;
  pos: TexturePos;
  isOutfit: boolean;
  /** Mounted is chosen (drawn only when the outfit has a mounted pattern). */
  mount: boolean;
  addon1: boolean;
  addon2: boolean;
  /** Outfit colors, null for the raw template colors. */
  colors: OutfitColors | null;
  get: PixelSource;
}

/** Outfits with a second pattern Z show it while mounted. */
export function mountedPattern(frameGroup: FrameGroup, isOutfit: boolean, mount: boolean): boolean {
  return isOutfit && mount && frameGroup.patternZ > 1;
}

/** The preview image and where the rider sits in it. */
export function renderPreviewImage(input: PreviewImageInput): { image: ImageData; riderOffset: [number, number] } {
  const { frameGroup, mountGroup, size, pos, isOutfit, colors, get } = input;
  const addons = isOutfit ? [input.addon1 ? 1 : -1, input.addon2 ? 2 : -1].filter((y) => y > 0) : [];
  const mounted = mountedPattern(frameGroup, isOutfit, input.mount);
  const riderPos = { ...pos, z: mounted ? 1 : pos.z };
  const rider = compose(frameGroup, frameGroup.sprites, size, { pos: riderPos, addons, colors }, get);
  if (!mounted || !mountGroup) return { image: rider, riderOffset: [0, 0] };
  const mountPos = { layer: 0, x: Math.min(pos.x, mountGroup.patternX - 1), y: 0, z: 0, frame: pos.frame % mountGroup.frames };
  const stacked = stackBottomRight([compose(mountGroup, mountGroup.sprites, size, { pos: mountPos, colors }, get), rider]);
  return { image: stacked.image, riderOffset: stacked.offsets[1] };
}
