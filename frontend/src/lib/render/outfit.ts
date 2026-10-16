// Outfit colors: the 133 color HSI palette of the client and template
// colorization (yellow = head, red = body, green = legs, blue = feet).

const STEPS = 19;
const VALUES = 7;
export const PALETTE_SIZE = STEPS * VALUES;

/** Converts a palette index (0..132) to 0xRRGGBB. Port of ColorUtils.HSItoRGB. */
export function hsiToRgb(color: number): number {
  if (color >= PALETTE_SIZE || color < 0) color = 0;
  let hue = 0,
    saturation = 0,
    intensity = 0;
  if (color % STEPS === 0) {
    intensity = 1 - color / STEPS / VALUES;
  } else {
    hue = (color % STEPS) * (1 / 18);
    saturation = 1;
    intensity = 1;
    switch (Math.floor(color / STEPS)) {
      case 0: saturation = 0.25; intensity = 1; break;
      case 1: saturation = 0.25; intensity = 0.75; break;
      case 2: saturation = 0.5; intensity = 0.75; break;
      case 3: saturation = 0.667; intensity = 0.75; break;
      case 4: saturation = 1; intensity = 1; break;
      case 5: saturation = 1; intensity = 0.75; break;
      case 6: saturation = 1; intensity = 0.5; break;
    }
  }
  if (intensity === 0) return 0;
  if (saturation === 0) {
    const gray = Math.floor(intensity * 0xff);
    return (gray << 16) | (gray << 8) | gray;
  }
  let red = 0,
    green = 0,
    blue = 0;
  if (hue < 1 / 6) {
    red = intensity; blue = intensity * (1 - saturation); green = blue + (intensity - blue) * 6 * hue;
  } else if (hue < 2 / 6) {
    green = intensity; blue = intensity * (1 - saturation); red = green - (intensity - blue) * (6 * hue - 1);
  } else if (hue < 3 / 6) {
    green = intensity; red = intensity * (1 - saturation); blue = red + (intensity - red) * (6 * hue - 2);
  } else if (hue < 4 / 6) {
    blue = intensity; red = intensity * (1 - saturation); green = blue - (intensity - red) * (6 * hue - 3);
  } else if (hue < 5 / 6) {
    blue = intensity; green = intensity * (1 - saturation); red = green + (intensity - green) * (6 * hue - 4);
  } else {
    red = intensity; green = intensity * (1 - saturation); blue = red - (intensity - green) * (6 * hue - 5);
  }
  return ((Math.floor(red * 0xff) & 0xff) << 16) | ((Math.floor(green * 0xff) & 0xff) << 8) | (Math.floor(blue * 0xff) & 0xff);
}

export interface OutfitColors {
  head: number;
  body: number;
  legs: number;
  feet: number;
}

export const DEFAULT_COLORS: OutfitColors = { head: 78, body: 69, legs: 58, feet: 76 };

/**
 * Multiplies base pixels by outfit colors where the template layer is
 * marked. base and template are RGBA buffers of the same size; base is
 * modified in place.
 */
export function colorize(base: Uint8ClampedArray, template: Uint8ClampedArray, colors: OutfitColors): void {
  const rgb = (color: number) => {
    const packed = hsiToRgb(color);
    return [(packed >> 16) & 0xff, (packed >> 8) & 0xff, packed & 0xff];
  };
  const head = rgb(colors.head);
  const body = rgb(colors.body);
  const legs = rgb(colors.legs);
  const feet = rgb(colors.feet);
  for (let offset = 0; offset < base.length; offset += 4) {
    if (template[offset + 3] === 0 || base[offset + 3] === 0) continue;
    const red = template[offset] > 0;
    const green = template[offset + 1] > 0;
    const blue = template[offset + 2] > 0;
    let multiplier: number[] | null = null;
    if (red && green && !blue) multiplier = head;
    else if (red && !green && !blue) multiplier = body;
    else if (!red && green && !blue) multiplier = legs;
    else if (!red && !green && blue) multiplier = feet;
    if (!multiplier) continue;
    base[offset] = (base[offset] * multiplier[0]) / 255;
    base[offset + 1] = (base[offset + 1] * multiplier[1]) / 255;
    base[offset + 2] = (base[offset + 2] * multiplier[2]) / 255;
  }
}
