// Object clipboard: copy a whole object or only its properties onto other
// objects. The copy is a plain snapshot, independent of later edits.
import type { Properties, Thing } from "./api";

export type PasteMode = "object" | "properties";

let copied: Thing | null = null;

export function copyThing(t: Thing): void {
  copied = JSON.parse(JSON.stringify(t));
}

export function clipboardThing(): Thing | null {
  return copied;
}

export function clearClipboard(): void {
  copied = null;
}

/**
 * Returns target with the clipboard pasted in. Pasting a whole object keeps
 * the target id and needs the same category; properties paste anywhere.
 */
export function pasteInto(target: Thing, source: Thing, mode: PasteMode): Thing {
  const src = JSON.parse(JSON.stringify(source)) as Thing;
  if (mode === "properties") return { ...target, props: src.props as Properties };
  if (src.category !== target.category) throw new Error("Objects can only be pasted onto the same category.");
  return { ...src, id: target.id, category: target.category };
}
