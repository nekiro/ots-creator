// Pixel-perfect UI scaling. Pixel fonts and skin textures only look right
// when one CSS pixel maps to a whole number of screen pixels. With Windows
// scaling at 125% or 150% we zoom the webview so the effective scale snaps
// to an integer (e.g. 150% -> 200%, 125% -> 100%).
//
// Wails clamps the webview zoom to >= 1, so shrinking (e.g. 1x on a 200%
// screen) uses CSS zoom on the root element instead. Code that turns mouse
// coordinates into CSS pixels goes through toCss().

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

/**
 * @param maxScale largest whole scale whose layout minimum fits the screen
 *   (see fittingScale). Auto stays within it, so it never cuts the window
 *   off; an explicit scale is kept as chosen.
 */
export function zoomFor(systemScale: number, mode: ScaleMode, maxScale = Infinity): number {
	if (!(systemScale > 0) || mode === "system") return 1;
	const target = mode === "auto" ? Math.max(1, Math.min(Math.round(systemScale), maxScale)) : mode;
	return target / systemScale;
}

/** Largest whole scale at which a layout of minimum size fits a work area in
 * screen pixels; at least 1. A 1080p screen fits 1x only, so 2x there would
 * show 960 CSS pixels and cut off the panels. */
export function fittingScale(
	area: { width: number; height: number } | null,
	minimum: { width: number; height: number },
): number {
	if (!area || !(area.width > 0) || !(area.height > 0)) return Infinity;
	return Math.max(1, Math.floor(Math.min(area.width / minimum.width, area.height / minimum.height)));
}

function loadMode(): ScaleMode {
	try {
		const stored = localStorage.getItem(KEY);
		if (stored === "system" || stored === "auto") return stored;
		const scale = Number(stored);
		if (scale === 1 || scale === 2 || scale === 3) return scale;
	} catch {
		/* storage unavailable */
	}
	return "auto";
}

let systemScale = 0;
let current: ScaleMode = "auto";
let rootZoom = 1;
/** Work area of the window's screen in screen pixels, null outside Wails. */
let screenArea: { width: number; height: number } | null = null;

function currentZoom(): number {
	return zoomFor(systemScale, current, fittingScale(screenArea, MIN_LAYOUT));
}

/** Whether the layout fits the window's screen at a whole scale. */
export function scaleFits(scale: number): boolean {
	return scale <= fittingScale(screenArea, MIN_LAYOUT);
}

async function readScreen(): Promise<void> {
	try {
		const screen = await Window.GetScreen();
		const area = screen?.PhysicalWorkArea;
		screenArea = area?.Width && area?.Height ? { width: area.Width, height: area.Height } : null;
	} catch {
		screenArea = null;
	}
}

export function toCss(distance: number): number {
	return distance / rootZoom;
}

export function splitZoom(zoom: number): { webview: number; css: number } {
	return zoom >= 1 ? { webview: zoom, css: 1 } : { webview: 1, css: zoom };
}

/** Smallest layout (CSS pixels) where every panel still fits; App.svelte
 * updates the width when the object list width changes. */
export const MIN_LAYOUT = { width: 1040, height: 600 };

export function setMinLayoutWidth(width: number): void {
	if (width === MIN_LAYOUT.width) return;
	const before = currentZoom();
	MIN_LAYOUT.width = width;
	// A wider object list can stop the scale from fitting the screen.
	void (currentZoom() === before ? applyMinSize() : apply());
}

async function applyMinSize(): Promise<void> {
	const z = currentZoom();
	try {
		await Window.SetMinSize(Math.ceil(MIN_LAYOUT.width * z), Math.ceil(MIN_LAYOUT.height * z));
	} catch {
		/* not running inside Wails */
	}
}

async function apply(): Promise<void> {
	const z = currentZoom();
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

export async function initPixelScale(): Promise<void> {
	current = loadMode();
	let zoom = 1;
	try {
		zoom = (await Window.GetZoom()) || 1;
	} catch {
		/* not in Wails */
	}
	systemScale = window.devicePixelRatio / zoom;
	await readScreen();
	await apply();

	const watch = () => {
		const media = window.matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`);
		media.addEventListener(
			"change",
			async () => {
				const webviewZoom = splitZoom(currentZoom()).webview;
				const next = window.devicePixelRatio / webviewZoom;
				if (Math.abs(next - systemScale) > 0.01) {
					systemScale = next;
					await readScreen(); // moved to another screen
					await apply();
				}
				watch();
			},
			{ once: true },
		);
	};
	watch();
}
