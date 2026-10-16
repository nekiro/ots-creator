// Importing, replacing, exporting and clearing sprites.
import { DialogService, ProjectService, SpriteService, errorMessage } from "../api";
import { ask } from "../confirm.svelte";
import { prefs } from "../prefs.svelte";
import { app, run, toast } from "../state.svelte";
import { IMAGE_FILTERS, guard } from "./shared";

export interface SpriteExportOptions {
  sheets: boolean;
  format: string;
  skipEmpty: boolean;
  columns: number;
  rows: number;
  transparent: boolean;
}

export const spriteCommands = {
  async importSprites() {
    if (!guard()) return;
    const paths = await DialogService.OpenFiles("importSprites", "Import sprites", IMAGE_FILTERS, true);
    if (!paths?.length) return;
    const ids = await run("Importing", () => SpriteService.ImportImages(paths));
    if (ids?.length) {
      toast(`Added ${ids.length} sprite(s): #${ids[0]}${ids.length > 1 ? `-#${ids.at(-1)}` : ""}`, "success");
      app.selectedSprites = [ids[0]];
    }
  },

  async replaceSprite() {
    const id = app.selectedSprites[0];
    if (!guard() || !id) return;
    const paths = await DialogService.OpenFiles("replaceSprite", `Replace sprite #${id}`, IMAGE_FILTERS, false);
    if (!paths?.length) return;
    await run("Replacing", () => SpriteService.Replace(id, paths[0]));
  },

  /** Exports every sprite into dir. The export keeps running when its
   * window is closed, so the editor stays usable, and can be stopped. */
  async exportAllSpritesTo(dir: string, options: SpriteExportOptions): Promise<void> {
    app.busy = "Exporting sprites";
    app.stoppable = true;
    app.stopping = false;
    try {
      const summary = await (options.sheets
        ? SpriteService.ExportAllSheets(dir, options.format, options.columns, options.rows, options.transparent)
        : SpriteService.ExportAll(dir, options.format, options.skipEmpty));
      if (app.dialog === "exportSprites") app.dialog = null;
      if (options.sheets) toast(`Exported ${summary.files.toLocaleString()} sprite sheet(s).`, "success");
      else toast(`Exported ${summary.files.toLocaleString()} sprite(s)${summary.skipped ? `, skipped ${summary.skipped.toLocaleString()} empty` : ""}.`, "success");
    } catch (err) {
      if (app.stopping) {
        if (app.dialog === "exportSprites") app.dialog = null;
        toast("Export stopped. Files written so far stay in the folder.");
      } else toast(errorMessage(err), "error");
    } finally {
      app.busy = null;
      app.progress = null;
      app.stoppable = false;
      app.stopping = false;
    }
  },

  /** Stops the running export after asking. */
  async stopTask() {
    if (!app.stoppable || app.stopping) return;
    const ok = await ask({ title: "Export in progress", message: "Sprites are still being exported. Stop the export?", ok: "Stop" });
    // The export may have finished while the question was open.
    if (!ok || !app.stoppable) return;
    app.stopping = true;
    await ProjectService.Cancel();
  },

  /** Asks for single files or sprite sheets, then exports every sprite. */
  exportAllSprites() {
    if (guard()) app.dialog = "exportSprites";
  },

  /** Points every use of the selected sprites at another sprite. */
  replaceRefs() {
    if (!guard() || app.selectedSprites.length === 0) return;
    app.dialog = "replaceRefs";
  },

  async exportSprites() {
    if (!guard() || app.selectedSprites.length === 0) return;
    const dir = await DialogService.PickDirectory("exportSprites", "Export sprites to");
    if (!dir) return;
    const files = await run("Exporting", () => SpriteService.Export(app.selectedSprites, dir, prefs.settings.exportFormat));
    if (files) toast(`Exported ${files.length} sprite(s).`, "success");
  },

  async removeSprites() {
    if (!guard() || app.selectedSprites.length === 0) return;
    const count = app.selectedSprites.length;
    const ok = await run("Removing", async () => {
      await SpriteService.Remove(app.selectedSprites);
      return true;
    });
    if (!ok) return;
    app.selectedSprites = [];
    toast(`Cleared ${count} sprite(s). Ctrl+Z to undo.`);
  },
};
