// The attribute map of an items.xml on client ids: which attributes the
// properties of a client item write (as mappedAttributes of the backend).
import type { Properties, XMLAttribute, XMLItemData, XMLMapping } from "./api";
import { getPath } from "./flags";

export interface MappedValue {
	key: string;
	value: string;
	set: boolean;
	/** Other keys the server reads as the key, in lower case. */
	aliases: string[];
}

/** What the map writes for the properties, by attribute key in lower case. */
export function mappedAttributes(mapping: XMLMapping[], props: Properties): Map<string, MappedValue> {
	const out = new Map<string, MappedValue>();
	for (const row of mapping) {
		const raw = getPath(props, row.client);
		const written: MappedValue = { key: row.key, value: "", set: false, aliases: [] };
		if (raw !== undefined && !raw === !!row.not) {
			written.set = true;
			written.value = row.value || (row.not || typeof raw === "boolean" ? "1" : String(raw));
		}
		// Several properties can write one key: the first that is set does,
		// and the key has the aliases of all of them.
		const key = row.key.toLowerCase();
		const existing = out.get(key);
		const aliases = [...(existing?.aliases ?? []), ...(row.aliases ?? []).map((alias) => alias.toLowerCase())];
		out.set(key, { ...(existing?.set ? existing : written), aliases });
	}
	return out;
}

const names = (written: MappedValue, key: string) =>
	key.toLowerCase() === written.key.toLowerCase() || written.aliases.includes(key.toLowerCase());

/**
 * Makes the entry say what the map writes: the attribute with the key or an
 * alias of it gets the value (one that reads the same is kept as written),
 * or is removed when the item does not have it.
 */
function setAttribute(item: XMLItemData, written: MappedValue): void {
	const attributes: XMLAttribute[] = (item.attributes ??= []);
	const index = attributes.findIndex((attribute) => names(written, attribute.key));
	if (!written.set) {
		for (let i = attributes.length - 1; i >= 0; i--) if (names(written, attributes[i].key)) attributes.splice(i, 1);
	} else if (index >= 0) {
		if (!sameValue(attributes[index].value, written.value)) attributes[index].value = written.value;
	} else {
		attributes.push({ key: written.key, value: written.value, children: [] });
	}
}

/** Writes the mapped attributes that differ between two states of the properties to the entry. */
export function applyMappedChanges(
	item: XMLItemData,
	before: Map<string, MappedValue>,
	after: Map<string, MappedValue>,
): void {
	for (const [key, now] of after) {
		const was = before.get(key);
		if (was && was.set === now.set && was.value === now.value) continue;
		setAttribute(item, now);
	}
}

/** Makes every mapped attribute of the entry what the properties say. */
export function fillFromClient(item: XMLItemData, mapping: XMLMapping[], props: Properties, name: string): void {
	if (!item.name && name) item.name = name;
	for (const written of mappedAttributes(mapping, props).values()) setAttribute(item, written);
}

/**
 * Gives the entry the attributes copied from another entry. The attributes
 * of the map stay as the entry has them and are not pasted: the client says
 * those for every item.
 */
export function pasteAttributes(item: XMLItemData, copied: XMLAttribute[], mapping: XMLMapping[]): void {
	const mapped = [...mappedAttributes(mapping, {} as Properties).values()];
	const isMapped = (attribute: XMLAttribute) => mapped.some((written) => names(written, attribute.key));
	const pasted = JSON.parse(JSON.stringify(copied.filter((attribute) => !isMapped(attribute)))) as XMLAttribute[];
	item.attributes = [...(item.attributes ?? []).filter(isMapped), ...pasted];
}

const TRUE_VALUES = new Set(["1", "true", "yes"]);
// Servers read flags as booleans: "1" and "true" are the same value.
const sameValue = (a: string, b: string) =>
	a === b || (TRUE_VALUES.has(a.toLowerCase()) && TRUE_VALUES.has(b.toLowerCase()));

/** Where the mapped attributes of the entry are not what the properties say, one line each. */
export function mappedDifferences(item: XMLItemData, mapping: XMLMapping[], props: Properties): string[] {
	const out: string[] = [];
	for (const written of mappedAttributes(mapping, props).values()) {
		const attribute = (item.attributes ?? []).find((candidate) => names(written, candidate.key));
		if (written.set && !attribute) out.push(`${written.key}="${written.value}" is missing`);
		else if (written.set && attribute && !sameValue(attribute.value, written.value))
			out.push(`${attribute.key} is "${attribute.value}", the client says "${written.value}"`);
		else if (!written.set && attribute && !["0", "false", "no"].includes(attribute.value.toLowerCase()))
			out.push(`${attribute.key}="${attribute.value}" is not set in the client`);
	}
	return out;
}
