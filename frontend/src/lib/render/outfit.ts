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
    const v = Math.floor(i * 0xff);
    return (v << 16) | (v << 8) | v;
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
  const rgb = (c: number) => {
    const v = hsiToRgb(c);
    return [(v >> 16) & 0xff, (v >> 8) & 0xff, v & 0xff];
  };
  const head = rgb(colors.head);
  const body = rgb(colors.body);
  const legs = rgb(colors.legs);
  const feet = rgb(colors.feet);
  for (let p = 0; p < base.length; p += 4) {
    if (template[p + 3] === 0 || base[p + 3] === 0) continue;
    const r = template[p] > 0;
    const g = template[p + 1] > 0;
    const b = template[p + 2] > 0;
    let c: number[] | null = null;
    if (r && g && !b) c = head;
    else if (r && !g && !b) c = body;
    else if (!r && g && !b) c = legs;
    else if (!r && !g && b) c = feet;
    if (!c) continue;
    base[p] = (base[p] * c[0]) / 255;
    base[p + 1] = (base[p + 1] * c[1]) / 255;
    base[p + 2] = (base[p + 2] * c[2]) / 255;
  }
}
