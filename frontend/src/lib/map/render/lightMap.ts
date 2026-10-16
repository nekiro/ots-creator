// The light of the map as the client computes it (OTClient LightView): one
// color per tile, the brightest of the ambient light and the item lights
// reaching the tile's center. Floors add their lights bottom up; a tile with
// a ground hides the light of the floors under it. The map is multiplied by
// it, stretched with linear filtering, so light fades from tile to tile.
import { CHUNK, splitKey } from "../data/chunks";
import { CLIENT_COLOR_RGB, clientColorOffset } from "../data/items";
import { GEO_BLOCK, LIGHT_BYTES, type GeoChunk } from "./geo";

/** Ambient light at or above this intensity is full daylight: nothing is darkened (client: LightView::isDark). */
export const DAYLIGHT_INTENSITY = 250;
/** The client's white, and the color of the ambient light. */
export const WHITE_LIGHT_COLOR = 215;
/** Light falls off over this many tiles at its edge (client: intensity * 0.2 per tile). */
const FALLOFF_PER_TILE = 0.2;
/** Lights weaker than this at a tile leave it as it is. */
const MIN_LIGHT = 0.01;

/** The rgb of an 8-bit client color (a 6x6x6 cube) times brightness; colors 0 and from 216 are black (client: Color::from8bit). */
export function lightColor(color: number, brightness = 1): [number, number, number] {
  const offset = clientColorOffset(color);
  return [Math.trunc(CLIENT_COLOR_RGB[offset] * brightness), Math.trunc(CLIENT_COLOR_RGB[offset + 1] * brightness), Math.trunc(CLIENT_COLOR_RGB[offset + 2] * brightness)];
}

/** Intensity of the ambient light at an ambient level (0 to 100 %, client: minimum ambient light). */
export function ambientIntensity(percent: number): number {
  return Math.trunc((Math.min(Math.max(percent, 0), 100) / 100) * 255);
}

/** A light that is not in the chunks (the play view's player): at a tile position of its floor, in tiles. */
export interface ExtraLight {
  x: number;
  y: number;
  intensity: number;
  color: number;
}

/** The chunks of one floor and how many tiles it is drawn up and left (see Camera.shift). */
export interface LightFloor {
  chunks: GeoChunk[];
  shift: number;
  /** Lights added after the chunks' lights. */
  extra?: ExtraLight[];
}

/** A rectangle of tiles on the screen grid (tile positions minus their floor's shift). */
export interface LightGrid {
  x: number;
  y: number;
  width: number;
  height: number;
}

/**
 * Fills pixels (rgba, width x height of the grid) with the light of each
 * tile of the grid. floors run bottom up; spriteSize is the world pixels
 * of a tile.
 */
export function buildLightMap(pixels: Uint8Array, grid: LightGrid, floors: LightFloor[], ambient: number, spriteSize: number): void {
  const [ambientRed, ambientGreen, ambientBlue] = lightColor(WHITE_LIGHT_COLOR, ambient / 255);
  const texels = grid.width * grid.height;
  for (let texel = 0; texel < texels; texel++) {
    pixels[texel * 4] = ambientRed;
    pixels[texel * 4 + 1] = ambientGreen;
    pixels[texel * 4 + 2] = ambientBlue;
    pixels[texel * 4 + 3] = 255;
  }
  const resetToAmbient = (gridX: number, gridY: number) => {
    if (gridX < 0 || gridY < 0 || gridX >= grid.width || gridY >= grid.height) return;
    const offset = (gridY * grid.width + gridX) * 4;
    pixels[offset] = ambientRed;
    pixels[offset + 1] = ambientGreen;
    pixels[offset + 2] = ambientBlue;
  };
  for (const { chunks, shift, extra } of floors) {
    // Grounds first: the floor's own lights shine over them.
    for (const chunk of chunks) {
      if (!chunk.shades) continue;
      const [chunkX, chunkY] = splitKey(chunk.key % GEO_BLOCK);
      const size = Math.round(Math.sqrt(chunk.shades.length * 8));
      const originX = chunkX * CHUNK - shift - grid.x;
      const originY = chunkY * CHUNK - shift - grid.y;
      const shades = chunk.shades;
      for (let byte = 0; byte < shades.length; byte++) {
        const bits = shades[byte];
        if (!bits) continue;
        for (let bit = 0; bit < 8; bit++) {
          if (!(bits & (1 << bit))) continue;
          const tile = byte * 8 + bit;
          resetToAmbient(originX + (tile % size), originY + Math.floor(tile / size));
        }
      }
    }
    for (const chunk of chunks) {
      if (!chunk.lightCount) continue;
      const [chunkX, chunkY] = splitKey(chunk.key % GEO_BLOCK);
      const originX = chunkX * CHUNK - shift - grid.x;
      const originY = chunkY * CHUNK - shift - grid.y;
      for (let index = 0; index < chunk.lightCount; index++) {
        const offset = index * LIGHT_BYTES;
        const intensity = chunk.lights.getUint8(offset + 4);
        const colorOffset = clientColorOffset(chunk.lights.getUint8(offset + 5));
        const red = CLIENT_COLOR_RGB[colorOffset];
        const green = CLIENT_COLOR_RGB[colorOffset + 1];
        const blue = CLIENT_COLOR_RGB[colorOffset + 2];
        if (!intensity || !(red | green | blue)) continue;
        // The light in grid tiles; tile centers are at x + 0.5.
        const lightX = originX + chunk.lights.getInt16(offset, true) / spriteSize;
        const lightY = originY + chunk.lights.getInt16(offset + 2, true) / spriteSize;
        addLight(pixels, grid, lightX, lightY, intensity, red, green, blue);
      }
    }
    for (const light of extra ?? []) {
      const [red, green, blue] = lightColor(light.color);
      if (light.intensity && red | green | blue) addLight(pixels, grid, light.x - shift - grid.x, light.y - shift - grid.y, light.intensity, red, green, blue);
    }
  }
}

/** Brightens the tiles a light reaches: full within intensity - 5 tiles, fading out to intensity tiles (client: LightView::updatePixels). */
function addLight(pixels: Uint8Array, grid: LightGrid, lightX: number, lightY: number, intensity: number, red: number, green: number, blue: number): void {
  const x0 = Math.max(0, Math.floor(lightX - intensity - 0.5));
  const x1 = Math.min(grid.width - 1, Math.ceil(lightX + intensity - 0.5));
  const y0 = Math.max(0, Math.floor(lightY - intensity - 0.5));
  const y1 = Math.min(grid.height - 1, Math.ceil(lightY + intensity - 0.5));
  for (let y = y0; y <= y1; y++) {
    const dy = y + 0.5 - lightY;
    for (let x = x0; x <= x1; x++) {
      const dx = x + 0.5 - lightX;
      const distance = Math.sqrt(dx * dx + dy * dy);
      if (distance > intensity) continue;
      const strength = (intensity - distance) * FALLOFF_PER_TILE;
      if (strength < MIN_LIGHT) continue;
      const factor = Math.min(strength, 1);
      const offset = (y * grid.width + x) * 4;
      pixels[offset] = Math.max(pixels[offset], Math.trunc(red * factor));
      pixels[offset + 1] = Math.max(pixels[offset + 1], Math.trunc(green * factor));
      pixels[offset + 2] = Math.max(pixels[offset + 2], Math.trunc(blue * factor));
    }
  }
}
