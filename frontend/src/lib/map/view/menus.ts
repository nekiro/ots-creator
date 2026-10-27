import { app, toast } from "../../state/app.svelte";
import type { MenuEntry } from "../../menu.svelte";
import { MapService, fmtPos, type Position } from "../data/api";
import { ItemFlags, TileFlags, type TileData } from "../data/chunks";
import { goTo, mapCommands, selectionScope } from "./commands";
import { edit, map, selectBrush, useCreature, warningCount } from "../state.svelte";
import { addHouse, addTown, addWaypoint } from "../editor/world";
import { keyLabel } from "./keys";
import type { ViewOptions } from "../render/scene";

const noMap = () => map.activeId === null;
const noSelection = () => !map.active?.selection;
const noClient = () => !app.open;

function copy(text: string) {
  navigator.clipboard.writeText(text).then(
    () => toast(`Copied ${text}`),
    () => toast("Clipboard is not available.", "error"),
  );
}

type ViewToggle = { [Key in keyof ViewOptions]: ViewOptions[Key] extends boolean ? Key : never }[keyof ViewOptions];
const toggle = (key: ViewToggle, label: string, keys?: string): MenuEntry => ({
  label,
  keys,
  checked: () => !!map.view[key],
  action: () => (map.view[key] = !map.view[key]),
});

export function mapMenus(): { label: string; items: MenuEntry[] }[] {
  const recent: MenuEntry[] = (map.recent ?? []).slice(0, 6).map((recentMap) => ({
    label: recentMap.path.length > 60 ? "…" + recentMap.path.slice(-58) : recentMap.path,
    action: () => mapCommands.open(recentMap.path),
  }));
  const undo = map.active?.undoLabel;
  const redo = map.active?.redoLabel;
  return [
    {
      label: "File",
      items: [
        { label: "New map", keys: keyLabel("new"), action: mapCommands.newMap, disabled: noClient },
        { label: "Open map…", keys: keyLabel("open"), action: () => mapCommands.open(), disabled: noClient },
        ...(recent.length ? (["-", ...recent] as MenuEntry[]) : []),
        "-",
        { label: "Save", keys: keyLabel("save"), action: () => mapCommands.save(), disabled: noMap },
        { label: "Save as…", keys: keyLabel("saveAs"), action: () => mapCommands.save(true), disabled: noMap },
        "-",
        { label: "Import a map into this one…", action: () => (map.dialog = "import"), disabled: noMap },
        { label: "Export minimap images…", action: () => (map.dialog = "minimap"), disabled: noMap },
        { label: "Save screenshot of the view…", action: mapCommands.exportImage, disabled: noMap },
        "-",
        { label: "Close map", keys: keyLabel("close"), action: () => mapCommands.close(), disabled: noMap },
      ],
    },
    {
      label: "Edit",
      items: [
        { label: undo ? `Undo ${undo.toLowerCase()}` : "Undo", keys: keyLabel("undo"), action: mapCommands.undo, disabled: () => !map.active?.canUndo },
        { label: redo ? `Redo ${redo.toLowerCase()}` : "Redo", keys: keyLabel("redo"), action: mapCommands.redo, disabled: () => !map.active?.canRedo },
        "-",
        { label: "Cut", keys: keyLabel("cut"), action: mapCommands.cut, disabled: noSelection },
        { label: "Copy", keys: keyLabel("copy"), action: mapCommands.copy, disabled: noSelection },
        { label: "Paste", keys: keyLabel("paste"), action: mapCommands.startPaste, disabled: noMap },
        { label: "Delete", keys: keyLabel("delete"), action: mapCommands.remove, disabled: noSelection },
        "-",
        { label: "Select everything on this floor", keys: keyLabel("selectAll"), action: mapCommands.selectAll, disabled: noMap },
        { label: "Deselect", keys: "Esc", action: mapCommands.clearSelection, disabled: noSelection },
        "-",
        { label: "Find on the map…", keys: keyLabel("find"), action: () => mapCommands.find(selectionScope()), disabled: noMap },
        { label: "Replace items…", keys: keyLabel("replace"), action: () => mapCommands.find({ ...selectionScope(), replace: true }), disabled: noMap },
        { label: "Go to anything…", keys: keyLabel("palette"), action: () => mapCommands.commandPalette() },
        "-",
        { label: "Automatic borders and walls", keys: keyLabel("automagic"), checked: () => !!map.settings?.automagic, action: mapCommands.toggleAutomagic },
      ],
    },
    {
      label: "View",
      items: [
        { label: "Zoom in", keys: keyLabel("zoomIn"), action: () => mapCommands.zoom(1), disabled: noMap },
        { label: "Zoom out", keys: keyLabel("zoomOut"), action: () => mapCommands.zoom(-1), disabled: noMap },
        { label: "Normal zoom", keys: keyLabel("zoomReset"), action: mapCommands.zoomReset, disabled: noMap },
        { label: "Floor up", keys: keyLabel("floorUp"), action: mapCommands.floorUp, disabled: noMap },
        { label: "Floor down", keys: keyLabel("floorDown"), action: mapCommands.floorDown, disabled: noMap },
        "-",
        toggle("lowerFloors", "Show lower floors", keyLabel("lowerFloors")),
        toggle("ghostHigher", "Show the floor above, faded", keyLabel("ghostHigher")),
        toggle("shade", "Darken lower floors"),
        toggle("grid", "Grid", keyLabel("grid")),
        toggle("clientBox", "What a player sees"),
        toggle("lights", "Lights", keyLabel("lights")),
        "-",
        toggle("creatures", "Creatures"),
        toggle("spawns", "Spawn areas"),
        "-",
        toggle("houses", "Houses"),
        toggle("special", "Protection zones and other areas"),
        toggle("zones", "Zones"),
        "-",
        toggle("markers", "Temples, house entrances and waypoints"),
        toggle("markedItems", "Marked items (invisible stairs…)"),
        "-",
        toggle("labels", "Names on the map"),
        toggle("coords", "Tile coordinates"),
        toggle("tooltips", "Item ids and attributes"),
        "-",
        toggle("animation", "Animations"),
        toggle("pathing", "Highlight blocking tiles"),
        toggle("highlightItems", "Highlight stacked items"),
        toggle("ghostItems", "Fade loose items"),
        toggle("onlyModified", "Only changed tiles"),
      ],
    },
    {
      label: "Map",
      items: [
        { label: "Properties and statistics", action: () => ((map.side = "world"), mapCommands.focus({ kind: "map" })), disabled: noMap },
        { label: "Convert map to client IDs", action: mapCommands.convertToClientIDs, disabled: () => noMap() || map.active?.itemMode !== "otb" || !!app.busy },
        { label: "Map tools…", action: () => (map.dialog = "tools"), disabled: noMap },
        {
          label: `Load warnings… (${map.activeId !== null ? warningCount(map.loadWarnings[map.activeId]).toLocaleString() : 0})`,
          action: () => (map.dialog = "warnings"),
          disabled: () => map.activeId === null || !map.loadWarnings[map.activeId]?.length,
        },
        "-",
        { label: "New town…", action: () => ((map.side = "world"), addTown()), disabled: noMap },
        { label: "New house", action: () => ((map.side = "world"), addHouse()), disabled: noMap },
        { label: "New waypoint…", action: () => ((map.side = "world"), addWaypoint()), disabled: noMap },
        "-",
        { label: "Items and brushes…", action: () => (map.dialog = "data"), disabled: noClient },
      ],
    },
  ];
}

