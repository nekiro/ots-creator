// Arrow buttons for scrollbars on WebKit (macOS WKWebView, WebKitGTK), which
// ignores ::-webkit-scrollbar-button. theme.css paints the arrows at the ends
// of the scrollbar; a press there never reaches the native scrollbar, so the
// step scrolling (with auto-repeat while held) is done here.

import { toCss } from "../pixelscale";

const BUTTON = 12;
const STEP = 40;
const REPEAT_DELAY = 300;
const REPEAT_INTERVAL = 50;

export function needsFakeButtons(userAgent: string): boolean {
  return /AppleWebKit/.test(userAgent) && !/Chrome|Chromium|Edg\//.test(userAgent);
}

/** Scroll step for a press at (x, y) inside the element's border box, or
 * null when the point is not on a scrollbar arrow. */
export function arrowAt(
  el: {
    clientLeft: number;
    clientTop: number;
    clientWidth: number;
    clientHeight: number;
    scrollWidth: number;
    scrollHeight: number;
  },
  x: number,
  y: number,
): { dx: number; dy: number } | null {
  x -= el.clientLeft;
  y -= el.clientTop;
  if (x >= el.clientWidth && y < el.clientHeight && el.scrollHeight > el.clientHeight) {
    if (y < BUTTON) return { dx: 0, dy: -STEP };
    if (y >= el.clientHeight - BUTTON) return { dx: 0, dy: STEP };
  }
  if (y >= el.clientHeight && x < el.clientWidth && el.scrollWidth > el.clientWidth) {
    if (x < BUTTON) return { dx: -STEP, dy: 0 };
    if (x >= el.clientWidth - BUTTON) return { dx: STEP, dy: 0 };
  }
  return null;
}

export function initScrollButtons(): void {
  if (!needsFakeButtons(navigator.userAgent)) return;
  document.documentElement.classList.add("sb-fake-buttons");

  let timer: number | undefined;
  const stop = () => {
    clearTimeout(timer);
    clearInterval(timer);
    timer = undefined;
  };

  window.addEventListener(
    "mousedown",
    (e) => {
      stop();
      const el = e.target;
      if (e.button !== 0 || !(el instanceof HTMLElement)) return;
      const rect = el.getBoundingClientRect();
      const step = arrowAt(el, toCss(e.clientX - rect.left), toCss(e.clientY - rect.top));
      if (!step) return;
      e.preventDefault();
      const scroll = () => el.scrollBy(step.dx, step.dy);
      scroll();
      timer = window.setTimeout(() => {
        timer = window.setInterval(scroll, REPEAT_INTERVAL);
      }, REPEAT_DELAY);
    },
    true,
  );
  window.addEventListener("mouseup", stop, true);
  window.addEventListener("blur", stop);
}
