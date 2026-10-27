// The light of the map as the client computes it (OTClient LightView, as
// voidcores makes it work like the 7.72 client): one color per tile, the
// brightest of the ambient light and the lights reaching the tile's
// center. Floors add their lights bottom up; a tile that shades hides the
// light of the floors under it, or dims it to 4/5 when a translucent thing
// is on it. The lights of the floors under the floor in view are dimmed to
// 4/5, and so is a light where its floor has no ground. The map is
// multiplied by it, stretched with linear filtering, so light fades from
// tile to tile.
import { CHUNK, splitKey } from "../data/chunks";
import { CLIENT_COLOR_RGB, clientColorOffset } from "../data/items";
import { GEO_BLOCK, LIGHT_BYTES, type GeoChunk } from "./geo";

/** Ambient light at or above this intensity is full daylight: nothing is darkened (client: LightView::isDark). */
export const DAYLIGHT_INTENSITY = 250;
/** The client's white, and the color of the ambient light. */
export const WHITE_LIGHT_COLOR = 215;
/** Light falls off over this many tiles at its edge (client: intensity * 0.2 per tile). */
const FALLOFF_PER_TILE = 0.2;
const MIN_LIGHT = 0.01;
/** What a dimmed light keeps (client: LightView::updatePixels, 0.8). */
const DIMMED = 0.8;
const SHADE_PLANE = 0;
const TRANSLUCENT_PLANE = 1;
const GROUND_PLANE = 2;
const SHADE_PLANES = 3;

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
  z: number;
  chunks: GeoChunk[];
  shift: number;
  extra?: ExtraLight[];
}

