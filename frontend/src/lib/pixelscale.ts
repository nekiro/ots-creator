// Pixel-perfect UI scaling. Pixel fonts and skin textures only look right
// when one CSS pixel maps to a whole number of screen pixels. With Windows
// scaling at 125% or 150% we zoom the webview so the effective scale snaps
// to an integer (e.g. 150% -> 200%, 125% -> 100%).
//
// Wails clamps the webview zoom to >= 1, so shrinking (e.g. 1x on a 200%
// screen) uses CSS zoom on the root element instead. Code that turns mouse
// coordinates into CSS pixels divides by cssZoom().

import { Window } from "@wailsio/runtime";

export type ScaleMode = "system" | "auto" | 1 | 2 | 3;

export const SCALE_MODES: { value: ScaleMode; label: string }[] = [
  { value: "auto", label: "Pixel-perfect (auto)" },
  { value: 1, label: "Pixel-perfect 1x" },
  { value: 2, label: "Pixel-perfect 2x" },
  { value: 3, label: "Pixel-perfect 3x" },
  { value: "system", label: "System scaling" },
];

const KEY = "ui-scale";

/** Webview zoom that turns the system scale into the requested integer scale. */
export function zoomFor(systemScale: number, mode: ScaleMode): number {
  if (!(systemScale > 0) || mode === "system") return 1;
  const target = mode === "auto" ? Math.max(1, Math.round(systemScale)) : mode;
  return target / systemScale;
}

export function loadMode(): ScaleMode {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "system" || v === "auto") return v;
    const n = Number(v);
    if (n === 1 || n === 2 || n === 3) return n;
  } catch {
    /* storage unavailable */
  }
  return "auto";
}

let systemScale = 0;
let current: ScaleMode = "auto";
let rootZoom = 1;

/** CSS zoom of the page (1 unless the UI is scaled below the system scale). */
export function cssZoom(): number {
  return rootZoom;
}

/** Converts a distance in viewport pixels (mouse events, getBoundingClientRect) to CSS pixels. */
export function toCss(v: number): number {
  return v / rootZoom;
}

/** Splits a zoom into the webview part (>= 1) and the CSS part (<= 1). */
export function splitZoom(z: number): { webview: number; css: number } {
  return z >= 1 ? { webview: z, css: 1 } : { webview: 1, css: z };
}

/** Smallest layout (CSS pixels) where every panel still fits; App.svelte
 * updates the width when the object list width changes. */
export const MIN_LAYOUT = { width: 1040, height: 600 };

export function setMinLayoutWidth(width: number): void {
  if (width === MIN_LAYOUT.width) return;
  MIN_LAYOUT.width = width;
  void applyMinSize();
}

async function applyMinSize(): Promise<void> {
  const z = zoomFor(systemScale, current);
  try {
    await Window.SetMinSize(Math.ceil(MIN_LAYOUT.width * z), Math.ceil(MIN_LAYOUT.height * z));
  } catch {
    /* not running inside Wails */
  }
}

async function apply(): Promise<void> {
  const z = zoomFor(systemScale, current);
  const { webview, css } = splitZoom(z);
  rootZoom = css;
  document.documentElement.style.zoom = css === 1 ? "" : String(css);
  try {
    await Window.SetZoom(webview);
  } catch {
    /* not running inside Wails (tests, server mode) */
  }
  // The window minimum is in screen units: scale it with the UI so a
  // pixel-perfect 2x UI cannot be squeezed below its layout minimum.
  await applyMinSize();
}

export async function setMode(mode: ScaleMode): Promise<void> {
  current = mode;
  try {
    localStorage.setItem(KEY, String(mode));
  } catch {
    /* ignore */
  }
  await apply();
}

export function getMode(): ScaleMode {
  return current;
}

/** Detects the system scale and applies the saved mode; re-applies when the
 * window moves to a monitor with another scale. */
export async function initPixelScale(): Promise<void> {
  current = loadMode();
  let zoom = 1;
  try {
    zoom = (await Window.GetZoom()) || 1;
  } catch {
    /* not in Wails */
  }
  systemScale = window.devicePixelRatio / zoom;
  await apply();

  const watch = () => {
    const mq = window.matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`);
    mq.addEventListener(
      "change",
      async () => {
        const z = splitZoom(zoomFor(systemScale, current)).webview;
        const next = window.devicePixelRatio / z;
        if (Math.abs(next - systemScale) > 0.01) {
          systemScale = next;
          await apply();
        }
        watch();
      },
      { once: true },
    );
  };
  watch();
}
