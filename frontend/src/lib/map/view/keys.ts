// Keyboard shortcuts of the map editor. Every action has default keys; the
// user can change them in the settings (map.keys holds only the changed
// actions). Mouse gestures, Esc, the arrows and 1-9 are fixed.
import { errorMessage } from "../../api";
import { toast } from "../../state/app.svelte";
import { mapCommands, selectionScope } from "./commands";
import { comboOf } from "./keycombo";
import { map, selectBrush } from "../state.svelte";
import { MAX_BRUSH_SIZE } from "../constants";

export { comboOf };

export type KeyGroup = "Drawing" | "Selection" | "View" | "Map";
export const KEY_GROUPS: KeyGroup[] = ["Drawing", "Selection", "View", "Map"];

export interface KeyAction {
  id: string;
  group: KeyGroup;
  label: string;
  keys: string[];
  run: () => unknown;
  /** Works without an open map and while typing in a field. */
  always?: boolean;
  repeat?: boolean;
}

export const KEY_ACTIONS: KeyAction[] = [
  { id: "brush", group: "Drawing", label: "Brush tool", keys: ["B"], run: () => (map.tool = "brush") },
  { id: "eraser", group: "Drawing", label: "Eraser", keys: ["E"], run: mapCommands.eraser },
  { id: "fill", group: "Drawing", label: "Fill with the ground brush", keys: ["F"], run: () => (map.fill = !map.fill) },
  { id: "sizeDown", group: "Drawing", label: "Smaller brush", keys: ["["], run: () => (map.size = Math.max(0, map.size - 1)), repeat: true },
  { id: "sizeUp", group: "Drawing", label: "Bigger brush", keys: ["]"], run: () => (map.size = Math.min(MAX_BRUSH_SIZE, map.size + 1)), repeat: true },
  { id: "shape", group: "Drawing", label: "Square or round brush", keys: ["Q"], run: () => (map.shape = map.shape === "square" ? "circle" : "square") },
  { id: "suggest", group: "Drawing", label: "Suggest items for the tile under the cursor", keys: ["W"], run: () => (map.suggest = map.hover) },
  { id: "automagic", group: "Drawing", label: "Automatic borders and walls", keys: ["A"], run: mapCommands.toggleAutomagic },

  { id: "select", group: "Selection", label: "Selection tool", keys: ["S"], run: () => ((map.tool = "select"), (map.pasting = false)) },
  { id: "copy", group: "Selection", label: "Copy", keys: ["Ctrl+C"], run: mapCommands.copy },
  { id: "cut", group: "Selection", label: "Cut", keys: ["Ctrl+X"], run: mapCommands.cut },
  { id: "paste", group: "Selection", label: "Paste", keys: ["Ctrl+V"], run: mapCommands.startPaste },
  { id: "delete", group: "Selection", label: "Delete the selection", keys: ["Delete"], run: mapCommands.remove },
  { id: "selectAll", group: "Selection", label: "Select the floor", keys: ["Ctrl+A"], run: mapCommands.selectAll },
  { id: "borderize", group: "Selection", label: "Borderize the selection", keys: ["Ctrl+B"], run: () => mapCommands.tool("borderizeSelection") },

  { id: "zoomIn", group: "View", label: "Zoom in", keys: ["+", "="], run: () => mapCommands.zoom(1), repeat: true },
  { id: "zoomOut", group: "View", label: "Zoom out", keys: ["-"], run: () => mapCommands.zoom(-1), repeat: true },
  { id: "zoomReset", group: "View", label: "Normal zoom", keys: ["Ctrl+0"], run: mapCommands.zoomReset },
  { id: "floorUp", group: "View", label: "Floor up", keys: ["PageUp"], run: mapCommands.floorUp, repeat: true },
  { id: "floorDown", group: "View", label: "Floor down", keys: ["PageDown"], run: mapCommands.floorDown, repeat: true },
  { id: "grid", group: "View", label: "Grid", keys: ["G"], run: () => (map.view.grid = !map.view.grid) },
  { id: "lowerFloors", group: "View", label: "Lower floors", keys: ["L"], run: () => (map.view.lowerFloors = !map.view.lowerFloors) },
  { id: "ghostHigher", group: "View", label: "Ghost floor above", keys: ["H"], run: () => (map.view.ghostHigher = !map.view.ghostHigher) },
  { id: "play", group: "View", label: "Play from here", keys: ["P"], run: () => mapCommands.play() },
  { id: "lights", group: "View", label: "Lights", keys: ["I"], run: () => (map.view.lights = !map.view.lights) },

  { id: "new", group: "Map", label: "New map", keys: ["Ctrl+N"], run: mapCommands.newMap, always: true },
  { id: "open", group: "Map", label: "Open map", keys: ["Ctrl+O"], run: () => mapCommands.open(), always: true },
  { id: "save", group: "Map", label: "Save", keys: ["Ctrl+S"], run: () => mapCommands.save(), always: true },
  { id: "saveAs", group: "Map", label: "Save as", keys: ["Ctrl+Shift+S"], run: () => mapCommands.save(true), always: true },
  { id: "close", group: "Map", label: "Close the map", keys: ["Ctrl+W"], run: () => mapCommands.close() },
  { id: "undo", group: "Map", label: "Undo", keys: ["Ctrl+Z"], run: mapCommands.undo, repeat: true },
  { id: "redo", group: "Map", label: "Redo", keys: ["Ctrl+Y", "Ctrl+Shift+Z"], run: mapCommands.redo, repeat: true },
  { id: "palette", group: "Map", label: "Go to anything", keys: ["Ctrl+K", "Ctrl+P", "Ctrl+G"], run: () => mapCommands.commandPalette(), always: true },
  { id: "find", group: "Map", label: "Find", keys: ["Ctrl+F"], run: () => mapCommands.find(selectionScope()) },
  { id: "replace", group: "Map", label: "Replace", keys: ["Ctrl+H"], run: () => mapCommands.find({ ...selectionScope(), replace: true }) },
  { id: "shortcuts", group: "Map", label: "Keyboard shortcuts", keys: ["F1"], run: () => (map.dialog = "shortcuts"), always: true },
];

