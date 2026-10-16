// Checks and dialog filters shared by the commands.
import { formatSize } from "../api";
import { ask } from "../confirm.svelte";
import { prefs } from "../state/prefs.svelte";
import { app, toast } from "../state/app.svelte";

export const OBJECT_FILTERS = [
  { name: "Object files (*.otobj, *.obd)", pattern: "*.otobj;*.obd" },
  { name: "OTS Creator objects (*.otobj)", pattern: "*.otobj" },
  { name: "Object Builder Data (*.obd)", pattern: "*.obd" },
];

export const IMAGE_FILTERS = [{ name: "Images (*.png, *.bmp, *.gif, *.jpg)", pattern: "*.png;*.bmp;*.gif;*.jpg;*.jpeg" }];

/** Save dialog filters with the preferred format first. */
export function imageFilters(preferred = prefs.settings.exportFormat) {
  const all = [
    { value: "png", filter: { name: "PNG image (*.png)", pattern: "*.png" } },
    { value: "bmp", filter: { name: "Bitmap (*.bmp)", pattern: "*.bmp" } },
    { value: "jpg", filter: { name: "JPEG image (*.jpg)", pattern: "*.jpg" } },
  ];
  return [...all.filter((entry) => entry.value === preferred), ...all.filter((entry) => entry.value !== preferred)].map((entry) => entry.filter);
}

/** A deep copy of plain data. */
export const snapshot = <T>(value: T): T => JSON.parse(JSON.stringify(value));

/** False (with a hint) when no client is open. */
export function guard(): boolean {
  if (!app.open) {
    toast("Open or create a client first.");
    return false;
  }
  return true;
}

export async function confirmDiscard(): Promise<boolean> {
  if (!app.dirty) return true;
  return ask({ title: "Unapplied changes", message: "The selected object has unapplied changes. Discard them?", ok: "Discard" });
}

/** Asks before the open client is replaced or closed with unsaved work. */
export async function confirmLeave(title: string, ok: string): Promise<boolean> {
  if (!(await confirmDiscard())) return false;
  if (!app.project?.info.changed) return true;
  return ask({ title, message: "The client has uncompiled changes. Continue anyway?", ok });
}

/** ", saves 12.3 MB" for a spr file; asset folders free space when compiled. */
export const savedNote = (bytes: number) => (bytes > 0 ? `, saves ${formatSize(bytes)}` : "");

/** Text put on the system clipboard by Copy object, so a later paste does
 * not pick up an image copied before it. */
export const OBJECT_CLIP = "OTS Creator object";
