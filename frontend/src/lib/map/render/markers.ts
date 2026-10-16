// Waypoints, house entrances and temples marked on the map.
import type { MapInfo } from "../data/api";
import { LABEL_FONT, MARKER_COLORS } from "../constants";
import type { Marker } from "./scene";

/** "Paupers Palace, Flat 07" -> "Flat 07": neighbors share the building. */
export function houseLabel(name: string): string {
  return name.slice(name.lastIndexOf(",") + 1).trim() || name;
}

/** Cuts a long name with an ellipsis. */
export function shortLabel(name: string): string {
  const max = LABEL_FONT.maxMarkerChars;
  return name.length > max ? name.slice(0, max - 1).trimEnd() + "…" : name;
}

/** Markers of a map. The color tells the kind, so labels carry only a short name. */
export function markersOf(info: Pick<MapInfo, "waypoints" | "houses" | "towns"> | null): Marker[] {
  const markers: Marker[] = [];
  if (!info) return markers;
  for (const waypoint of info.waypoints ?? []) {
    if (waypoint) markers.push({ ...waypoint.pos, label: shortLabel(waypoint.name), full: waypoint.name, color: MARKER_COLORS.waypoint });
  }
  for (const house of info.houses ?? []) {
    if (house.exit && (house.exit.x || house.exit.y)) markers.push({ ...house.exit, label: shortLabel(houseLabel(house.name)), full: house.name, color: MARKER_COLORS.houseExit });
  }
  for (const town of info.towns ?? []) {
    if (town && town.temple.x) markers.push({ ...town.temple, label: shortLabel(town.name), full: town.name, color: MARKER_COLORS.temple });
  }
  return markers;
}
