// Names and health bars over creatures in the play view, laid out as the
// voidcores client does (Creature::drawInformation): in screen pixels at a
// fixed size, kept inside the view.

/** Name colors (Creature::setHealthPercent, drawInformation): NPCs too take the health color. */
export const INFO_COLORS = {
  /** Above 92 % health. */
  fullHealth: "#00bc00",
  /** Creatures covered by a floor above. */
  covered: "#606060",
} as const;

/** Bar colors (rgba 0..1). */
export const BAR_COLORS = {
  background: [0, 0, 0, 1],
  health: [0, 0xbc / 255, 0, 1],
  covered: [0x60 / 255, 0x60 / 255, 0x60 / 255, 1],
  mana: [0, 0, 1, 1],
} as const;

/** Sizes in screen pixels. */
const BAR_WIDTH = 27;
const BAR_HEIGHT = 4;
/** The name's top is this far over the bar. */
const NAME_OVER_BAR = 12;
/** Height of the name font (verdana-11px-rounded). */
const NAME_HEIGHT = 16;
/** Bar and name are kept this far apart when the view's edge pushes one (the local player's info is taller). */
const SPREAD = 12;
const PLAYER_SPREAD = 24;

export interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface InfoLayout {
  /** The bar's black background; the health bar is inside it, 1 pixel in. */
  bar: Rect;
  /** Top of the name. */
  nameTop: number;
}

/**
 * Where the bar and the name of a creature go: (x, y) is the point over
 * the creature (its tile's top-left plus 16 - displacement, minus
 * displacement + 2, both raised by elevation), px the device pixels of a
 * screen pixel, view the map's size in device pixels.
 */
export function infoLayout(x: number, y: number, px: number, viewWidth: number, viewHeight: number, isLocalPlayer: boolean): InfoLayout {
  const bar: Rect = { x: Math.trunc(x - (BAR_WIDTH / 2) * px), y: Math.trunc(y), width: BAR_WIDTH * px, height: BAR_HEIGHT * px };
  // Rect::bind: inside the view.
  bar.x = Math.min(Math.max(bar.x, 0), viewWidth - bar.width);
  bar.y = Math.min(Math.max(bar.y, 0), viewHeight - bar.height);
  let nameTop = Math.min(Math.max(Math.trunc(y - NAME_OVER_BAR * px), 0), viewHeight - NAME_HEIGHT * px);
  const spread = (isLocalPlayer ? PLAYER_SPREAD : SPREAD) * px;
  if (nameTop === 0) bar.y = nameTop + spread;
  if (bar.y + bar.height === viewHeight) nameTop = bar.y - spread;
  return { bar, nameTop };
}

/** The health bar inside the background: 1 pixel in, its width by health. */
export function healthRect(bar: Rect, px: number, healthPercent: number): Rect {
  return { x: bar.x + px, y: bar.y + px, width: Math.round((healthPercent / 100) * (BAR_WIDTH - 2) * px), height: bar.height - 2 * px };
}

/** The local player's mana bar: under the health bar, overlapping its last row. */
export function manaBar(bar: Rect, px: number): Rect {
  return { ...bar, y: bar.y + bar.height - px };
}

/** The name's letters stand on this row under its top: the font's
 * letters end on row 11 of their 16 pixel cell, drawn 2 pixels up
 * (verdana-11px-rounded y-offset), so their outline ends 2 pixels over the
 * bar, a row free. */
export const NAME_BASELINE = 10;
