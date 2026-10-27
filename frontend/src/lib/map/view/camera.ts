import { clamp } from "../../util/math";
import { pos, type MapInfo, type Position } from "../data/api";
import { GROUND_FLOOR } from "../constants";

/** Zoom levels the view steps through (device pixels per world pixel, before the pixel ratio). */
export const ZOOMS = [0.03125, 0.0625, 0.125, 0.1875, 0.25, 0.375, 0.5, 0.75, 1, 1.5, 2, 3, 4, 6, 8];

/** The zoom level `step` levels from the current zoom (a zoom between levels counts as the next one up). */
export function stepZoom(current: number, step: number): number {
  let index = ZOOMS.findIndex((zoom) => zoom >= current - 1e-6);
  if (index < 0) index = ZOOMS.length - 1;
  return ZOOMS[clamp(index + step, 0, ZOOMS.length - 1)];
}

/** Camera center after zooming so the point offset (CSS pixels from the view center) stays in place. */
export function zoomAround(camX: number, camY: number, zoom: number, next: number, offsetX: number, offsetY: number): { x: number; y: number } {
  return { x: camX + offsetX / zoom - offsetX / next, y: camY + offsetY / zoom - offsetY / next };
}

/** Tiles a pan of `count` tiles at normal zoom moves at `zoom`: the same distance on screen, at least one tile. */
export function panTiles(count: number, zoom: number): number {
  return count ? Math.sign(count) * Math.max(1, Math.round(Math.abs(count) * (1 / zoom))) : 0;
}

const MAX_START = 1000;
const DEFAULT_MAP_SIZE = 2048;

/** Where a map is first shown: its first temple, the middle of its tiles, or near the middle of its area. */
export function startPosition(info: Pick<MapInfo, "hasBounds" | "bounds" | "towns" | "props"> | undefined): Position {
  if (info?.hasBounds) {
    const [x0, y0, x1, y1] = info.bounds;
    const town = info.towns?.[0];
    if (town && town.temple.x) return town.temple;
    return pos(Math.round((x0 + x1) / 2), Math.round((y0 + y1) / 2), GROUND_FLOOR);
  }
  const width = info?.props.width ?? DEFAULT_MAP_SIZE;
  const height = info?.props.height ?? DEFAULT_MAP_SIZE;
  return pos(Math.min(MAX_START, Math.floor(width / 2)), Math.min(MAX_START, Math.floor(height / 2)), GROUND_FLOOR);
}
