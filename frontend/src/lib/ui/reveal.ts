/**
 * Scrolls `list` so that `element` is visible. Unlike scrollIntoView it scrolls
 * only the list, never its scrollable ancestors, so a popup inside a dialog
 * does not shift the dialog. `list` must be the offsetParent of `element`
 * (positioned).
 */
export function reveal(list: HTMLElement | undefined | null, element: HTMLElement | null | undefined): void {
  if (!list || !element) return;
  const top = element.offsetTop;
  const bottom = top + element.offsetHeight;
  if (top < list.scrollTop) list.scrollTop = top;
  else if (bottom > list.scrollTop + list.clientHeight) list.scrollTop = bottom - list.clientHeight;
}
