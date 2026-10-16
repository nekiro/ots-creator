// What a drop of files onto the window means, from the paths and the
// element they were dropped on.
import { OBJECT_EXT } from "../api";

/** Id of the preview's file drop target: images dropped there are sheets. */
export const PREVIEW_DROP = "preview-drop";
/** Ids of object list cells as file drop targets: an image dropped on an
 * object is a sheet for that object. */
export const thingDropId = (id: number) => `thing-drop-${id}`;
const THING_DROP = /^thing-drop-(\d+)$/;
export const IMAGE_EXT = /\.(png|bmp|gif|jpe?g)$/i;
const CLIENT_EXT = /(\.(dat|spr|otfi)|catalog-content\.json)$/i;
const HAS_EXT = /\.[^\\/]+$/;

export type DropAction =
  /** One image on the preview: a sheet of the focused object. */
  | { kind: "previewSheet"; path: string }
  /** One image on an object of the list: a sheet of that object. */
  | { kind: "thingSheet"; path: string; id: number }
  /** Objects or images without an open client: shown in the viewer. */
  | { kind: "view"; path: string }
  | { kind: "importObjects"; paths: string[] }
  | { kind: "importImages"; paths: string[] }
  /** A client file or folder. */
  | { kind: "openClient"; path: string }
  | { kind: "unknown" };

export interface DropContext {
  /** A client is open. */
  open: boolean;
  /** An object is focused. */
  focused: boolean;
}

export function classifyDrop(paths: string[], target: string, context: DropContext): DropAction {
  const sheet = context.open && paths.length === 1 && IMAGE_EXT.test(paths[0]) ? paths[0] : null;
  if (sheet && target === PREVIEW_DROP && context.focused) return { kind: "previewSheet", path: sheet };
  const onThing = THING_DROP.exec(target);
  if (sheet && onThing) return { kind: "thingSheet", path: sheet, id: Number(onThing[1]) };
  const objects = paths.filter((path) => OBJECT_EXT.test(path));
  const images = paths.filter((path) => IMAGE_EXT.test(path));
  const client = paths.find((path) => CLIENT_EXT.test(path) || !HAS_EXT.test(path));
  if ((objects.length || images.length) && !context.open) return { kind: "view", path: objects[0] ?? images[0] };
  if (objects.length) return { kind: "importObjects", paths: objects };
  if (images.length && context.open) return { kind: "importImages", paths: images };
  if (client) return { kind: "openClient", path: client };
  return { kind: "unknown" };
}
