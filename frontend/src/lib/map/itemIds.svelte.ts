// Before a map opens or is created, the user confirms how it stores its item
// ids: server ids (translated with the items.otb of the client) or client
// ids. ItemIdsDialog (mounted in App) renders the pending request;
// chooseItemIds resolves with the choice, null when canceled.

import type { ItemIDsChoice, ItemIDsPlan } from "./data/api";

export interface ItemIdsRequest {
	/** "Open map" or "New map". */
	title: string;
	/** File name of the map; "" for a new map. */
	name: string;
	plan: ItemIDsPlan;
}

export const itemIdsState = $state<{
	request: (ItemIdsRequest & { resolve: (choice: ItemIDsChoice | null) => void }) | null;
}>({ request: null });

export function chooseItemIds(request: ItemIdsRequest): Promise<ItemIDsChoice | null> {
	itemIdsState.request?.resolve(null);
	return new Promise((resolve) => {
		itemIdsState.request = { ...request, resolve };
	});
}

export function answerItemIds(choice: ItemIDsChoice | null): void {
	const request = itemIdsState.request;
	itemIdsState.request = null;
	request?.resolve(choice);
}

/** Short label of an item id mode for the status bar and dialogs. */
export function itemIdsLabel(mode: string): string {
	return mode === "client" ? "client ids" : "server ids";
}
