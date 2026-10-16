// Files dropped onto the window.
import { SpriteService, ThingService } from "../api";
import { app, applyDraft, run, select, toast } from "../state.svelte";
import { classifyDrop } from "./classifyDrop";
import { reportImported } from "./objects";
import { projectCommands } from "./project";
import { sheetCommands } from "./sheets";
import { confirmLeave } from "./shared";

export const dropCommands = {
  /**
   * Files dropped on the window. target is the id of the drop target
   * element: one image dropped on the preview or on an object of the list
   * is a sprite sheet for that object (group: the preview's frame group).
   */
  async drop(paths: string[], target = "", group = 0) {
    if (app.dialog || !paths.length) return;
    const action = classifyDrop(paths, target, { open: app.open, focused: app.focused !== null });
    switch (action.kind) {
      case "previewSheet":
        await sheetCommands.loadSheet(action.path, group);
        return;
      case "thingSheet":
        // Keep unapplied edits of the current object, then switch to the target.
        if (app.dirty) {
          await applyDraft();
          if (app.dirty) return;
        }
        await select(action.id);
        if (app.focused === action.id) await sheetCommands.loadSheet(action.path, 0);
        return;
      case "view":
        projectCommands.viewObd(action.path);
        return;
      case "importObjects": {
        const results = await run("Importing", () => ThingService.ImportObjects(action.paths, 0));
        if (results) await reportImported(results);
        return;
      }
      case "importImages": {
        const ids = await run("Importing", () => SpriteService.ImportImages(action.paths));
        if (ids?.length) {
          toast(`Added ${ids.length} sprite(s).`, "success");
          app.selectedSprites = [ids[0]];
        }
        return;
      }
      case "openClient":
        if (app.open && !(await confirmLeave("Open client", "Open"))) return;
        projectCommands.open(action.path);
        return;
      case "unknown":
        toast("Drop a client folder, a .dat, an assets folder, .otobj, .obd or image files.");
    }
  },
};
