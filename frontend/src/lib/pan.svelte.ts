// Canvas-style panning of a centered view: hold space and drag, drag with
// the middle button, or use the wheel (shift: sideways, ctrl: zoom). The
// owner positions its content at the stage center moved by `x`/`y`.

/** Pixels of the content that always stay inside the stage. */
const KEEP = 32;

function typing(e: KeyboardEvent): boolean {
  const t = e.target as HTMLElement | null;
  return !!t && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
}

export class PanView {
  x = $state(0);
  y = $state(0);
  /** Space is held over the stage: a left drag pans. */
  ready = $state(false);
  dragging = $state(false);

  private over = false;
  private drag: { id: number; x: number; y: number; px: number; py: number } | null = null;
  private stage: HTMLElement | null = null;

  /**
   * @param size content and stage size in pixels, read when clamping
   * @param onzoom ctrl+wheel: +1 or -1
   */
  constructor(
    private size: () => { w: number; h: number; sw: number; sh: number },
    private onzoom?: (delta: number) => void,
  ) {}

  reset(): void {
    this.x = 0;
    this.y = 0;
  }

  /** Moves to x, y, keeping a corner of the content inside the stage. */
  set(x: number, y: number): void {
    const { w, h, sw, sh } = this.size();
    const mx = Math.max(0, (w + sw) / 2 - KEEP);
    const my = Math.max(0, (h + sh) / 2 - KEEP);
    this.x = Math.round(Math.min(mx, Math.max(-mx, x)));
    this.y = Math.round(Math.min(my, Math.max(-my, y)));
  }

  /** Scales the offset for a zoom change, so the centered pixel stays put. */
  rescale(factor: number): void {
    this.set(this.x * factor, this.y * factor);
  }

  /** CSS transform for content placed at left: 50%, top: 50%. */
  get transform(): string {
    return `translate(calc(-50% + ${this.x}px), calc(-50% + ${this.y}px))`;
  }

  /** Attach to the window: space toggles pan mode while over the stage. */
  key = (e: KeyboardEvent): void => {
    if (e.code !== "Space" || typing(e)) return;
    if (e.type === "keyup") {
      if (this.ready) e.preventDefault(); // a focused button must not click
      this.ready = false;
    } else if (this.over || this.ready) {
      e.preventDefault();
      this.ready = true;
    }
  };

  blur = (): void => {
    this.ready = false;
  };

  /** Spread onto the stage element. */
  readonly handlers = {
    onmouseenter: () => (this.over = true),
    onmouseleave: () => (this.over = false),
    onwheel: (e: WheelEvent) => {
      e.preventDefault();
      if (e.ctrlKey) {
        this.onzoom?.(e.deltaY < 0 ? 1 : -1);
        return;
      }
      const [dx, dy] = e.shiftKey ? [e.deltaY, e.deltaX] : [e.deltaX, e.deltaY];
      this.set(this.x - dx, this.y - dy);
    },
    onpointerdown: (e: PointerEvent) => {
      if (!(this.ready && e.button === 0) && e.button !== 1) return;
      e.preventDefault();
      this.stage = e.currentTarget as HTMLElement;
      this.stage.setPointerCapture(e.pointerId);
      this.drag = { id: e.pointerId, x: e.clientX, y: e.clientY, px: this.x, py: this.y };
      this.dragging = true;
    },
    onpointermove: (e: PointerEvent) => {
      const d = this.drag;
      if (d?.id !== e.pointerId) return;
      this.set(d.px + e.clientX - d.x, d.py + e.clientY - d.y);
    },
    onpointerup: (e: PointerEvent) => this.end(e),
    onpointercancel: (e: PointerEvent) => this.end(e),
  };

  private end(e: PointerEvent): void {
    if (this.drag?.id !== e.pointerId) return;
    this.drag = null;
    this.dragging = false;
  }
}

/** Zoom steps of image views: fine below 1x, whole pixels above 2x. */
export const ZOOM_LEVELS = [0.1, 0.15, 0.2, 0.25, 0.33, 0.5, 0.67, 0.75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10, 12, 16];

/** The next zoom level from z in direction d (+1 in, -1 out). */
export function stepZoom(z: number, d: number): number {
  const eps = 1e-6;
  if (d > 0) return ZOOM_LEVELS.find((l) => l > z + eps) ?? ZOOM_LEVELS[ZOOM_LEVELS.length - 1];
  return [...ZOOM_LEVELS].reverse().find((l) => l < z - eps) ?? ZOOM_LEVELS[0];
}

/** "3×" from 1x up, "50%" below. */
export function zoomLabel(z: number): string {
  return z >= 1 ? `${Math.round(z * 100) / 100}×` : `${Math.round(z * 100)}%`;
}

/**
 * Zoom that fits content of w x h into a stage of sw x sh (minus a margin):
 * whole steps when it is 1x or more, so pixels stay even.
 */
export function fitZoom(w: number, h: number, sw: number, sh: number, max = 8, margin = 16): number {
  if (!w || !h || !sw || !sh) return 1;
  const r = Math.min((sw - margin) / w, (sh - margin) / h);
  return r >= 1 ? Math.min(max, Math.floor(r)) : Math.max(ZOOM_LEVELS[0], r);
}
