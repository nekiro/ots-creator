<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import { errorMessage } from "../../../lib/api";
	import {
		collab,
		CollabService,
		leaveSession,
		relativeTime,
		type Found,
		type KnownSession,
		type SessionInfo,
	} from "../../../lib/map/collab.svelte";
	import { map } from "../../../lib/map/state.svelte";
	import { mapCommands } from "../../../lib/map/view/commands";
	import { toast } from "../../../lib/state/app.svelte";
	import Dialog from "../../../lib/ui/Dialog.svelte";
	import Icon from "../../../lib/ui/Icon.svelte";

	const BROWSE_EVERY_MS = 4000;

	const settings = collab.status?.settings;
	let tab = $state<"join" | "host">(map.activeId !== null && !collab.active ? "host" : "join");
	let name = $state(settings?.name ?? "");
	let code = $state("");
	let password = $state("");
	let busy = $state(false);

	let session = $state(settings?.session ?? "");
	let hostPassword = $state("");
	let port = $state(settings?.port || 47816);
	let upnp = $state(settings?.upnp ?? true);
	let discoverable = $state(settings?.discoverable ?? true);

	let found = $state.raw<Found[]>([]);
	let browsing = $state(false);
	let known = $state.raw<KnownSession[]>([]);
	/** Live state of the known sessions by key: null while asking. */
	let online = $state.raw<Record<string, SessionInfo | false | null>>({});
	/** A found or known session that needs a password before joining. */
	let askPassword = $state<{ code: string; title: string; fingerprint: string } | null>(null);

	const close = () => (map.dialog = null);
	const hostedMap = $derived(map.maps.find((open) => open.id === (collab.mapId ?? map.activeId)) ?? null);

	let browseTimer = 0;
	async function browse() {
		if (browsing || collab.active) return;
		browsing = true;
		try {
			found = ((await CollabService.Browse()) ?? []).filter((entry): entry is Found => !!entry);
		} catch {
			found = [];
		} finally {
			browsing = false;
		}
	}

	async function loadKnown() {
		known = ((await CollabService.KnownSessions()) ?? []).filter((entry): entry is KnownSession => !!entry);
		online = Object.fromEntries(known.map((entry) => [entry.key, null]));
		for (const entry of known) {
			CollabService.Probe(entry.invite)
				.then((info) => (online = { ...online, [entry.key]: info }))
				.catch(() => (online = { ...online, [entry.key]: false }));
		}
	}

	onMount(() => {
		void browse();
		void loadKnown();
		browseTimer = window.setInterval(() => void browse(), BROWSE_EVERY_MS);
	});
	onDestroy(() => clearInterval(browseTimer));

	async function join(joinCode: string, joinPassword = "", fingerprint = "") {
		if (!joinCode.trim()) return;
		busy = true;
		close();
		await mapCommands.join(joinCode.trim(), joinPassword, name, fingerprint);
		busy = false;
	}

	function joinFound(entry: Found) {
		if (entry.password) askPassword = { code: entry.address, title: entry.name, fingerprint: entry.fingerprint };
		else void join(entry.address, "", entry.fingerprint);
	}

	function joinAsked() {
		const target = askPassword;
		askPassword = null;
		if (target) void join(target.code, password, target.fingerprint);
	}

	function joinKnown(entry: KnownSession) {
		void join(entry.invite);
	}

	async function forget(entry: KnownSession) {
		await CollabService.ForgetSession(entry.key);
		known = known.filter((other) => other.key !== entry.key);
	}

	async function host() {
		if (!hostedMap) return;
		busy = true;
		try {
			await CollabService.Host({
				mapId: hostedMap.id,
				session,
				name,
				password: hostPassword,
				port: Number(port) || 47816,
				upnp,
				discoverable,
			});
		} catch (err) {
			toast(errorMessage(err), "error", 10000);
		} finally {
			busy = false;
		}
	}

	async function copyInvite() {
		const invite = collab.status?.invite;
		if (!invite) return;
		await navigator.clipboard?.writeText(invite);
		toast("Invite code copied: send it to the people you map with.", "success");
	}

	async function end() {
		await leaveSession();
		close();
	}

	const network = $derived(collab.status?.network);
	const upnpText = $derived.by(() => {
		const state = network?.upnp ?? "off";
		if (state === "off") return "not used";
		if (state === "working") return "asking the router…";
		if (state === "ok") return `the router forwards port ${collab.status?.session.port}`;
		return `failed: ${state}`;
	});
</script>

