import type { Properties, Thing, XMLAttribute } from "./api";

export type PasteMode = "object" | "properties";

let copied: Thing[] = [];
let copiedAttributes: XMLAttribute[] | null = null;

/**
 * Copies the objects. A single item goes with the attributes of its
 * items.xml entry when an items.xml is attached.
 */
export function copyThings(things: Thing[], serverAttributes: XMLAttribute[] | null = null): void {
	copied = JSON.parse(JSON.stringify(things));
	copiedAttributes = serverAttributes && JSON.parse(JSON.stringify(serverAttributes));
}

/** The items.xml attributes of the copied item; null when it was copied without. */
export function clipboardServerAttributes(): XMLAttribute[] | null {
	return copiedAttributes;
}

export function clipboardThings(): Thing[] {
	return copied;
}

export function clipboardThing(): Thing | null {
	return copied.length === 1 ? copied[0] : null;
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
