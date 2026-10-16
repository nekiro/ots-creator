// Opening, compiling and closing clients, undo and the app's windows.
import { CompareService, DialogService, Format, ProjectService, WindowService, type Recent } from "../api";
import { ask } from "../confirm.svelte";
import { map } from "../map/state.svelte";
import { app, applyDraft, revertDraft, run, toast } from "../state.svelte";
import { offerOptimize, openedNote, otbNote, sheetsNote } from "./compile";
import { confirmLeave, guard } from "./shared";

export const projectCommands = {
  open: (path = "") => {
    app.openPath = path;
    app.dialog = "open";
  },
  /**
   * Asks for the client folder first. Clients with nothing to choose (assets,
   * or a known version with an .otfi) open right away; others show the open
   * form filled with the folder. other loads client B of the compare window.
   */
  async browseOpen(other = false) {
    const dir = await DialogService.PickDirectory(other ? "compareClient" : "openClient", "Open client folder");
    if (!dir) return;
    let files;
    try {
      files = await ProjectService.Inspect(dir);
    } catch {
      files = null;
    }
    if (!files?.detected || !(files.format === Format.FormatAssets || files.hasOtfi)) {
      app.openOther = other;
      projectCommands.open(dir);
      return;
    }
    const request = { format: files.format, datPath: files.datPath, sprPath: files.sprPath, version: null, features: files.features, otbPath: null };
    const state = await run("Loading client", () => (other ? CompareService.Open(request) : ProjectService.Open(request)));
    if (!state?.open) return;
    if (other) app.dialog = "compare";
    openedNote(state);
    toast(`Loaded ${files.detected.name}: ${state.info.counts.items - 99} items, ${state.info.counts.sprites} sprites.`, "success");
  },
  create: () => (app.dialog = "new"),
  /** Opens the compare window; without a second client it asks for one first. */
  compare: () => {
    if (!guard()) return;
    app.openOther = !app.other?.open;
    app.dialog = app.openOther ? "open" : "compare";
  },
  compileAs: () => guard() && (app.dialog = "compile"),

  async compile() {
    if (!guard()) return;
    if (app.dirty) await applyDraft();
    if (!app.project?.info.datPath) {
      app.dialog = "compile";
      return;
    }
    if (!(await offerOptimize())) return;
    const stats = await run("Compiling", () => ProjectService.Compile());
    if (stats) toast(`Client compiled.${sheetsNote(stats)}${otbNote(stats)}`, "success");
  },

  async close() {
    if (!(await confirmLeave("Close client", "Close"))) return;
    await ProjectService.Close();
  },

  /** Reopens a recent client with the version and features it was saved with. */
  async openRecent(recent: Recent) {
    if (app.open && !(await confirmLeave("Open client", "Open"))) return;
    const state = await run("Loading client", () =>
      ProjectService.Open({
        format: recent.format === Format.FormatAssets ? Format.FormatAssets : Format.FormatDat,
        datPath: recent.datPath,
        sprPath: recent.sprPath,
        version: recent.version,
        features: recent.features,
        otbPath: null,
      }),
    );
    openedNote(state);
    if (state?.open) toast(`Loaded ${recent.version.name}.`, "success");
  },

  /** Window close with unsaved work or a running export (the backend held
   * the close). */
  async quit() {
    const maps = map.maps.filter((open) => open.changed).map((open) => open.name);
    const lost = [
      maps.length && `unsaved maps (${maps.join(", ")})`,
      app.stoppable && "an export in progress",
      app.dirty && "unapplied object changes",
      app.project?.info.changed && "uncompiled client changes",
      app.other?.info.changed && "uncompiled changes in the compared client",
    ].filter(Boolean);
    const message = `You have ${lost.join(" and ") || "unsaved changes"}. Quit anyway?`;
    if (await ask({ title: "Quit OTS Creator", message, ok: "Quit" })) await WindowService.Quit();
  },

  async undo() {
    if (!guard()) return;
    const label = await run("Undo", () => ProjectService.Undo());
    if (label) toast(`Undo: ${label}`);
  },

  async redo() {
    if (!guard()) return;
    const label = await run("Redo", () => ProjectService.Redo());
    if (label) toast(`Redo: ${label}`);
  },

  apply: () => applyDraft(),
  revert: () => revertDraft(),

  /** Opens the market browser. */
  market() {
    app.dialog = "market";
  },

  /** Opens the object viewer, optionally with an object or image file. */
  viewObd(path = "") {
    app.obdPath = path;
    app.dialog = "obd";
  },
};
