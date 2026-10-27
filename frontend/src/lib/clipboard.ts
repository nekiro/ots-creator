import type { Properties, Thing } from "./api";

export type PasteMode = "object" | "properties";

let copied: Thing[] = [];

export function copyThings(things: Thing[]): void {
  copied = JSON.parse(JSON.stringify(things));
}

export function clipboardThings(): Thing[] {
  return copied;
}

export function clipboardThing(): Thing | null {
  return copied.length === 1 ? copied[0] : null;
}

export function clearClipboard(): void {
  copied = [];
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
