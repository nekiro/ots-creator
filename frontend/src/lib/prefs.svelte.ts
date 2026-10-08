// User preferences and recent clients, stored by the backend
// (SettingsService) in the user's config directory.
import { Format, SettingsService, errorMessage, type Recent, type Settings } from "./api";
import { toast } from "./state.svelte";

export const IMAGE_FORMATS = [
  { value: "png", label: "PNG" },
  { value: "bmp", label: "BMP" },
  { value: "jpg", label: "JPG" },
];

export const prefs = $state<{ settings: Settings }>({
  settings: { checkUpdates: true, reopenLast: false, sheetBackground: "magenta", exportFormat: "png", listColumns: 7, recent: [] },
});

/** Objects per row in the object list (matches internal/settings). */
export const LIST_COLUMNS = { min: 6, max: 12 };

/** Object list cell geometry, shared with ThingList. */
export const LIST_CELL = { width: 36, gap: 2 };

/** Width of the object list panel that fits `columns` cells next to the
 * scrollbar (36px is the panel frame, padding and scrollbar). */
export function listPanelWidth(columns: number): number {
  return columns * (LIST_CELL.width + LIST_CELL.gap) - LIST_CELL.gap + 36;
}

export async function loadPrefs(): Promise<void> {
  try {
    const s = await SettingsService.Get();
    prefs.settings = { ...s, recent: s.recent ?? [] };
  } catch {
    /* not running inside Wails: keep defaults */
  }
}

/** Saves preference changes; the recent list is managed by the backend. */
export async function savePrefs(change: Partial<Omit<Settings, "recent">>): Promise<void> {
  const next = { ...prefs.settings, ...change };
  try {
    await SettingsService.Update(next);
    prefs.settings = next;
  } catch (e) {
    toast(errorMessage(e), "error");
  }
}

export async function removeRecent(r: Recent | null): Promise<void> {
  try {
    await SettingsService.RemoveRecent(r?.datPath ?? "");
  } finally {
    await loadPrefs();
  }
}

/** Keeps the end of a long path: "…\client\10.98". */
export function shortPath(path: string, max: number): string {
  return path.length > max ? "…" + path.slice(path.length - max + 1) : path;
}

/** Folder of a recent client, for display (an asset folder is its own path). */
export function recentDir(r: Recent): string {
  if (r.format === Format.FormatAssets) return r.datPath;
  return r.datPath.replace(/[\\/][^\\/]*$/, "");
}
