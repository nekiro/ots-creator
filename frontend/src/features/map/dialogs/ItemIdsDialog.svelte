<script lang="ts">
	// Asks how a map to open or create stores its item ids, with the proposal
	// of the backend and why it was made (see chooseItemIds).
	import { DialogService } from "../../../lib/api";
	import { OTB_FILTERS } from "../../../lib/commands/shared";
	import { ItemIdsMode } from "../../../lib/map/data/api";
	import { answerItemIds, itemIdsLabel, itemIdsState } from "../../../lib/map/itemIds.svelte";
	import Dialog from "../../../lib/ui/Dialog.svelte";
	import Select from "../../../lib/ui/Select.svelte";

	const request = $derived(itemIdsState.request);
	let mode = $state<ItemIdsMode>(ItemIdsMode.ModeOTB);
	let otbPath = $state("");

	// Every request starts from its proposal.
	$effect.pre(() => {
		if (!request) return;
		mode = request.plan.mode;
		otbPath = request.plan.clientOtb || request.plan.foundOtb;
	});

	const server = $derived(mode === ItemIdsMode.ModeOTB);
	const attaches = $derived(server && !!otbPath && otbPath !== request?.plan.clientOtb);
	const fileDiffers = $derived(!!request?.plan.fileMode && request.plan.fileMode !== mode);

	async function browse() {
		const paths = await DialogService.OpenFiles("itemsOtb", "items.otb", OTB_FILTERS, false);
		if (paths?.length) {
			otbPath = paths[0];
			// Picking an items.otb means server ids.
			mode = ItemIdsMode.ModeOTB;
		}
	}

	function confirm() {
		if (server && !otbPath) return;
		answerItemIds({ mode, otbPath: attaches ? otbPath : "" });
	}
</script>

{#if request}
	<Dialog title={request.title} width={520} onclose={() => answerItemIds(null)}>
		<div class="col form">
			{#if request.name}<div class="name" title={request.name}>{request.name}</div>{/if}
			<div class="row">
				<span class="t-label lbl">Item ids</span>
				<Select
					grow
					value={mode}
					options={[
						{ value: ItemIdsMode.ModeOTB, label: "Server ids: translated with items.otb" },
						{ value: ItemIdsMode.ModeClient, label: "Client ids: no items.otb" },
					]}
					onchange={(value) => (mode = value)}
				/>
			</div>
			<p class="t-label hint">Proposed: {itemIdsLabel(request.plan.mode)}, because {request.plan.reason}.</p>
			{#if fileDiffers}
				<p class="warn">
					The map file says it uses {itemIdsLabel(request.plan.fileMode)}: its items will show wrong.
				</p>
			{/if}
			{#if server}
				<div class="row">
					<span class="t-label lbl">items.otb</span>
					<input
						class="t-input grow"
						readonly
						value={otbPath}
						placeholder="Choose the items.otb of the server"
					/>
					<button class="t-btn" onclick={browse}>Browse…</button>
				</div>
				{#if request.plan.foundOtb && otbPath !== request.plan.foundOtb}
					<div class="row">
						<span class="t-label grow found" title={request.plan.foundOtb}
							>Found: {request.plan.foundOtb}</span
						>
						<button class="t-btn" onclick={() => (otbPath = request.plan.foundOtb)}>Use it</button>
					</div>
				{/if}
				<p class="t-label hint">
					{#if !otbPath}
						Server ids need the items.otb of the server.
					{:else if attaches}
						It will be attached to the client{request.plan.clientOtb
							? `, replacing ${request.plan.clientOtb}`
							: ""}: every map on server ids reads its ids with it, and compiling the client updates it.
					{:else}
						The items.otb of the client: compiling the client updates it too.
					{/if}
				</p>
			{:else}
				<p class="t-label hint">The item ids of the map are the ids of the client objects, used as they are.</p>
			{/if}
		</div>
		{#snippet footer()}
			<span class="grow"></span>
			<button class="t-btn primary" disabled={server && !otbPath} onclick={confirm}
				>{request.name ? "Open" : "Create"}</button
			>
			<button class="t-btn" onclick={() => answerItemIds(null)}>Cancel</button>
		{/snippet}
	</Dialog>
{/if}

<style>
	.form {
		gap: 6px;
	}
	.name {
		color: var(--gold);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.lbl {
		width: 70px;
		flex: none;
	}
	.hint {
		margin: 0;
		font-size: 12px;
	}
	.warn {
		margin: 0;
		font-size: 12px;
		color: #e9b46a;
	}
	.found {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12px;
	}
</style>
