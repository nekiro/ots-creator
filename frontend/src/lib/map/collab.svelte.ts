// Mapping together (internal/collab): one editor hosts a map, others join
// it. The backend shares the edits; this module follows the session for the
// UI: who is in it, where they point on the map, and the chat.
import { Events } from "@wailsio/runtime";
import * as CollabService from "../../../bindings/github.com/nekiro/ots-creator/internal/app/collabservice";
import type {
	CollabEvent,
	CollabSettings,
	CollabStatus,
	HostRequest,
	KnownSession,
} from "../../../bindings/github.com/nekiro/ots-creator/internal/app/models";
import type {
	Chat,
	Found,
	Peer,
	Presence,
	SessionInfo,
} from "../../../bindings/github.com/nekiro/ots-creator/internal/collab/models";
import { errorMessage } from "../api";
import { toast } from "../state/app.svelte";
import type { Position } from "./data/api";
import { applyChange, map } from "./state.svelte";

export { CollabService };
export type { Chat, CollabSettings, CollabStatus, Found, HostRequest, KnownSession, Peer, SessionInfo };

/** How often at most the own cursor is sent to the others. */
const PRESENCE_EVERY_MS = 80;

class CollabState {
	status = $state.raw<CollabStatus | null>(null);
	/** Where each peer points (by peer id), updated more often than the status. */
	presence = $state.raw<Record<number, Presence>>({});
	chat = $state.raw<Chat[]>([]);
	/** Chat messages that came while the session panel was folded. */
	unread = $state(0);
	chatOpen = $state(true);

	readonly role = $derived(this.status?.role ?? "");
	readonly active = $derived(this.role === "host" || this.role === "guest");
	readonly mapId = $derived(this.active ? (this.status?.mapId ?? null) : null);
	readonly peers = $derived<Peer[]>((this.status?.peers ?? []).filter((peer): peer is Peer => !!peer));
	/** The other peers with where they point now. */
	readonly others = $derived<Peer[]>(
		this.peers
			.filter((peer) => peer.id !== this.status?.you)
			.map((peer) => {
				const moved = this.presence[peer.id];
				return moved ? { ...peer, pos: moved.pos, brush: moved.brush } : peer;
			}),
	);
}

export const collab = new CollabState();

function setStatus(status: CollabStatus): void {
	const previous = collab.status;
	collab.status = status;
	collab.chat = (status.chat ?? []).filter((chat): chat is Chat => !!chat);
	if (!status.role || status.role === "joining") collab.presence = {};
	if (status.error && status.error !== previous?.error) toast(status.error, "error", 12000);
	if (previous?.role === "host" || previous?.role === "guest") {
		const before = new Set((previous.peers ?? []).map((peer) => peer?.id));
		const after = new Set((status.peers ?? []).map((peer) => peer?.id));
		for (const peer of status.peers ?? [])
			if (peer && !before.has(peer.id) && peer.id !== status.you) toast(`${peer.name} joined the map.`, "info");
		for (const peer of previous.peers ?? [])
			if (peer && !after.has(peer.id) && status.role) toast(`${peer.name} left the map.`, "info");
	}
}

/** Follows the session; returns the function that stops. */
export function followCollab(): () => void {
	void CollabService.Status().then(setStatus);
	return Events.On("collab:event", (event: { data: CollabEvent }) => {
		const data = event.data;
		if (!data) return;
		switch (data.kind) {
			case "status":
				if (data.status) setStatus(data.status);
				break;
			case "change":
				applyChange(data.id, data.change);
				break;
			case "presence":
				if (data.presence) collab.presence = { ...collab.presence, [data.presence.peer]: data.presence };
				break;
			case "chat":
				if (data.chat) {
					collab.chat = [...collab.chat, data.chat];
					if (!collab.chatOpen && data.chat.peer !== collab.status?.you) collab.unread++;
				}
				break;
		}
	});
}

let presenceTimer = 0;
let presenceDue: { id: number; pos: Position | null; brush: string } | null = null;
let presenceLast = "";

/** Shares where this editor points on the shared map (throttled). */
export function sendPresence(id: number | null, pos: Position | null): void {
	if (!collab.active || id === null || id !== collab.mapId) return;
	presenceDue = { id, pos, brush: map.tool === "brush" ? (map.brush?.label ?? "") : "" };
	if (presenceTimer) return;
	presenceTimer = window.setTimeout(() => {
		presenceTimer = 0;
		const due = presenceDue;
		presenceDue = null;
		if (!due) return;
		const key = JSON.stringify(due);
		if (key === presenceLast) return;
		presenceLast = key;
		void CollabService.SetPresence(due.id, due.pos, due.brush).catch(() => {});
	}, PRESENCE_EVERY_MS);
}

export async function sendChat(text: string): Promise<void> {
	try {
		await CollabService.SendChat(text);
	} catch (err) {
		toast(errorMessage(err), "error");
	}
}

export async function leaveSession(): Promise<void> {
	await CollabService.Leave();
}

/** Peer color for a peer id, also for peers that left. */
export function peerColor(id: number): string {
	return collab.peers.find((peer) => peer.id === id)?.color ?? "#ccc";
}

export function relativeTime(when: number): string {
	const minutes = Math.round((Date.now() - when) / 60000);
	if (minutes < 1) return "just now";
	if (minutes < 60) return `${minutes} min ago`;
	const hours = Math.round(minutes / 60);
	if (hours < 48) return `${hours} h ago`;
	return `${Math.round(hours / 24)} days ago`;
}