/** Index (among tile items, ground excluded) of the topmost item with
 * text to read or write, or -1. */
export function readableIndex(tile: TileData | null | undefined): number {
  const items = map.items;
  if (!tile || !items) return -1;
  const ground = tile.flags & TileFlags.ground ? 1 : 0;
  for (let i = tile.ids.length - 1; i >= ground; i--) {
    const id = tile.ids[i];
    if (tile.itemFlags[i] & ItemFlags.text || items.is(id, "readable") || items.is(id, "writable")) return i - ground;
  }
  return -1;
}

export function openText(pos: Position, index: number): void {
  map.textItem = { pos, index };
  map.dialog = "text";
}

export function mapContextMenu(position: Position, tileData: TileData | null, actions: { pick: () => void }): MenuEntry[] {
  const items = map.items;
  const top = tileData && tileData.ids.length ? tileData.ids[tileData.ids.length - 1] : 0;
  const topIndex = tileData ? tileData.ids.length - 1 - (tileData.flags & TileFlags.ground ? 1 : 0) : -1;
  const out: MenuEntry[] = [
    { label: `Inspect tile ${fmtPos(position)}`, action: () => mapCommands.inspect(position) },
    { label: "Copy position", action: () => copy(fmtPos(position)) },
    { label: "Play from here", keys: keyLabel("play"), action: () => mapCommands.play(position) },
  ];
  const readable = readableIndex(tileData);
  if (readable >= 0) {
    out.push("-", { label: "Read and edit text…", keys: "Double click", action: () => openText(position, readable) });
  }
  if (tileData?.creatures?.length) {
    out.push("-", ...tileData.creatures.map((creature): MenuEntry => ({ label: `Use creature: ${creature.name}`, action: () => useCreature(creature) })));
  }
  if (top && items) {
    out.push(
      "-",
      { label: `Use brush: ${items.name(top)}`, keys: "Ctrl+Alt+click", action: actions.pick },
      { label: "Copy item id", action: () => copy(String(top)) },
      { label: "Copy item name", action: () => copy(items.name(top)) },
    );
    if (topIndex >= 0) {
      out.push(
        { label: "Rotate item", action: () => edit((id) => MapService.RotateItem(id, { pos: position, index: topIndex, path: [] })) },
        { label: "Open or close door", action: () => edit((id) => MapService.SwitchDoor(id, position, topIndex)) },
      );
    }
    if (tileData && tileData.itemFlags[tileData.ids.length - 1] & ItemFlags.teleport) {
      out.push({
        label: "Go to teleport destination",
        action: async () => {
          const tile = await MapService.Tile(map.activeId!, position);
          const topItem = tile.items?.[tile.items.length - 1];
          if (topItem?.ext?.dest) goTo(topItem.ext.dest);
        },
      });
    }
  }
  out.push(
    "-",
    { label: "Select tile", action: () => edit((id) => MapService.SelectTiles(id, [position], false)) },
    { label: "Copy", keys: keyLabel("copy"), action: mapCommands.copy, disabled: noSelection },
    { label: "Cut", keys: keyLabel("cut"), action: mapCommands.cut, disabled: noSelection },
    { label: "Paste here", action: () => edit((id) => MapService.Paste(id, position)) },
    { label: "Delete selection", keys: keyLabel("delete"), action: mapCommands.remove, disabled: noSelection },
  );
  const house = tileData?.house;
  if (house) {
    const name = map.active?.houses?.find((candidate) => candidate?.id === house)?.name ?? `House ${house}`;
    out.push("-", { label: `Show ${name}`, action: () => mapCommands.focus({ kind: "house", id: house }) }, { label: `Paint tiles of ${name}`, action: () => selectBrush({ kind: "house", house, label: name }) });
  }
  return out;
}
