// Adding, removing, importing and exporting objects.
import { CATEGORY_NAMES, DialogService, ThingService, minId } from "../api";
import { ask } from "../confirm.svelte";
import { prefs } from "../prefs.svelte";
import { app, run, select, toast } from "../state.svelte";
import { OBJECT_FILTERS, confirmDiscard, guard } from "./shared";

type ImportResults = NonNullable<Awaited<ReturnType<typeof ThingService.ImportObjects>>>;

/** Toasts what an object import did and selects the last imported object. */
export async function reportImported(results: ImportResults): Promise<void> {
  const imported = results.filter((result) => !result.error);
  if (imported.length) toast(`Imported ${imported.length} object(s).`, "success");
  for (const failed of results.filter((result) => result.error)) toast(`${failed.path}: ${failed.error}`, "error");
  const last = imported.at(-1);
  if (last && last.category === app.category) await select(last.id);
}

export const objectCommands = {
  async newThing() {
    if (!guard() || !(await confirmDiscard())) return;
    const id = await run("Creating", () => ThingService.Add(app.category));
    if (id) await select(id);
  },

  async duplicate() {
    if (!guard() || app.selection.length === 0) return;
    const ids = await run("Duplicating", () => ThingService.Duplicate(app.category, app.selection));
    if (ids?.length) await select(ids[0]);
  },

  async remove() {
    if (!guard() || app.selection.length === 0) return;
    // No confirmation: Ctrl+Z brings the objects back.
    const count = app.selection.length;
    const ok = await run("Removing", async () => {
      await ThingService.Remove(app.category, app.selection);
      return true;
    });
    if (ok) toast(`Removed ${count} ${CATEGORY_NAMES[app.category]}(s). Ctrl+Z to undo.`);
  },

  async importObd(replace = false) {
    if (!guard()) return;
    const paths = await DialogService.OpenFiles(replace ? "replaceObject" : "importObjects", replace ? "Replace with object" : "Import objects", OBJECT_FILTERS, !replace);
    if (!paths?.length) return;
    const replaceId = replace ? (app.focused ?? 0) : 0;
    const results = await run("Importing", () => ThingService.ImportObjects(paths, replaceId));
    if (results) await reportImported(results);
  },

  async exportObd() {
    if (!guard() || app.selection.length === 0) return;
    const dir = await DialogService.PickDirectory("exportObjects", "Export objects to");
    if (!dir) return;
    const files = await run("Exporting", () => ThingService.ExportObjects(app.category, app.selection, dir, prefs.settings.objectFormat));
    if (files) toast(`Exported ${files.length} file(s).`, "success");
  },

  /** Exports every object with sprites, one folder per category. */
  async exportAllObjects() {
    if (!guard()) return;
    const dir = await DialogService.PickDirectory("exportAllObjects", "Export all objects to");
    if (!dir) return;
    const summary = await run("Exporting objects", () => ThingService.ExportAll(dir, true, prefs.settings.objectFormat));
    if (summary) toast(`Exported ${summary.files.toLocaleString()} object(s), skipped ${summary.skipped.toLocaleString()} empty.`, "success");
  },

  async convertFrameGroups(toGroups: boolean) {
    if (!guard() || !(await confirmDiscard())) return;
    const message = toGroups
      ? "Split every outfit into an idle group (frame 1) and a walking group (the other frames)? Use this when moving to a client with frame groups."
      : "Merge the idle and walking groups of every outfit into one group (idle frame first)? Use this when moving to a client without frame groups.";
    if (!(await ask({ title: "Convert frame groups", message, ok: "Convert" }))) return;
    const result = await run("Converting", () => ThingService.ConvertFrameGroups(toGroups));
    if (!result) return;
    const skipped = result.skipped ? ` ${result.skipped} skipped: idle and walking layouts differ.` : "";
    toast(`Converted ${result.converted} outfit(s).${skipped}`, result.converted ? "success" : "info");
  },

  /** Opens the share window for the focused object. */
  shareObject() {
    if (!guard() || app.focused === null) return;
    app.shareTarget = { kind: "object", category: app.category, id: app.focused };
    app.dialog = "share";
  },

  /** Opens the share window for the selected sprites. */
  shareSprites() {
    if (!guard() || app.selectedSprites.length === 0) return;
    app.shareTarget = { kind: "sprites", ids: [...app.selectedSprites].sort((first, second) => first - second) };
    app.dialog = "share";
  },

  /** Opens the share window with a picker, starting from the selection. */
  sharePick() {
    app.shareTarget = !app.open
      ? { kind: "file", path: "", pick: true }
      : app.focused === null && app.selectedSprites.length
        ? { kind: "sprites", ids: [...app.selectedSprites].sort((first, second) => first - second), pick: true }
        : { kind: "object", category: app.category, id: app.focused ?? minId(app.category), pick: true };
    app.dialog = "share";
  },
};
