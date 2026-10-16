// Zoom levels of the object preview and the sheet editor.
import { clamp } from "../util/math";

/** Largest whole zoom (1 to max) at which content fits in a stage, keeping a margin. */
export function fitZoom(stageWidth: number, stageHeight: number, contentWidth: number, contentHeight: number, margin: number, max: number): number {
  const ratio = Math.min((stageWidth - margin) / contentWidth, (stageHeight - margin) / contentHeight);
  return clamp(Math.floor(ratio), 1, max);
}
