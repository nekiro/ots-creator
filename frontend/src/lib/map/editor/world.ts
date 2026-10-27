import { ask, askText } from "../../confirm.svelte";
import { toast } from "../../state/app.svelte";
import { MapService, type HouseInfo, type Position, type Town } from "../data/api";
import { goTo, mapCommands, spriteSize } from "../view/commands";
import { worldToPosition } from "../view/coords";
import { edit, map, selectBrush } from "../state.svelte";

export function viewCenter(): Position {
  const camera = map.cam;
  return worldToPosition(camera.x, camera.y, camera.floor, spriteSize());
}

/** The entries of a list from the map info without its gaps, kept per list:
 * map infos are replaced, never changed in place (map.maps is raw state). */
const presentCache = new WeakMap<object, unknown[]>();
const NOTHING: never[] = [];
function present<Entry>(list: (Entry | null)[] | null | undefined): Entry[] {
  if (!list) return NOTHING;
  let entries = presentCache.get(list) as Entry[] | undefined;
  if (!entries) presentCache.set(list, (entries = list.filter((entry): entry is Entry => !!entry)));
  return entries;
}

export function towns(): Town[] {
  return present(map.active?.towns);
}

export function houses(): HouseInfo[] {
  return present(map.active?.houses);
}

const townNamesCache = new WeakMap<object, Map<number, string>>();

export function townName(id: number): string {
  const list = map.active?.towns;
  if (!list) return "No town";
  let names = townNamesCache.get(list);
  if (!names) {
    names = new Map();
    // The first town with an id wins, as a search from the start would.
    for (const town of present(list)) if (!names.has(town.id)) names.set(town.id, town.name);
    townNamesCache.set(list, names);
  }
  return names.get(id) ?? "No town";
}

export async function addTown(): Promise<void> {
  const name = await askText({ title: "New town", label: "Name", value: `Town ${towns().length + 1}`, ok: "Add" });
  if (!name) return;
  const change = await edit((id) => MapService.SaveTown(id, { id: 0, name, temple: viewCenter() }));
  if (change?.count) {
    mapCommands.focus({ kind: "town", id: change.count });
    toast("The temple is at the center of the view. Move it from the inspector.");
  }
}

export async function saveTown(town: Town): Promise<void> {
  await edit((id) => MapService.SaveTown(id, town));
}

export async function removeTown(town: Town): Promise<void> {
  const used = houses().filter((house) => house.townId === town.id).length;
  const message = used ? `${town.name} has ${used} house${used === 1 ? "" : "s"}; they keep their tiles but lose their town.` : `Remove ${town.name}?`;
  if (!(await ask({ title: "Remove town", message, ok: "Remove" }))) return;
  await edit((id) => MapService.RemoveTown(id, town.id));
  map.focus = null;
}

export async function addHouse(townId = 0): Promise<void> {
  const number = houses().length + 1;
  const change = await edit((id) => MapService.SaveHouse(id, { id: 0, name: `House ${number}`, townId: townId || towns()[0]?.id || 0, rent: 0, guildhall: false, exit: { x: 0, y: 0, z: 0 } }));
  if (!change?.count) return;
  const house = change.info.houses?.find((candidate) => candidate?.id === change.count);
  if (house) {
    mapCommands.focus({ kind: "house", id: house.id });
    paintHouse(house);
  }
}

export function paintHouse(house: HouseInfo): void {
  selectBrush({ kind: "house", house: house.id, label: house.name });
}

export async function saveHouse(house: HouseInfo): Promise<void> {
  const { id: houseId, name, townId, rent, guildhall, exit, clientId, beds } = house;
  await edit((id) => MapService.SaveHouse(id, { id: houseId, name, townId, rent, guildhall, exit, clientId, beds }));
}

export async function removeHouse(house: HouseInfo): Promise<void> {
  if (!(await ask({ title: "Remove house", message: `Remove ${house.name}? Its tiles stop being house tiles.`, ok: "Remove" }))) return;
  await edit((id) => MapService.RemoveHouse(id, house.id));
  map.focus = null;
}

export async function goHouse(house: HouseInfo): Promise<void> {
  if (house.exit?.x) return goTo(house.exit);
  const tiles = await MapService.HouseTiles(map.activeId!, house.id);
  if (tiles?.length) goTo(tiles[0]);
  else toast("The house has no tiles yet: paint them on the map.");
}

export async function selectHouseTiles(house: HouseInfo): Promise<void> {
  const tiles = await MapService.HouseTiles(map.activeId!, house.id);
  if (!tiles?.length) return toast("The house has no tiles yet.");
  await edit((id) => MapService.SelectTiles(id, tiles, false));
  goTo(tiles[0]);
}

export async function addWaypoint(): Promise<void> {
  const name = await askText({ title: "New waypoint", label: "Name", ok: "Place on the map" });
  if (!name) return;
  if (map.active?.waypoints?.some((waypoint) => waypoint?.name === name)) return toast(`There is already a waypoint "${name}".`, "error");
  selectBrush({ kind: "waypoint", waypoint: name, label: `Waypoint ${name}` });
  map.focus = { kind: "waypoint", name };
  toast(`Click the map to place "${name}".`);
}

export async function renameWaypoint(old: string, name: string): Promise<void> {
  const waypoint = map.active?.waypoints?.find((candidate) => candidate?.name === old);
  if (!name || !waypoint || name === old) return;
  const change = await edit((id) => MapService.SaveWaypoint(id, old, { name, pos: waypoint.pos }));
  if (change && map.focus?.kind === "waypoint") map.focus = { kind: "waypoint", name };
}

export async function removeWaypoint(name: string): Promise<void> {
  await edit((id) => MapService.RemoveWaypoint(id, name));
  map.focus = null;
}

export async function addZone(): Promise<void> {
  const name = await askText({ title: "New zone", label: "Name", ok: "Add" });
  if (!name) return;
  const change = await edit((id) => MapService.SaveZone(id, { id: 0, name }));
  if (change?.count) {
    map.focus = { kind: "zone", id: change.count };
    selectBrush({ kind: "zone", zone: change.count, label: `Zone ${name}` });
  }
}

export async function removeZone(zoneId: number): Promise<void> {
  await edit((id) => MapService.RemoveZone(id, zoneId));
  map.focus = null;
}

export function pickPosition(label: string, done: (position: Position) => void): void {
  map.pick = { label, done };
}
