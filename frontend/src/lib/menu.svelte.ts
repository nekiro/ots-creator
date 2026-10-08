// Menu entries shared by the menu bar and context menus.
import { toCss } from "./pixelscale";

export interface MenuItem {
  label: string;
  keys?: string;
  action: () => unknown;
  disabled?: () => boolean;
  checked?: () => boolean;
}
export type MenuEntry = MenuItem | "-";

/** The open context menu, rendered once by ContextMenu in App. */
export const contextMenu = $state<{ items: MenuEntry[] | null; x: number; y: number }>({ items: null, x: 0, y: 0 });

export function openContextMenu(e: MouseEvent, items: MenuEntry[]): void {
  e.preventDefault();
  e.stopPropagation();
  contextMenu.items = items;
  contextMenu.x = toCss(e.clientX);
  contextMenu.y = toCss(e.clientY);
}

export function closeContextMenu(): void {
  contextMenu.items = null;
}