<Dialog title="Map together" width={560} onclose={close}>
	<div class="col form">
		{#if collab.role === "host"}
			<div class="sect">
				<div class="h">You are hosting {collab.status?.session.name}</div>
				<p class="t-label hint">
					Send the invite code to the people you map with. In OTS Creator they choose <b>Map together</b> ›
					<b>Join</b> and paste it. The code holds your addresses, the key of this computer and the password.
				</p>
				<div class="row">
					<input class="t-input grow code" readonly value={collab.status?.invite ?? ""} />
					<button class="t-btn primary" onclick={copyInvite}><Icon name="copy" /> Copy</button>
				</div>
			</div>
			<div class="sect">
				<div class="h">How others reach you</div>
				<div class="kv">
					<span class="t-label">Local network</span>
					<span>{(network?.local ?? []).join(", ") || "none"}</span>
					<span class="t-label">Internet</span>
					<span>{network?.publicIp ? `${network.publicIp}:${collab.status?.session.port}` : "unknown"}</span>
					<span class="t-label">Router (UPnP)</span>
					<span class:bad={network?.upnp !== "ok" && network?.upnp !== "off" && network?.upnp !== "working"}
						>{upnpText}</span
					>
					<span class="t-label">Network list</span>
					<span>{network?.lan ? "shown to editors on your network" : network?.lanError || "hidden"}</span>
				</div>
				{#if network?.carrierNat}
					<p class="t-label hint bad">
						Your router is behind your provider's network (carrier-grade NAT): people from the internet
						cannot connect to you directly. Use a VPN that all of you join (Tailscale, ZeroTier, Radmin VPN)
						and share the invite code again: it includes the VPN address.
					</p>
				{:else if network?.upnp !== "ok" && network?.upnp !== "working"}
					<p class="t-label hint">
						Friends on your network or VPN can join now. For friends on the internet, forward TCP port
						{collab.status?.session.port} on your router to this computer, or all join one VPN (Tailscale, ZeroTier,
						Radmin VPN).
					</p>
				{/if}
			</div>
		{:else if collab.role === "guest"}
			<div class="sect">
				<div class="h">You are in {collab.status?.session.name}</div>
				<p class="t-label hint">
					Hosted by {collab.status?.session.host} at {collab.status?.address}. Your edits show to everyone at
					once; the host saves the map.
				</p>
			</div>
		{:else}
			<div class="t-tabs" role="tablist">
				<button class="t-tab" class:on={tab === "join"} role="tab" onclick={() => (tab = "join")}>Join</button>
				<button class="t-tab" class:on={tab === "host"} role="tab" onclick={() => (tab = "host")}
					>Host a map</button
				>
			</div>
			<label class="row">
				<span class="t-label lbl">Your name</span>
				<input class="t-input grow" bind:value={name} maxlength="32" />
			</label>
			{#if tab === "join"}
				<div class="sect">
					<div class="h">Invite code or address</div>
					<div class="row">
						<input
							class="t-input grow code"
							placeholder="ots:… or 203.0.113.5:47816"
							bind:value={code}
							onkeydown={(event) => event.key === "Enter" && join(code, password)}
						/>
					</div>
					<div class="row">
						<span class="t-label lbl">Password</span>
						<input
							class="t-input grow"
							type="password"
							bind:value={password}
							placeholder="if the host set one"
						/>
						<button
							class="t-btn primary"
							disabled={busy || !code.trim()}
							onclick={() => join(code, password)}>Join</button
						>
					</div>
				</div>
				<div class="sect">
					<div class="row">
						<div class="h grow">On your network</div>
						<span class="spin"
							>{#if browsing}<span class="t-spinner"></span>{/if}</span
						>
					</div>
					{#if found.length}
						<div class="t-panel list">
							{#each found as entry (entry.address)}
								<div class="entry">
									<div class="grow">
										<div class="name">
											{entry.name}{#if entry.password}<span class="t-label">
													· password</span
												>{/if}
										</div>
										<div class="t-label small">
											{entry.host} · {entry.map} · {entry.peers} mapper{entry.peers === 1
												? ""
												: "s"} · {entry.address}
										</div>
									</div>
									<button class="t-btn" onclick={() => joinFound(entry)}>Join</button>
								</div>
							{/each}
						</div>
					{:else}
						<p class="t-label hint">
							No sessions found on your network yet. Sessions of friends on the internet join with their
							invite code.
						</p>
					{/if}
				</div>
				{#if known.length}
					<div class="sect">
						<div class="h">Joined before</div>
						<div class="t-panel list">
							{#each known as entry (entry.key)}
								{@const state = online[entry.key]}
								<div class="entry">
									<span
										class="dot"
										class:on={!!state}
										title={state ? "Open now" : state === null ? "Checking…" : "Not open now"}
									></span>
									<div class="grow">
										<div class="name">{state ? state.name : entry.name}</div>
										<div class="t-label small">
											{entry.host} · {state
												? `${state.map} · ${state.peers} mapping now`
												: state === null
													? "checking…"
													: "not open now"} · joined
											{relativeTime(entry.lastJoined)}
										</div>
									</div>
									<button class="t-btn" disabled={!state} onclick={() => joinKnown(entry)}
										>Join</button
									>
									<button class="t-icon-btn" title="Forget" onclick={() => forget(entry)}
										><Icon name="close" /></button
									>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			{:else}
				<div class="sect">
					{#if hostedMap}
						<p class="t-label hint">
							Share <b>{hostedMap.name}</b>: others get a copy of it and see all edits live. You keep the
							map and save it; the session ends when you close it.
						</p>
					{:else}
						<p class="t-label hint bad">Open or create the map to share first.</p>
					{/if}
					<label class="row">
						<span class="t-label lbl">Session name</span>
						<input class="t-input grow" bind:value={session} placeholder="{name || 'My'}'s map" />
					</label>
					<label class="row">
						<span class="t-label lbl">Password</span>
						<input class="t-input grow" type="password" bind:value={hostPassword} placeholder="optional" />
					</label>
					<label class="row">
						<span class="t-label lbl">Port</span>
						<input class="t-input port" type="number" min="1" max="65535" bind:value={port} />
						<span class="t-label">TCP</span>
					</label>
					<label class="row">
						<input class="t-check" type="checkbox" bind:checked={upnp} />Open the port on my router (UPnP)
					</label>
					<label class="row">
						<input class="t-check" type="checkbox" bind:checked={discoverable} />Show the session to editors
						on my network
					</label>
					<p class="t-label hint">
						Everyone needs the same client and items as you. Your system may ask to let OTS Creator accept
						connections: allow it.
					</p>
				</div>
			{/if}
		{/if}
	</div>
	{#snippet footer()}
		{#if collab.role === "host"}
			<button class="t-btn" onclick={end}>End session</button>
		{:else if collab.role === "guest"}
			<button class="t-btn" onclick={end}>Leave session</button>
		{:else if tab === "host"}
			<button class="t-btn primary" disabled={busy || !hostedMap} onclick={host}>Start hosting</button>
		{/if}
		<button class="t-btn" onclick={close}>Close</button>
	{/snippet}
</Dialog>

{#if askPassword}
	<Dialog title="Password" width={360} onclose={() => (askPassword = null)}>
		<div class="col">
			<p class="t-label hint">{askPassword.title} needs a password.</p>
			<input
				class="t-input"
				type="password"
				bind:value={password}
				onkeydown={(event) => event.key === "Enter" && joinAsked()}
			/>
		</div>
		{#snippet footer()}
			<button class="t-btn primary" onclick={joinAsked}>Join</button>
			<button class="t-btn" onclick={() => (askPassword = null)}>Cancel</button>
		{/snippet}
	</Dialog>
{/if}

<style>
	.form {
		gap: 10px;
	}
	.sect {
		display: flex;
		flex-direction: column;
		gap: 5px;
	}
	.h {
		color: var(--gold);
	}
	.lbl {
		width: 96px;
		flex: none;
	}
	.hint {
		margin: 0;
		font-size: 12px;
	}
	.bad {
		color: #e9b46a;
	}
	.code {
		font-family: var(--font-small, monospace);
		font-size: 12px;
	}
	.port {
		width: 90px;
	}
	.list {
		display: flex;
		flex-direction: column;
		max-height: 180px;
		overflow-y: auto;
		padding: 2px;
	}
	.entry {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 6px;
	}
	.entry + .entry {
		border-top: 1px solid rgba(255, 255, 255, 0.06);
	}
	.name {
		color: var(--text-bright);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.small {
		font-size: 11px;
	}
	/* A spinning box sticks out of its corners: kept inside so the dialog
	   does not scroll sideways. */
	.spin {
		display: grid;
		place-items: center;
		width: 16px;
		height: 16px;
		flex: none;
		overflow: hidden;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #555;
		flex: none;
	}
	.dot.on {
		background: #6c6;
	}
	.kv {
		display: grid;
		grid-template-columns: 110px 1fr;
		gap: 3px 8px;
		font-size: 12px;
		word-break: break-all;
	}
</style>
