// Canvas-style panning of a centered view: hold space and drag, drag with
// the middle button, or use the wheel (shift: sideways, ctrl: zoom). The
// owner positions its content at the stage center moved by `x`/`y`.
import { isTyping } from "./ui/typing";

/** Pixels of the content that always stay inside the stage. */
const KEEP = 32;

export class PanView {
  x = $state(0);
  y = $state(0);
  /** Space is held over the stage: a left drag pans. */
  ready = $state(false);
  dragging = $state(false);

  private over = false;
  private drag: { pointerId: number; clientX: number; clientY: number; startX: number; startY: number } | null = null;
  private stage: HTMLElement | null = null;

  /**
   * @param size content and stage size in pixels, read when clamping
   * @param onzoom ctrl+wheel: +1 or -1
   */
  constructor(
    private size: () => { width: number; height: number; stageWidth: number; stageHeight: number },
    private onzoom?: (delta: number) => void,
  ) {}

  reset(): void {
    this.x = 0;
    this.y = 0;
  }

  /** Moves to x, y, keeping a corner of the content inside the stage. */
  set(x: number, y: number): void {
    const { width, height, stageWidth, stageHeight } = this.size();
    const maxX = Math.max(0, (width + stageWidth) / 2 - KEEP);
    const maxY = Math.max(0, (height + stageHeight) / 2 - KEEP);
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
    if (event.code !== "Space" || isTyping(event)) return;
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
      const [deltaX, deltaY] = event.shiftKey ? [event.deltaY, event.deltaX] : [event.deltaX, event.deltaY];
      this.set(this.x - deltaX, this.y - deltaY);
    },
    onpointerdown: (event: PointerEvent) => {
      if (!(this.ready && event.button === 0) && event.button !== 1) return;
      event.preventDefault();
      this.stage = event.currentTarget as HTMLElement;
      this.stage.setPointerCapture(event.pointerId);
      this.drag = { pointerId: event.pointerId, clientX: event.clientX, clientY: event.clientY, startX: this.x, startY: this.y };
      this.dragging = true;
    },
    onpointermove: (event: PointerEvent) => {
      const drag = this.drag;
      if (drag?.pointerId !== event.pointerId) return;
      this.set(drag.startX + event.clientX - drag.clientX, drag.startY + event.clientY - drag.clientY);
    },
    onpointerup: (event: PointerEvent) => this.end(event),
    onpointercancel: (event: PointerEvent) => this.end(event),
  };

  private end(event: PointerEvent): void {
    if (this.drag?.pointerId !== event.pointerId) return;
    this.drag = null;
    this.dragging = false;
  }
}
