// Names and health bars over creatures in the play view, laid out as the
// voidcores client does (Creature::drawInformation): in screen pixels at a
// fixed size, kept inside the view, dimmed by the light of the creature's
// field.

/** Name and health bar colors, rgb (Creature::setHealthPercent, drawInformation): NPCs too take the health color. */
export const INFO_COLORS = {
  /** Above 92 % health. */
  fullHealth: [0, 0xbc, 0],
  /** Creatures covered by a floor above: light grey. */
  covered: [192, 192, 192],
} as const;

/** Bar colors (rgba 0..1); the health bar takes the name's color. */
export const BAR_COLORS = {
  background: [0, 0, 0, 1],
  mana: [0, 0, 1, 1],
} as const;

/** Sizes in screen pixels. */
const BAR_WIDTH = 27;
const BAR_HEIGHT = 4;
/** The bar's left is this far left of the point over the creature, and the name centered this far right of it. */
const BAR_CENTER = 14;
/** The name's top is this far over the bar. */
const NAME_OVER_BAR = 12;
/** What the view's edge leaves under the bar: with the name, without it, and the player's mana bar. */
const UNDER_BAR_WITH_NAME = 17;
const UNDER_BAR = 5;
const MANA_BARS = 6;

export interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface InfoLayout {
  /** The bar's black background; the health bar is inside it, 1 pixel in. */
  bar: Rect;
  nameTop: number;
  nameCenter: number;
}

export interface InfoParts {
  names: boolean;
  bars: boolean;
  manaBar: boolean;
}

/**
 * Where the bar and the name of a creature go: (x, y) is the point over
 * the creature (its tile's top-left plus 16 - displacement, minus
 * displacement + 2, both raised by elevation), px the device pixels of a
 * screen pixel, view the map's size in device pixels. At the view's edge
 * the bar stays a pixel inside and keeps room for the name over it and
 * the player's mana bar under it.
 */
export function infoLayout(x: number, y: number, px: number, viewWidth: number, viewHeight: number, isLocalPlayer: boolean, parts: InfoParts): InfoLayout {
  const bar: Rect = { x: Math.trunc(x - BAR_CENTER * px), y: Math.trunc(y), width: BAR_WIDTH * px, height: BAR_HEIGHT * px };
  const manaBars = parts.bars && parts.manaBar && isLocalPlayer ? MANA_BARS : 0;
  const minBarY = (parts.names ? NAME_OVER_BAR + 1 : 1) * px;
  const maxBarY = viewHeight - ((parts.names ? UNDER_BAR_WITH_NAME : UNDER_BAR) + manaBars) * px;
  bar.x = clamp(bar.x, px, Math.max(px, viewWidth - (BAR_WIDTH + 1) * px));
  bar.y = clamp(bar.y, minBarY, Math.max(minBarY, maxBarY));
  return { bar, nameTop: bar.y - NAME_OVER_BAR * px, nameCenter: bar.x + BAR_CENTER * px };
}

export function nameLeft(center: number, nameWidth: number, px: number, viewWidth: number): number {
  return clamp(center - Math.trunc(nameWidth / 2), px, Math.max(px, viewWidth - nameWidth - px));
}

const clamp = (value: number, low: number, high: number) => Math.min(Math.max(value, low), high);

/**
 * The color of a creature's name and health bar in the light of its field
 * (brightness: the red of the field's light, 0 to 255); the player sees
 * itself in at least its own light, 2 to 5 levels of 51 (client:
 * Creature::drawInformation).
 */
export function litInfoColor(rgb: readonly [number, number, number], brightness: number, playerLight?: number): [number, number, number] {
  if (playerLight !== undefined) brightness = Math.max(brightness, clamp(playerLight, 2, 5) * 51);
  const level = Math.trunc(brightness / 5);
  return [Math.trunc((level * rgb[0]) / 51), Math.trunc((level * rgb[1]) / 51), Math.trunc((level * rgb[2]) / 51)];
}

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
