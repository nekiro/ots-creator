// Canvas-style panning of a centered view: hold space and drag, drag with
// the middle button, or use the wheel (shift: sideways, ctrl: zoom). The
// owner positions its content at the stage center moved by `x`/`y`.

/** Pixels of the content that always stay inside the stage. */
const KEEP = 32;

function typing(event: KeyboardEvent): boolean {
  const target = event.target as HTMLElement | null;
  return !!target && (target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName));
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
    const maxX = Math.max(0, (w + sw) / 2 - KEEP);
    const maxY = Math.max(0, (h + sh) / 2 - KEEP);
    this.x = Math.round(Math.min(maxX, Math.max(-maxX, x)));
    this.y = Math.round(Math.min(maxY, Math.max(-maxY, y)));
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
  key = (event: KeyboardEvent): void => {
    if (event.code !== "Space" || typing(event)) return;
    if (event.type === "keyup") {
      if (this.ready) event.preventDefault(); // a focused button must not click
      this.ready = false;
    } else if (this.over || this.ready) {
      event.preventDefault();
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
    onwheel: (event: WheelEvent) => {
      event.preventDefault();
      if (event.ctrlKey) {
        this.onzoom?.(event.deltaY < 0 ? 1 : -1);
        return;
      }
      const [dx, dy] = event.shiftKey ? [event.deltaY, event.deltaX] : [event.deltaX, event.deltaY];
      this.set(this.x - dx, this.y - dy);
    },
    onpointerdown: (event: PointerEvent) => {
      if (!(this.ready && event.button === 0) && event.button !== 1) return;
      event.preventDefault();
      this.stage = event.currentTarget as HTMLElement;
      this.stage.setPointerCapture(event.pointerId);
      this.drag = { id: event.pointerId, x: event.clientX, y: event.clientY, px: this.x, py: this.y };
      this.dragging = true;
    },
    onpointermove: (event: PointerEvent) => {
      const drag = this.drag;
      if (drag?.id !== event.pointerId) return;
      this.set(drag.px + event.clientX - drag.x, drag.py + event.clientY - drag.y);
    },
    onpointerup: (event: PointerEvent) => this.end(event),
    onpointercancel: (event: PointerEvent) => this.end(event),
  };

  private end(event: PointerEvent): void {
    if (this.drag?.id !== event.pointerId) return;
    this.drag = null;
    this.dragging = false;
  }
}

/** Zoom steps of image views: fine below 1x, whole pixels above 2x. */
export const ZOOM_LEVELS = [0.1, 0.15, 0.2, 0.25, 0.33, 0.5, 0.67, 0.75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10, 12, 16];

/** The next zoom level from zoom in a direction (+1 in, -1 out). */
export function stepZoom(zoom: number, direction: number): number {
  const epsilon = 1e-6;
  if (direction > 0) return ZOOM_LEVELS.find((level) => level > zoom + epsilon) ?? ZOOM_LEVELS[ZOOM_LEVELS.length - 1];
  return [...ZOOM_LEVELS].reverse().find((level) => level < zoom - epsilon) ?? ZOOM_LEVELS[0];
}

/** "3×" from 1x up, "50%" below. */
export function zoomLabel(zoom: number): string {
  return zoom >= 1 ? `${Math.round(zoom * 100) / 100}×` : `${Math.round(zoom * 100)}%`;
}

/**
 * Zoom that fits content of w x h into a stage of sw x sh (minus a margin):
 * whole steps when it is 1x or more, so pixels stay even.
 */
export function fitZoom(w: number, h: number, sw: number, sh: number, max = 8, margin = 16): number {
  if (!w || !h || !sw || !sh) return 1;
  const ratio = Math.min((sw - margin) / w, (sh - margin) / h);
  return ratio >= 1 ? Math.min(max, Math.floor(ratio)) : Math.max(ZOOM_LEVELS[0], ratio);
}
