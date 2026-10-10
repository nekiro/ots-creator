// User actions shared by the menu bar, toolbar, context menus and shortcuts.
import {
  CATEGORY_NAMES,
  Format,
  DialogService,
  ProjectService,
  SpriteService,
  ThingService,
  WindowService,
  encodeBytes,
  errorMessage,
  minId,
  type PropsPatch,
  type Recent,
  type Thing,
  OBJECT_EXT,
} from "./api";
import { clipboardThing, copyThing, pasteInto, type PasteMode } from "./clipboard";
import { ask } from "./confirm.svelte";
import { prefs } from "./prefs.svelte";
import { app, applyDraft, revertDraft, run, select, toast } from "./state.svelte";

const OBJECTS = [
  { name: "Object files (*.otobj, *.obd)", pattern: "*.otobj;*.obd" },
  { name: "OTS Creator objects (*.otobj)", pattern: "*.otobj" },
  { name: "Object Builder Data (*.obd)", pattern: "*.obd" },
];

const IMAGES = [{ name: "Images (*.png, *.bmp, *.gif, *.jpg)", pattern: "*.png;*.bmp;*.gif;*.jpg;*.jpeg" }];
const IMAGE_EXT = /\.(png|bmp|gif|jpe?g)$/i;
/** Id of the preview's file drop target: images dropped there are sheets. */
export const PREVIEW_DROP = "preview-drop";
/** Ids of object list cells as file drop targets: an image dropped on an
 * object is a sheet for that object. */
export const thingDropId = (id: number) => `thing-drop-${id}`;
const THING_DROP = /^thing-drop-(\d+)$/;
const CLIENT_EXT = /(\.(dat|spr|otfi)|catalog-content\.json)$/i;
const HAS_EXT = /\.[^\\/]+$/;

/** Save dialog filters with the preferred format first. */
function imageFilters(preferred = prefs.settings.exportFormat) {
  const all = [
    { value: "png", f: { name: "PNG image (*.png)", pattern: "*.png" } },
    { value: "bmp", f: { name: "Bitmap (*.bmp)", pattern: "*.bmp" } },
    { value: "jpg", f: { name: "JPEG image (*.jpg)", pattern: "*.jpg" } },
  ];
  const fmt = preferred;
  return [...all.filter((x) => x.value === fmt), ...all.filter((x) => x.value !== fmt)].map((x) => x.f);
}

const snapshot = <T>(v: T): T => JSON.parse(JSON.stringify(v));

function guard(): boolean {
  if (!app.open) {
    toast("Open or create a client first.");
    return false;
  }
  return true;
}

async function confirmDiscard(): Promise<boolean> {
  if (!app.dirty) return true;
  return ask({ title: "Unapplied changes", message: "The selected object has unapplied changes. Discard them?", ok: "Discard" });
}

/** Asks before the open client is replaced or closed with unsaved work. */
async function confirmLeave(title: string, ok: string): Promise<boolean> {
  if (!(await confirmDiscard())) return false;
  if (!app.project?.info.changed) return true;
  return ask({ title, message: "The client has uncompiled changes. Continue anyway?", ok });
}

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

/** Text put on the system clipboard by Copy object, so a later paste does
 * not pick up an image copied before it. */
const OBJECT_CLIP = "OTS Creator object";

/**
 * Paste (Ctrl+V outside text fields): an image on the clipboard (copied
 * image or PNG file) becomes a sprite sheet of the focused object, like a
 * drop on the preview; otherwise the copied object is pasted.
 */
export function handlePaste(e: ClipboardEvent, group: number): void {
  if (app.dialog || (e.target as HTMLElement).closest?.("input, textarea, select")) return;
  const data = e.clipboardData;
  const image = [...(data?.files ?? [])].find((f) => f.type.startsWith("image/"));
  e.preventDefault();
  if (image && data?.getData("text/plain") !== OBJECT_CLIP && app.open && app.focused !== null) {
    void image.arrayBuffer().then(
      (b) => commands.pasteSheet(new Uint8Array(b), group),
      (err) => toast(errorMessage(err), "error"),
    );
    return;
  }
  if (image && data?.getData("text/plain") !== OBJECT_CLIP && app.open) {
    toast("Select an object to paste the sprite sheet into.");
    return;
  }
  commands.paste("object").catch((err) => toast(errorMessage(err), "error"));
}

