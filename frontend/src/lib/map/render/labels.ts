// Names and texts drawn on a 2D canvas over the map.
import { LABEL_FONT } from "../constants";
import type { Label } from "./scene";

/** Left, top, right, bottom in device pixels. */
export type Box = [number, number, number, number];

export function boxesOverlap(first: Box, second: Box): boolean {
  return first[0] < second[2] && first[2] > second[0] && first[1] < second[3] && first[3] > second[1];
}

/** Offsets of the eight copies that make the 1px black outline of names. */
const OUTLINE = [
  [-1, -1], [0, -1], [1, -1],
  [-1, 0], [1, 0],
  [-1, 1], [0, 1], [1, 1],
];

const NAME_FONT = '"Tibia Verdana", Verdana, sans-serif';
const SMALL_FONT = '"Tibia Small", Verdana, sans-serif';

/** Loads the label fonts; labels drawn before used the fallback. */
export function loadLabelFonts(): Promise<unknown> {
  return Promise.all([document.fonts?.load(`bold ${LABEL_FONT.nameSize}px "Tibia Verdana"`), document.fonts?.load(`${LABEL_FONT.smallSize}px "Tibia Small"`)]);
}

/** Labels under the mouse come last: drawn over the others. */
const topLast = (first: Label, second: Label) => Number(!!first.top) - Number(!!second.top);

const rounded = (label: Label): Label => ({ ...label, x: Math.round(label.x), y: Math.round(label.y) });

/** Device pixels per CSS pixel as a whole number (at least 1): labels scale by it. */
export function wholePixelRatio(devicePixelRatio: number): number {
  return Math.max(1, Math.round(devicePixelRatio || 1));
}

/** Clears the canvas and draws the labels of a frame. */
export function drawLabels(context: CanvasRenderingContext2D, width: number, height: number, labels: Label[], devicePixelRatio: number): void {
  context.clearRect(0, 0, width, height);
  if (!labels.length) return;
  // The client's name font: the pixel font at a whole multiple of its 14px
  // size, with a hard 1px black outline drawn as offset copies.
  const pixel = wholePixelRatio(devicePixelRatio);
  context.textAlign = "center";
  const nameFont = `bold ${LABEL_FONT.nameSize * pixel}px ${NAME_FONT}`;
  const names = labels
    .filter((label) => !label.small)
    .slice(0, LABEL_FONT.maxNames)
    .sort(topLast)
    .map(rounded);
  // Item texts are many and long: one that would cover a name (or an
  // earlier text) is left out.
  const boxes: Box[] = [];
  const pad = 2 * pixel;
  const isFree = (box: Box) => !boxes.some((other) => boxesOverlap(box, other));
  const smallBox = (label: Label, textWidth: number): Box => [label.x - textWidth / 2 - pad, label.y - 6 * pixel, label.x + textWidth / 2 + pad, label.y + 6 * pixel];
  context.font = nameFont;
  for (const label of names) {
    const textWidth = context.measureText(label.text).width;
    if (label.clampX) label.x = Math.round(Math.min(Math.max(label.x, textWidth / 2), width - textWidth / 2));
    boxes.push([label.x - textWidth / 2 - pad, label.y - 15 * pixel, label.x + textWidth / 2 + pad, label.y]);
  }
  // Small labels (tile coordinates, item texts): the small pixel font, many
  // of them, so one stroke for the outline instead of eight copies.
  const small = labels
    .filter((label) => label.small)
    .sort(topLast)
    .map(rounded);
  if (small.length) {
    context.font = `${LABEL_FONT.smallSize * pixel}px ${SMALL_FONT}`;
    for (const label of small) if (label.top) boxes.push(smallBox(label, context.measureText(label.text).width));
    context.textBaseline = "middle";
    context.lineWidth = 2 * pixel;
    context.lineJoin = "round";
    context.strokeStyle = "#000";
    for (const label of small) {
      context.textAlign = label.align ?? "center";
      if (label.cull) {
        const box = smallBox(label, context.measureText(label.text).width);
        if (!isFree(box)) continue;
        boxes.push(box);
      }
      context.strokeText(label.text, label.x, label.y);
      context.fillStyle = label.color;
      context.fillText(label.text, label.x, label.y);
    }
  }
  context.font = nameFont;
  context.textAlign = "center";
  context.textBaseline = "bottom";
  context.fillStyle = "#000";
  for (const label of names) {
    context.textBaseline = label.baseline ? "alphabetic" : "bottom";
    for (const [dx, dy] of OUTLINE) context.fillText(label.text, label.x + dx * pixel, label.y + dy * pixel);
  }
  for (const label of names) {
    context.textBaseline = label.baseline ? "alphabetic" : "bottom";
    context.fillStyle = label.color;
    context.fillText(label.text, label.x, label.y);
  }
}
