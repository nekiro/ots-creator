// Outfit colors: the 133 color HSI palette of the client and template
// colorization (yellow = head, red = body, green = legs, blue = feet).

const STEPS = 19;
const VALUES = 7;
export const PALETTE_SIZE = STEPS * VALUES;

/** Converts a palette index (0..132) to 0xRRGGBB. Port of ColorUtils.HSItoRGB. */
export function hsiToRgb(color: number): number {
  if (color >= PALETTE_SIZE || color < 0) color = 0;
  let h = 0,
    s = 0,
    i = 0;
  if (color % STEPS === 0) {
    i = 1 - color / STEPS / VALUES;
  } else {
    h = (color % STEPS) * (1 / 18);
    s = 1;
    i = 1;
    switch (Math.floor(color / STEPS)) {
      case 0: s = 0.25; i = 1; break;
      case 1: s = 0.25; i = 0.75; break;
      case 2: s = 0.5; i = 0.75; break;
      case 3: s = 0.667; i = 0.75; break;
      case 4: s = 1; i = 1; break;
      case 5: s = 1; i = 0.75; break;
      case 6: s = 1; i = 0.5; break;
    }
  }
  if (i === 0) return 0;
  if (s === 0) {
    const gray = Math.floor(i * 0xff);
    return (gray << 16) | (gray << 8) | gray;
  }
  let r = 0,
    g = 0,
    b = 0;
  if (h < 1 / 6) {
    r = i; b = i * (1 - s); g = b + (i - b) * 6 * h;
  } else if (h < 2 / 6) {
    g = i; b = i * (1 - s); r = g - (i - b) * (6 * h - 1);
  } else if (h < 3 / 6) {
    g = i; r = i * (1 - s); b = r + (i - r) * (6 * h - 2);
  } else if (h < 4 / 6) {
    b = i; r = i * (1 - s); g = b - (i - r) * (6 * h - 3);
  } else if (h < 5 / 6) {
    b = i; g = i * (1 - s); r = g + (i - g) * (6 * h - 4);
  } else {
    r = i; g = i * (1 - s); b = r - (i - g) * (6 * h - 5);
  }
  return ((Math.floor(r * 0xff) & 0xff) << 16) | ((Math.floor(g * 0xff) & 0xff) << 8) | (Math.floor(b * 0xff) & 0xff);
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
