import { describe, expect, it } from "vitest";
import type { Properties, XMLItemData, XMLMapping } from "./api";
import { applyMappedChanges, fillFromClient, mappedAttributes, mappedDifferences, pasteAttributes } from "./itemsXml";

const mapping: XMLMapping[] = [
	{ client: "pickupable", key: "pickupable" },
	{ client: "unmoveable", key: "moveable", not: true },
	{ client: "groundSpeed", key: "groundspeed" },
	{ client: "unpassable", key: "blocksolid", value: "true", aliases: ["Blocking"] },
	{ client: "writable", key: "writeable" },
	{ client: "writableOnce", key: "writeable" },
];

const props = (set: Partial<Properties>) =>
	({
		pickupable: false,
		unmoveable: false,
		groundSpeed: 0,
		unpassable: false,
		writable: false,
		writableOnce: false,
		...set,
	}) as Properties;

const entry = (): XMLItemData => ({
	name: "",
	article: "",
	plural: "",
	fields: [],
	attributes: [{ key: "weight", value: "5", children: [] }],
});

describe("items.xml attribute map", () => {
	it("writes what the properties say", () => {
		const mapped = mappedAttributes(
			mapping,
			props({ pickupable: true, groundSpeed: 150, unpassable: true, writableOnce: true }),
		);
		expect(mapped.get("pickupable")).toEqual({ key: "pickupable", value: "1", set: true, aliases: [] });
		expect(mapped.get("moveable")).toEqual({ key: "moveable", value: "1", set: true, aliases: [] });
		expect(mapped.get("blocksolid")?.aliases).toEqual(["blocking"]);
		expect(mapped.get("groundspeed")?.value).toBe("150");
		expect(mapped.get("blocksolid")?.value).toBe("true");
		expect(mapped.get("writeable")?.set).toBe(true);
		expect(mappedAttributes(mapping, props({ unmoveable: true })).get("moveable")?.set).toBe(false);
	});

	it("follows a changed property only", () => {
		const item = entry();
		applyMappedChanges(
			item,
			mappedAttributes(mapping, props({})),
			mappedAttributes(mapping, props({ pickupable: true })),
		);
		expect(item.attributes).toEqual([
			{ key: "weight", value: "5", children: [] },
			{ key: "pickupable", value: "1", children: [] },
		]);
		applyMappedChanges(
			item,
			mappedAttributes(mapping, props({ pickupable: true })),
			mappedAttributes(mapping, props({})),
		);
		expect(item.attributes).toEqual([{ key: "weight", value: "5", children: [] }]);
	});

	it("reads an alias as the key", () => {
		const item = entry();
		item.attributes = [{ key: "blocking", value: "1", children: [] }];
		expect(mappedDifferences(item, mapping, props({ unpassable: true }))).toEqual(['moveable="1" is missing']);
		fillFromClient(item, mapping, props({ unpassable: true }), "");
		expect(item.attributes).toEqual([
			{ key: "blocking", value: "1", children: [] },
			{ key: "moveable", value: "1", children: [] },
		]);
		expect(mappedDifferences(item, mapping, props({}))).toEqual(['blocking="1" is not set in the client']);
		fillFromClient(item, mapping, props({}), "");
		expect(item.attributes?.map((attribute) => attribute.key)).toEqual(["moveable"]);
	});

	it("fills an entry from the client", () => {
		const item = entry();
		fillFromClient(item, mapping, props({ pickupable: true }), "sword");
		expect(item.name).toBe("sword");
		expect(item.attributes?.map((attribute) => attribute.key)).toEqual(["weight", "pickupable", "moveable"]);
	});
	it("pastes copied attributes, keeping the mapped ones of the entry", () => {
		const item = entry();
		item.attributes = [
			{ key: "Pickupable", value: "1", children: [] },
			{ key: "armor", value: "4", children: [] },
		];
		const copied = [
			{ key: "weight", value: "3500", children: [{ key: "inner", value: "1", children: [] }] },
			{ key: "moveable", value: "1", children: [] },
		];
		pasteAttributes(item, copied, mapping);
		expect(item.attributes.map((attribute) => attribute.key)).toEqual(["Pickupable", "weight"]);
		// The pasted attributes are copies.
		item.attributes[1].children![0].value = "2";
		expect(copied[0].children[0].value).toBe("1");
		pasteAttributes(item, copied, []);
		expect(item.attributes.map((attribute) => attribute.key)).toEqual(["weight", "moveable"]);
	});
	it("lists where an entry differs from the client", () => {
		const item = entry();
		item.attributes = [
			{ key: "Pickupable", value: "true", children: [] },
			{ key: "groundspeed", value: "100", children: [] },
			{ key: "blocksolid", value: "1", children: [] },
			{ key: "writeable", value: "0", children: [] },
		];
		expect(mappedDifferences(item, mapping, props({ pickupable: true, groundSpeed: 150 }))).toEqual([
			'moveable="1" is missing',
			'groundspeed is "100", the client says "150"',
			'blocksolid="1" is not set in the client',
		]);
		fillFromClient(item, mapping, props({ pickupable: true, groundSpeed: 150 }), "");
		expect(mappedDifferences(item, mapping, props({ pickupable: true, groundSpeed: 150 }))).toEqual([]);
	});
});
