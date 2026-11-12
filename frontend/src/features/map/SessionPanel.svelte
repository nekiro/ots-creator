<script lang="ts">
	import { tick } from "svelte";
	import { collab, CollabService, peerColor, sendChat, type Peer } from "../../lib/map/collab.svelte";
	import { map } from "../../lib/map/state.svelte";
	import { goTo } from "../../lib/map/view/commands";
	import Icon from "../../lib/ui/Icon.svelte";
	import MiniWindow from "../../lib/ui/MiniWindow.svelte";

	let text = $state("");
	let log = $state<HTMLDivElement>();

	const title = $derived(collab.role === "host" ? "Hosting" : `In ${collab.status?.session.name ?? "session"}`);

	function follow(peer: Peer) {
		if (!peer.pos || collab.mapId === null) return;
		map.activeId = collab.mapId;
		goTo(peer.pos, collab.mapId);
	}

	async function send() {
		const message = text.trim();
		if (!message) return;
		text = "";
		await sendChat(message);
	}

	function toggleChat() {
		collab.chatOpen = !collab.chatOpen;
		if (collab.chatOpen) collab.unread = 0;
	}

	$effect(() => {
		void collab.chat.length;
		if (collab.chatOpen) void tick().then(() => log?.scrollTo({ top: log.scrollHeight }));
	});

	function time(when: number): string {
		return new Date(when).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
	}
</script>

<MiniWindow {title} class="session">
	{#snippet actions()}
		<button class="t-icon-btn" title="Invite code and session" onclick={() => (map.dialog = "collab")}
			><Icon name="people" /></button
		>
	{/snippet}
	<div class="peers">
		{#each collab.peers as peer (peer.id)}
			{@const live = collab.others.find((other) => other.id === peer.id) ?? peer}
			<div class="peer">
				<span class="swatch" style:background={peer.color}></span>
				<button
					class="pname"
					disabled={!live.pos || peer.id === collab.status?.you}
					title={live.pos ? `Go to ${live.pos.x}, ${live.pos.y}, ${live.pos.z}` : ""}
					onclick={() => follow(live)}
					>{peer.name}{peer.id === collab.status?.you ? " (you)" : ""}{peer.host ? " · host" : ""}</button
				>
				{#if live.brush && peer.id !== collab.status?.you}<span class="t-label brush">{live.brush}</span>{/if}
				{#if collab.role === "host" && !peer.host}
					<button
						class="t-icon-btn"
						title="Remove from the session"
						onclick={() => CollabService.Kick(peer.id)}><Icon name="close" /></button
					>
				{/if}
			</div>
		{/each}
	</div>
	<button class="chathead t-label" onclick={toggleChat}>
		Chat{collab.unread ? ` (${collab.unread} new)` : ""}
		<span class="fold">{collab.chatOpen ? "▾" : "▸"}</span>
	</button>
	{#if collab.chatOpen}
		<div class="log t-panel" bind:this={log}>
			{#each collab.chat as message, index (index)}
				<div class="msg">
					<span class="t-label when">{time(message.time)}</span>
					<span class="who" style:color={peerColor(message.peer)}>{message.name}:</span>
					<span class="text">{message.text}</span>
				</div>
			{:else}
				<div class="t-label empty">No messages yet.</div>
			{/each}
		</div>
		<input
			class="t-input"
			placeholder="Message, Enter sends"
			maxlength="1000"
			bind:value={text}
			onkeydown={(event) => {
				event.stopPropagation();
				if (event.key === "Enter") void send();
			}}
		/>
	{/if}
</MiniWindow>

<style>
	.peers {
		display: flex;
		flex-direction: column;
		gap: 2px;
		max-height: 140px;
		overflow-y: auto;
	}
	.peer {
		display: flex;
		align-items: center;
		gap: 6px;
		min-height: 20px;
	}
	.swatch {
		width: 10px;
		height: 10px;
		flex: none;
		border: 1px solid rgba(0, 0, 0, 0.6);
	}
	.pname {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-align: left;
		background: none;
		border: none;
		padding: 0;
		color: var(--text-bright);
		font: inherit;
		cursor: pointer;
	}
	.pname:disabled {
		cursor: default;
	}
	.pname:not(:disabled):hover {
		text-decoration: underline;
	}
	.brush {
		max-width: 90px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 11px;
	}
	.chathead {
		display: flex;
		align-items: center;
		margin-top: 6px;
		padding: 0;
		background: none;
		border: none;
		cursor: pointer;
		font: inherit;
		color: var(--gold);
	}
	.fold {
		margin-left: auto;
	}
	.log {
		height: 120px;
		overflow-y: auto;
		padding: 3px 5px;
		margin: 3px 0;
		font-size: 12px;
		user-select: text;
	}
	.msg {
		word-break: break-word;
	}
	.when {
		font-size: 10px;
		margin-right: 3px;
	}
	.who {
		font-weight: bold;
		margin-right: 3px;
	}
	.empty {
		font-size: 11px;
	}
</style>