/** A rectangle of tiles on the screen grid (tile positions minus their floor's shift). */
export interface LightGrid {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** A light of the pass, at a grid position in tiles (client: LightView::TileLight). */
interface GridLight {
  x: number;
  y: number;
  intensity: number;
  red: number;
  green: number;
  blue: number;
  floor: number;
}

/** What the floors left on each tile of the grid (client: LightView::TileShade), reused from pass to pass. */
let firstLights = new Uint32Array(0);
let firstUndimmedLights = new Uint32Array(0);
let groundFloors = new Uint16Array(0);

/**
 * Fills pixels (rgba, width x height of the grid) with the light of each
 * tile of the grid. floors run bottom up; viewFloor is the floor in view
 * (client: the camera's), spriteSize the world pixels of a tile.
 */
export function buildLightMap(pixels: Uint8Array, grid: LightGrid, floors: LightFloor[], ambient: number, spriteSize: number, viewFloor: number): void {
  const texels = grid.width * grid.height;
  if (firstLights.length < texels) {
    firstLights = new Uint32Array(texels);
    firstUndimmedLights = new Uint32Array(texels);
    groundFloors = new Uint16Array(texels);
  }
  firstLights.fill(0, 0, texels);
  firstUndimmedLights.fill(0, 0, texels);
  groundFloors.fill(0, 0, texels);
  const lights: GridLight[] = [];
  // Lights before this index come from the floors under the one in view.
  let dimmedLights = 0;
  const tileAt = (gridX: number, gridY: number) => (gridX < 0 || gridY < 0 || gridX >= grid.width || gridY >= grid.height ? -1 : gridY * grid.width + gridX);
  floors.forEach(({ z, chunks, shift, extra }, floorIndex) => {
    if (z === viewFloor && floorIndex > 0) dimmedLights = lights.length;
    // Shades first: the floor's own lights shine over them (client: LightView::resetShade, markGround).
    for (const chunk of chunks) {
      if (!chunk.shades) continue;
      const [chunkX, chunkY] = splitKey(chunk.key % GEO_BLOCK);
      const planeBytes = chunk.shades.length / SHADE_PLANES;
      const size = Math.round(Math.sqrt(planeBytes * 8));
      const originX = chunkX * CHUNK - shift - grid.x;
      const originY = chunkY * CHUNK - shift - grid.y;
      const shades = chunk.shades;
      for (let byte = 0; byte < planeBytes; byte++) {
        const shadeBits = shades[SHADE_PLANE * planeBytes + byte];
        const groundBits = shades[GROUND_PLANE * planeBytes + byte];
        if (!(shadeBits | groundBits)) continue;
        const translucentBits = shades[TRANSLUCENT_PLANE * planeBytes + byte];
        for (let bit = 0; bit < 8; bit++) {
          const mask = 1 << bit;
          if (!((shadeBits | groundBits) & mask)) continue;
          const tile = byte * 8 + bit;
          const texel = tileAt(originX + (tile % size), originY + Math.floor(tile / size));
          if (texel < 0) continue;
          if (shadeBits & mask) {
            firstUndimmedLights[texel] = lights.length;
            if (!(translucentBits & mask)) firstLights[texel] = lights.length;
          }
          if (groundBits & mask) groundFloors[texel] |= 1 << z;
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
        const x = originX + chunk.lights.getInt16(offset, true) / spriteSize;
        const y = originY + chunk.lights.getInt16(offset + 2, true) / spriteSize;
        lights.push({ x, y, intensity, red, green, blue, floor: z });
      }
    }
    for (const light of extra ?? []) {
      const [red, green, blue] = lightColor(light.color);
      if (light.intensity && red | green | blue) lights.push({ x: light.x - shift - grid.x, y: light.y - shift - grid.y, intensity: light.intensity, red, green, blue, floor: z });
    }
  });

  const [ambientRed, ambientGreen, ambientBlue] = lightColor(WHITE_LIGHT_COLOR, ambient / 255);
  for (let texel = 0; texel < texels; texel++) {
    pixels[texel * 4] = ambientRed;
    pixels[texel * 4 + 1] = ambientGreen;
    pixels[texel * 4 + 2] = ambientBlue;
    pixels[texel * 4 + 3] = 255;
  }
  lights.forEach((light, lightIndex) => addLight(pixels, grid, light, lightIndex, lightIndex < dimmedLights));
}

/** Brightens the tiles a light reaches: full within intensity - 5 tiles, fading out to intensity tiles (client: LightView::updatePixels). */
function addLight(pixels: Uint8Array, grid: LightGrid, light: GridLight, lightIndex: number, fromLowerFloor: boolean): void {
  const { x: lightX, y: lightY, intensity } = light;
  const x0 = Math.max(0, Math.floor(lightX - intensity - 0.5));
  const x1 = Math.min(grid.width - 1, Math.ceil(lightX + intensity - 0.5));
  const y0 = Math.max(0, Math.floor(lightY - intensity - 0.5));
  const y1 = Math.min(grid.height - 1, Math.ceil(lightY + intensity - 0.5));
  for (let y = y0; y <= y1; y++) {
    const dy = y + 0.5 - lightY;
    for (let x = x0; x <= x1; x++) {
      const texel = y * grid.width + x;
      if (lightIndex < firstLights[texel]) continue;
      const dx = x + 0.5 - lightX;
      const distance = Math.sqrt(dx * dx + dy * dy);
      if (distance > intensity) continue;
      let strength = (intensity - distance) * FALLOFF_PER_TILE;
      if (strength < MIN_LIGHT) continue;
      if (!(groundFloors[texel] & (1 << light.floor))) strength *= DIMMED;
      strength = Math.min(strength, 1);
      if (lightIndex < firstUndimmedLights[texel]) strength *= DIMMED;
      if (fromLowerFloor) strength *= DIMMED;
      const offset = texel * 4;
      pixels[offset] = Math.max(pixels[offset], Math.trunc(light.red * strength));
      pixels[offset + 1] = Math.max(pixels[offset + 1], Math.trunc(light.green * strength));
      pixels[offset + 2] = Math.max(pixels[offset + 2], Math.trunc(light.blue * strength));
    }
  }
}
