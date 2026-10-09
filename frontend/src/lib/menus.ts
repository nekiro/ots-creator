// Menu entries shared by the menu bar and the list context menus.
import { clipboardThing } from "./clipboard";
import { commands } from "./commands";
import type { MenuEntry } from "./menu.svelte";
import { app, toast } from "./state.svelte";

const noProject = () => !app.open;
const noThing = () => !app.open || app.selection.length === 0;
const notSingle = () => !app.open || app.selection.length !== 1;
const noSprite = () => !app.open || app.selectedSprites.length === 0;
const noClip = () => noThing() || !clipboardThing();

function copy(text: string) {
  navigator.clipboard.writeText(text).then(
    () => toast(`Copied ${text}`),
    () => toast("Clipboard is not available.", "error"),
  );
}

export function editMenu(): MenuEntry[] {
  return [
    { label: "Undo", keys: "Ctrl+Z", action: commands.undo, disabled: () => !app.project?.info.canUndo },
    { label: "Redo", keys: "Ctrl+Y", action: commands.redo, disabled: () => !app.project?.info.canRedo },
    "-",
    ...clipboardEntries(),
    "-",
    { label: "Apply changes", keys: "Ctrl+Enter", action: commands.apply, disabled: () => !app.dirty },
    { label: "Revert changes", action: commands.revert, disabled: () => !app.dirty },
  ];
}

function clipboardEntries(): MenuEntry[] {
  return [
    { label: "Copy object", keys: "Ctrl+C", action: commands.copyObject, disabled: () => !app.draft },
    { label: "Paste object", keys: "Ctrl+V", action: () => commands.paste("object"), disabled: noClip },
    { label: "Paste properties", keys: "Ctrl+Shift+V", action: () => commands.paste("properties"), disabled: noClip },
  ];
}

export function toolsMenu(): MenuEntry[] {
  return [
    { label: "Slicer…", action: () => (app.dialog = "slicer"), disabled: noProject },
    { label: "Optimize sprites…", action: () => (app.dialog = "optimize"), disabled: noProject },
    { label: "Frame durations…", action: () => (app.dialog = "durations"), disabled: noProject },
    "-",
    { label: "Split outfits into frame groups…", action: () => commands.convertFrameGroups(true), disabled: noProject },
    { label: "Merge outfit frame groups…", action: () => commands.convertFrameGroups(false), disabled: noProject },
    "-",
    { label: "Compare & merge clients…", action: commands.compare, disabled: noProject },
    { label: "Object viewer…", action: () => commands.viewObd() },
    { label: "Market…", action: commands.market },
  ];
}

export function objectMenu(context = false): MenuEntry[] {
  return [
    { label: "New object", action: commands.newThing, disabled: noProject },
    { label: "Duplicate", keys: "Ctrl+D", action: commands.duplicate, disabled: noThing },
    { label: "Replace with object (OBD)…", action: () => commands.importObd(true), disabled: notSingle },
    { label: "Remove", keys: "Del", action: commands.remove, disabled: noThing },
    "-",
    ...(context ? [] : [{ label: "Import objects (OBD)…", keys: "Ctrl+I", action: () => commands.importObd(), disabled: noProject }]),
    { label: "Export selected (OBD)…", keys: "Ctrl+E", action: commands.exportObd, disabled: noThing },
    "-",
    { label: "Import sprite sheet…", action: () => commands.importSheet(0), disabled: notSingle },
    { label: "Export sprite sheet…", action: () => commands.exportSheet(0), disabled: notSingle },
    { label: "Share to market…", action: commands.shareObject, disabled: notSingle },
    ...(context
      ? (["-", ...clipboardEntries(), "-", { label: "Copy id", action: () => copy(app.selection.join(", ")), disabled: noThing }] as MenuEntry[])
      : []),
  ];
}

/** Context menu of the empty area of the object list. */
export function objectListMenu(): MenuEntry[] {
  return [
    { label: "New object", action: commands.newThing, disabled: noProject },
    { label: "Import objects (OBD)…", keys: "Ctrl+I", action: () => commands.importObd(), disabled: noProject },
    { label: "Browse market…", action: commands.market },
  ];
}

/** Context menu of the empty area of the sprite list. */
export function spriteListMenu(): MenuEntry[] {
  return [
    { label: "Import images…", action: commands.importSprites, disabled: noProject },
    { label: "Browse market…", action: commands.market },
  ];
}

export function spriteMenu(context = false): MenuEntry[] {
  return [
    ...(context ? [] : [{ label: "Import images…", action: commands.importSprites, disabled: noProject }]),
    { label: "Replace selected…", action: commands.replaceSprite, disabled: noSprite },
    { label: "Export selected…", action: commands.exportSprites, disabled: noSprite },
    { label: "Share to market…", action: commands.shareSprites, disabled: noSprite },
    "-",
    { label: "Clear selected", keys: "Del", action: commands.removeSprites, disabled: noSprite },
    ...(context
      ? (["-", { label: "Copy id", action: () => copy(app.selectedSprites.join(", ")), disabled: noSprite }] as MenuEntry[])
      : []),
  ];
}

