// Sprite sheets of the focused object: export, import, copy and paste.
import { CATEGORY_NAMES, DialogService, ThingService, encodeBytes, errorMessage, res } from "../api";
import { app, applyDraft, run, toast, versions } from "../state.svelte";
import { IMAGE_FILTERS, guard, imageFilters } from "./shared";

async function importSheetWith(load: (id: number) => Promise<void>): Promise<void> {
  if (!guard() || app.focused === null) return;
  if (app.dirty) {
    await applyDraft();
    if (app.dirty) return; // the edit was rejected
  }
  const id = app.focused;
  const ok = await run("Importing", async () => {
    await load(id);
    return true;
  });
  if (ok) toast("Sprite sheet imported.", "success");
}

export const sheetCommands = {
  /** Opens the sprite sheet export window with a preview. */
  exportSheet(group = 0) {
    if (!guard() || app.focused === null) return;
    app.sheetGroup = group;
    app.dialog = "sheet";
  },

  /** Asks where to save and writes the sheet in the chosen format. */
  async saveSheet(group: number, format: string, transparent: boolean): Promise<boolean> {
    if (!guard() || app.focused === null) return false;
    const name = `${CATEGORY_NAMES[app.category]}_${app.focused}.${format}`;
    const path = await DialogService.SaveFile("exportSheet", "Export sprite sheet", name, imageFilters(format));
    if (!path) return false;
    const ok = await run("Exporting", async () => {
      await ThingService.ExportSheet(app.category, app.focused!, group, path, transparent && format === "png");
      return true;
    });
    if (ok) toast(`Sprite sheet saved to ${path}.`, "success");
    return !!ok;
  },

  async importSheet(group = 0) {
    if (!guard() || app.focused === null) return;
    const paths = await DialogService.OpenFiles("importSheet", "Import sprite sheet", IMAGE_FILTERS, false);
    if (paths?.length) await sheetCommands.loadSheet(paths[0], group);
  },

  /** Puts a sheet image into the focused object. A sheet with the size of
   * the group's layout (the edited one: unapplied changes are applied
   * first, like ObjectBuilder) fills it; any other sheet sets the layout
   * from the image. */
  async loadSheet(path: string, group = 0) {
    await importSheetWith((id) => ThingService.ImportSheet(app.category, id, group, path));
  },

  /** loadSheet for image bytes from the clipboard. */
  async pasteSheet(data: Uint8Array, group = 0) {
    await importSheetWith((id) => ThingService.PasteSheet(app.category, id, group, encodeBytes(data)));
  },

  /** Puts the sprite sheet of the focused object on the clipboard as a
   * PNG with transparency; Ctrl+V on another object pastes it there. */
  async copySheet(group = 0) {
    if (!guard() || app.focused === null) return;
    const category = app.category;
    const id = app.focused;
    try {
      const response = await fetch(res.sheet(category, id, group, true, versions.thing(category, id)));
      if (!response.ok) throw new Error((await response.text()) || response.statusText);
      const blob = await response.blob();
      await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
      toast(`Copied the sprite sheet of ${CATEGORY_NAMES[category]} #${id}.`);
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  },
};