export const commands = {
  open: (path = "") => {
    app.openPath = path;
    app.dialog = "open";
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
    const ok = await run("Compiling", async () => {
      await ProjectService.Compile();
      return true;
    });
    if (ok) toast("Client compiled.", "success");
  },

  async close() {
    if (!(await confirmLeave("Close client", "Close"))) return;
    await ProjectService.Close();
  },

  /** Reopens a recent client with the version and features it was saved with. */
  async openRecent(r: Recent) {
    if (app.open && !(await confirmLeave("Open client", "Open"))) return;
    const st = await run("Loading client", () =>
      ProjectService.Open({
        format: r.format === Format.FormatAssets ? Format.FormatAssets : Format.FormatDat,
        datPath: r.datPath,
        sprPath: r.sprPath,
        version: r.version,
        features: r.features,
      }),
    );
    if (st?.open) toast(`Loaded ${r.version.name}.`, "success");
  },

  /** Window close with unsaved work or a running export (the backend held
   * the close). */
  async quit() {
    const lost = [
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
    const n = app.selection.length;
    const ok = await run("Removing", async () => {
      await ThingService.Remove(app.category, app.selection);
      return true;
    });
    if (ok) toast(`Removed ${n} ${CATEGORY_NAMES[app.category]}(s). Ctrl+Z to undo.`);
  },

  async importObd(replace = false) {
    if (!guard()) return;
    const paths = await DialogService.OpenFiles(replace ? "replaceObject" : "importObjects", replace ? "Replace with object" : "Import objects", OBJECTS, !replace);
    if (!paths?.length) return;
    const replaceId = replace ? (app.focused ?? 0) : 0;
    const results = await run("Importing", () => ThingService.ImportObjects(paths, replaceId));
    if (!results) return;
    const failed = results.filter((r) => r.error);
    const ok = results.length - failed.length;
    if (ok) toast(`Imported ${ok} object(s).`, "success");
    for (const f of failed) toast(`${f.path}: ${f.error}`, "error");
    const last = results.filter((r) => !r.error).at(-1);
    if (last && last.category === app.category) await select(last.id);
  },

  async exportObd() {
    if (!guard() || app.selection.length === 0) return;
    const dir = await DialogService.PickDirectory("exportObjects", "Export objects to");
    if (!dir) return;
    const files = await run("Exporting", () => ThingService.ExportObjects(app.category, app.selection, dir, prefs.settings.objectFormat));
    if (files) toast(`Exported ${files.length} file(s).`, "success");
  },

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
    const paths = await DialogService.OpenFiles("importSheet", "Import sprite sheet", IMAGES, false);
    if (paths?.length) await commands.loadSheet(paths[0], group);
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

  async importSprites() {
    if (!guard()) return;
    const paths = await DialogService.OpenFiles("importSprites", "Import sprites", IMAGES, true);
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
    const paths = await DialogService.OpenFiles("replaceSprite", `Replace sprite #${id}`, IMAGES, false);
    if (!paths?.length) return;
    await run("Replacing", () => SpriteService.Replace(id, paths[0]));
  },

  /** Exports every object with sprites, one folder per category. */
  async exportAllObjects() {
    if (!guard()) return;
    const dir = await DialogService.PickDirectory("exportAllObjects", "Export all objects to");
    if (!dir) return;
    const sum = await run("Exporting objects", () => ThingService.ExportAll(dir, true, prefs.settings.objectFormat));
    if (sum) toast(`Exported ${sum.files.toLocaleString()} object(s), skipped ${sum.skipped.toLocaleString()} empty.`, "success");
  },

  /** Exports every sprite into dir. The export keeps running when its
   * window is closed, so the editor stays usable, and can be stopped. */
  async exportAllSpritesTo(
    dir: string,
    o: { sheets: boolean; format: string; skipEmpty: boolean; columns: number; rows: number; transparent: boolean },
  ): Promise<void> {
    app.busy = "Exporting sprites";
    app.stoppable = true;
    app.stopping = false;
    try {
      const sum = await (o.sheets
        ? SpriteService.ExportAllSheets(dir, o.format, o.columns, o.rows, o.transparent)
        : SpriteService.ExportAll(dir, o.format, o.skipEmpty));
      if (app.dialog === "exportSprites") app.dialog = null;
      if (o.sheets) toast(`Exported ${sum.files.toLocaleString()} sprite sheet(s).`, "success");
      else toast(`Exported ${sum.files.toLocaleString()} sprite(s)${sum.skipped ? `, skipped ${sum.skipped.toLocaleString()} empty` : ""}.`, "success");
    } catch (e) {
      if (app.stopping) {
        if (app.dialog === "exportSprites") app.dialog = null;
        toast("Export stopped. Files written so far stay in the folder.");
      } else toast(errorMessage(e), "error");
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
    const n = app.selectedSprites.length;
    const ok = await run("Removing", async () => {
      await SpriteService.Remove(app.selectedSprites);
      return true;
    });
    if (!ok) return;
    app.selectedSprites = [];
    toast(`Cleared ${n} sprite(s). Ctrl+Z to undo.`);
  },

  copyObject() {
    if (!app.draft) return;
    copyThing(snapshot(app.draft));
    // Replaces an image on the system clipboard (see handlePaste).
    navigator.clipboard?.writeText(OBJECT_CLIP).catch(() => {});
    toast(`Copied ${CATEGORY_NAMES[app.draft.category]} #${app.draft.id}.`);
  },

  /**
   * Pastes the copied object (or its properties) onto the selection. One
   * object is pasted into the editor (Apply saves it); several are saved
   * directly as one undo step.
   */
  async paste(mode: PasteMode) {
    const src = clipboardThing();
    if (!guard() || app.selection.length === 0) return;
    if (!src) {
      toast("Copy an object first (Ctrl+C).");
      return;
    }
    try {
      if (app.selection.length === 1 && app.draft) {
        app.draft = pasteInto(snapshot(app.draft), src, mode);
        return;
      }
      const ids = [...app.selection];
      if (mode === "properties") {
        const n = await run("Pasting", () => ThingService.Patch(app.category, ids, src.props as unknown as PropsPatch));
        if (n !== undefined) toast(`Pasted properties onto ${n} object(s).`, "success");
        return;
      }
      const things: Thing[] = ids.map((id) => pasteInto({ ...src, id, category: app.category }, src, "object"));
      const ok = await run("Pasting", async () => {
        await ThingService.UpdateMany(things);
        return true;
      });
      if (ok) toast(`Pasted onto ${ids.length} object(s).`, "success");
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  },

  async convertFrameGroups(toGroups: boolean) {
    if (!guard() || !(await confirmDiscard())) return;
    const message = toGroups
      ? "Split every outfit into an idle group (frame 1) and a walking group (the other frames)? Use this when moving to a client with frame groups."
      : "Merge the idle and walking groups of every outfit into one group (idle frame first)? Use this when moving to a client without frame groups.";
    if (!(await ask({ title: "Convert frame groups", message, ok: "Convert" }))) return;
    const res = await run("Converting", () => ThingService.ConvertFrameGroups(toGroups));
    if (!res) return;
    const skipped = res.skipped ? ` ${res.skipped} skipped: idle and walking layouts differ.` : "";
    toast(`Converted ${res.converted} outfit(s).${skipped}`, res.converted ? "success" : "info");
  },

  /** Handles files dropped onto the window. */
  /**
   * Files dropped on the window. target is the id of the drop target
   * element: one image dropped on the preview or on an object of the list
   * is a sprite sheet for that object (group: the preview's frame group).
   */
  async drop(paths: string[], target = "", group = 0) {
    if (app.dialog || !paths.length) return;
    const sheet = app.open && paths.length === 1 && IMAGE_EXT.test(paths[0]) ? paths[0] : null;
    if (sheet && target === PREVIEW_DROP && app.focused !== null) {
      await commands.loadSheet(sheet, group);
      return;
    }
    const onThing = THING_DROP.exec(target);
    if (sheet && onThing) {
      // Keep unapplied edits of the current object, then switch to the target.
      if (app.dirty) {
        await applyDraft();
        if (app.dirty) return;
      }
      const id = Number(onThing[1]);
      await select(id);
      if (app.focused === id) await commands.loadSheet(sheet, 0);
      return;
    }
    const obd = paths.filter((p) => OBJECT_EXT.test(p));
    const images = paths.filter((p) => IMAGE_EXT.test(p));
    const client = paths.find((p) => CLIENT_EXT.test(p) || !HAS_EXT.test(p));
    if ((obd.length || images.length) && !app.open) {
      commands.viewObd(obd[0] ?? images[0]);
    } else if (obd.length) {
      const results = await run("Importing", () => ThingService.ImportObjects(obd, 0));
      if (!results) return;
      const ok = results.filter((r) => !r.error).length;
      if (ok) toast(`Imported ${ok} object(s).`, "success");
      for (const f of results.filter((r) => r.error)) toast(`${f.path}: ${f.error}`, "error");
      const last = results.filter((r) => !r.error).at(-1);
      if (last && last.category === app.category) await select(last.id);
    } else if (images.length && app.open) {
      const ids = await run("Importing", () => SpriteService.ImportImages(images));
      if (ids?.length) {
        toast(`Added ${ids.length} sprite(s).`, "success");
        app.selectedSprites = [ids[0]];
      }
    } else if (client) {
      if (app.open && !(await confirmLeave("Open client", "Open"))) return;
      commands.open(client);
    } else {
      toast("Drop a client folder, a .dat, an assets folder, .otobj, .obd or image files.");
    }
  },

  /** Opens the market browser. */
  market() {
    app.dialog = "market";
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
    app.shareTarget = { kind: "sprites", ids: [...app.selectedSprites].sort((a, b) => a - b) };
    app.dialog = "share";
  },

  /** Opens the share window with a picker, starting from the selection. */
  sharePick() {
    app.shareTarget = !app.open
      ? { kind: "file", path: "", pick: true }
      : app.focused === null && app.selectedSprites.length
        ? { kind: "sprites", ids: [...app.selectedSprites].sort((a, b) => a - b), pick: true }
        : { kind: "object", category: app.category, id: app.focused ?? minId(app.category), pick: true };
    app.dialog = "share";
  },

  /** Opens the object viewer, optionally with an object or image file. */
  viewObd(path = "") {
    app.obdPath = path;
    app.dialog = "obd";
  },
};

/** Global keyboard shortcuts. Inputs keep their own editing keys. */
export function handleShortcut(e: KeyboardEvent): void {
  const target = e.target as HTMLElement;
  const typing = target.closest("input, textarea, select") !== null;
  const mod = e.ctrlKey || e.metaKey;
  const key = e.key.toLowerCase();
  const run = (fn: () => unknown) => {
    e.preventDefault();
    Promise.resolve(fn()).catch((err) => toast(errorMessage(err), "error"));
  };
  if (app.dialog) return;
  if (mod && key === "o") return run(() => commands.open());
  if (mod && key === "n") return run(commands.create);
  if (mod && e.shiftKey && key === "s") return run(commands.compileAs);
  if (mod && key === "s") return run(commands.compile);
  if (mod && key === "enter") return run(commands.apply);
  if (typing) return;
  if (mod && key === "z" && !e.shiftKey) return run(commands.undo);
  if (mod && (key === "y" || (key === "z" && e.shiftKey))) return run(commands.redo);
  if (mod && key === "d") return run(commands.duplicate);
  if (mod && key === "i") return run(() => commands.importObd());
  if (mod && key === "e") return run(() => commands.exportObd());
  if (mod && key === "c") return run(commands.copyObject);
  if (mod && e.shiftKey && key === "v") return run(() => commands.paste("properties"));
  // Plain Ctrl+V arrives as a paste event (handlePaste) with the clipboard.
}