export const FIXED_KEYS: { group: KeyGroup; keys: string; label: string }[] = [
  { group: "Drawing", keys: "Click / drag", label: "Draw with the brush" },
  { group: "Drawing", keys: "Ctrl + click", label: "Erase what the brush places" },
  { group: "Drawing", keys: "Shift + drag", label: "Draw a rectangle" },
  { group: "Drawing", keys: "Alt + ground brush", label: "Only replace the ground you started on" },
  { group: "Drawing", keys: "Alt + wall click", label: "Change the wall shape" },
  { group: "Drawing", keys: "Alt + door", label: "Place the door open" },
  { group: "Drawing", keys: "Ctrl + Alt + click", label: "Use the item under the cursor as brush" },
  { group: "Drawing", keys: "Alt + wheel", label: "Brush size" },
  { group: "Drawing", keys: "Right click", label: "Drop the brush, back to selection (with selection: the tile menu)" },
  { group: "Drawing", keys: "1 – 9", label: "Recent brushes" },
  { group: "Drawing", keys: "Tab / Shift + Tab", label: "Swap the item under the cursor for its next / previous variant" },
  { group: "Drawing", keys: "Ctrl + click (suggestions)", label: "Pick several suggestions, Enter paints a mix of them" },
  { group: "Drawing", keys: "Enter (after a part)", label: "Add the rest of an object made of several items" },
  { group: "Selection", keys: "Drag", label: "Select a box (drag the selection to move it)" },
  { group: "Selection", keys: "Ctrl + click", label: "Add or remove the top thing" },
  { group: "Selection", keys: "Shift + drag", label: "Box selection; Ctrl adds, Alt removes" },
  { group: "Selection", keys: "Click", label: "Select a tile and show it in the inspector" },
  { group: "Selection", keys: "Esc", label: "Cancel paste, leave the brush, deselect, back to the map" },
  { group: "View", keys: "Wheel", label: "Zoom at the cursor" },
  { group: "View", keys: "Ctrl + wheel", label: "Floor up / down" },
  { group: "View", keys: "Middle or right drag, Space + drag", label: "Move the view" },
  { group: "View", keys: "Arrows (Shift: faster)", label: "Move the view" },
];

export function keysOf(action: KeyAction, overrides: Record<string, string[]> = map.keys): string[] {
  return overrides[action.id] ?? action.keys;
}

export function keyLabel(id: string): string | undefined {
  const action = KEY_ACTIONS.find((candidate) => candidate.id === id);
  return action ? keysOf(action)[0] : undefined;
}

export function mapShortcut(event: KeyboardEvent): boolean {
  const combo = comboOf(event);
  if (!combo) return false;
  const typing = (event.target as HTMLElement).closest("input, textarea, select, [contenteditable]") !== null;
  const ready = !typing && map.activeId !== null;
  const run = (action: () => unknown) => {
    event.preventDefault();
    Promise.resolve(action()).catch((err) => toast(errorMessage(err), "error"));
    return true;
  };
  for (const action of KEY_ACTIONS) {
    if ((action.always || ready) && keysOf(action).includes(combo)) {
      if (event.repeat && !action.repeat) {
        event.preventDefault();
        return true;
      }
      return run(action.run);
    }
  }
  if (!ready || event.ctrlKey || event.metaKey) return false;
  switch (event.key) {
    case "Escape":
      if (map.pick) return run(() => (map.pick = null));
      if (map.pasting) return run(() => (map.pasting = false));
      if (map.tool === "brush") return run(() => (map.tool = "select"));
      if (map.active?.selection) return run(mapCommands.clearSelection);
      if (map.findOpen) return run(() => ((map.findOpen = false), (map.results = null)));
      if (map.focus) return run(() => mapCommands.inspect(null));
      return false;
    case "ArrowLeft":
    case "ArrowRight":
    case "ArrowUp":
    case "ArrowDown": {
      const step = event.shiftKey ? 10 : 2;
      const move = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] }[event.key]!;
      return run(() => mapCommands.pan(move[0], move[1]));
    }
  }
  if (/^[1-9]$/.test(event.key)) {
    const brush = map.recentBrushes[Number(event.key) - 1];
    if (brush) return run(() => selectBrush(brush));
  }
  return false;
}
