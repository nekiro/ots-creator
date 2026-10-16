// Client versions offered by the version pickers.
import { ProjectService, type Version } from "./api";

/** The known client versions, newest first. */
export async function loadVersions(): Promise<Version[]> {
  return ((await ProjectService.Versions()) ?? []).slice().reverse();
}

/** Picker value of a version: versions can share a number or a name. */
export const keyOfVersion = (version: Version) => `${version.value}|${version.name}|${version.datSignature}|${version.sprSignature}`;
