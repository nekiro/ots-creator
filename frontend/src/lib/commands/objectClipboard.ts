// Copying and pasting objects, and Ctrl+V of images as sprite sheets.
import { CATEGORY_NAMES, ThingService, errorMessage, type PropsPatch, type Thing } from "../api";
import { clipboardThing, clipboardThings, copyThings, pasteInto, type PasteMode } from "../clipboard";
import { app, run, select, toast } from "../state/app.svelte";
import { sheetCommands } from "./sheets";
import { OBJECT_CLIP, confirmDiscard, guard, snapshot } from "./shared";

export const clipboardCommands = {
  /** Copies the selection; the focused object is copied with unapplied edits. */
  async copyObject() {
    const draft = app.draft;
    if (!draft) return;
    const ids = app.selection.length > 1 ? [...app.selection].sort((left, right) => left - right) : [draft.id];
    const category = app.category;
    const things = await run("Copying", () => Promise.all(ids.map(async (id) => (id === draft.id ? snapshot(draft) : ((await ThingService.Get(category, id)) as Thing)))));
    if (!things) return;
    copyThings(things);
    // Replaces an image on the system clipboard (see handlePaste).
    navigator.clipboard?.writeText(OBJECT_CLIP).catch(() => {});
    toast(things.length === 1 ? `Copied ${CATEGORY_NAMES[draft.category]} #${draft.id}.` : `Copied ${things.length} ${CATEGORY_NAMES[draft.category]}(s).`);
  },

  /**
   * Pastes the copied object (or its properties) onto the selection. One
   * object is pasted into the editor (Apply saves it); several are saved
   * directly as one undo step. Several copied objects go to the ids from the
   * focused one on (see pasteCopies).
   */
  async paste(mode: PasteMode) {
    if (!guard() || app.selection.length === 0) return;
    const copied = clipboardThings();
    if (copied.length > 1) {
      if (mode === "properties") toast("Copy a single object to paste its properties.");
      else await pasteCopies(copied);
      return;
    }
    const source = clipboardThing();
    if (!source) {
      toast("Copy an object first (Ctrl+C).");
      return;
    }
    try {
      if (app.selection.length === 1 && app.draft) {
        app.draft = pasteInto(snapshot(app.draft), source, mode);
        return;
      }
      const ids = [...app.selection];
      if (mode === "properties") {
        const patched = await run("Pasting", () => ThingService.Patch(app.category, ids, source.props as unknown as PropsPatch));
        if (patched !== undefined) toast(`Pasted properties onto ${patched} object(s).`, "success");
        return;
      }
      const things: Thing[] = ids.map((id) => pasteInto({ ...source, id, category: app.category }, source, "object"));
      const ok = await run("Pasting", async () => {
        await ThingService.UpdateMany(things);
        return true;
      });
      if (ok) toast(`Pasted onto ${ids.length} object(s).`, "success");
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  },
};

/**
 * Puts the copied objects at the ids from the focused one on, replacing the
 * objects there (past the end the category grows), and selects them.
 */
async function pasteCopies(copied: Thing[]): Promise<void> {
  const at = app.focused;
  if (at === null || !(await confirmDiscard())) return;
  const category = app.category;
  const ids = await run("Pasting", () => ThingService.Paste(category, at, copied));
  if (!ids?.length || app.category !== category) return;
  await select(ids[0]);
  app.selection = ids;
  toast(`Pasted ${ids.length} ${CATEGORY_NAMES[category]}(s) onto #${ids[0]}-#${ids[ids.length - 1]}. Ctrl+Z to undo.`, "success");
}

/**
 * Paste (Ctrl+V outside text fields): an image on the clipboard (copied
 * image or PNG file) becomes a sprite sheet of the focused object, like a
 * drop on the preview; otherwise the copied object is pasted.
 */
export function handlePaste(event: ClipboardEvent, group: number): void {
  if (app.dialog || (event.target as HTMLElement).closest?.("input, textarea, select")) return;
  const data = event.clipboardData;
  const image = [...(data?.files ?? [])].find((file) => file.type.startsWith("image/"));
  event.preventDefault();
  if (image && data?.getData("text/plain") !== OBJECT_CLIP && app.open && app.focused !== null) {
    void image.arrayBuffer().then(
      (buffer) => sheetCommands.pasteSheet(new Uint8Array(buffer), group),
      (err) => toast(errorMessage(err), "error"),
    );
    return;
  }
  if (image && data?.getData("text/plain") !== OBJECT_CLIP && app.open) {
    toast("Select an object to paste the sprite sheet into.");
    return;
  }
  clipboardCommands.paste("object").catch((err) => toast(errorMessage(err), "error"));
}
