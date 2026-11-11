<script lang="ts">
	// Batch tools. They work on the selection when there is one, otherwise on
	// the whole map; everything can be undone.
	import { ask } from "../../../lib/confirm.svelte";
	import { MapService } from "../../../lib/map/data/api";
	import { mapCommands } from "../../../lib/map/view/commands";
	import { edit, map } from "../../../lib/map/state.svelte";
	import { toast } from "../../../lib/state/app.svelte";
	import Dialog from "../../../lib/ui/Dialog.svelte";
	import NumberField from "../../../lib/ui/NumberField.svelte";

	interface Tool {
		label: string;
		hint: string;
		selection: string | null;
		all: string;
		confirm?: string;
	}
	const TOOLS: Tool[] = [
		{
			label: "Redo borders and walls",
			hint: "Places the borders of every ground and connects walls, carpets and tables again.",
			selection: "borderizeSelection",
			all: "borderizeMap",
		},
		{
			label: "Randomize grounds",
			hint: "Gives grounds new random variations of their brush.",
			selection: "randomizeSelection",
			all: "randomizeMap",
		},
		{
			label: "Remove unknown items",
			hint: "Items the item list does not know (a wrong items.otb or a deleted object).",
			selection: "removeInvalid",
			all: "removeInvalid",
		},
		{
			label: "Remove corpses",
			hint: "Corpses without attributes.",
			selection: "removeCorpses",
			all: "removeCorpses",
		},
		{
			label: "Remove empty spawns",
			hint: "Spawn areas with no creature in them.",
			selection: null,
			all: "removeEmptySpawns",
		},
		{
			label: "Clear tiles of missing houses",
			hint: "House tiles whose house does not exist any more.",
			selection: null,
			all: "clearInvalidHouses",
		},
		{
			label: "Remove tiles nobody can see",
			hint: "Tiles farther than the client view from any walkable tile.",
			selection: null,
			all: "removeUnreachable",
			confirm: "This removes whole tiles from the map. Continue?",
		},
		{
			label: "Forget changes",
			hint: "Marks every tile unchanged, for View › Only changed tiles.",
			selection: null,
			all: "clearModified",
		},
	];

	const hasSelection = $derived((map.active?.selection ?? 0) > 0);
	let onSelection = $state((map.active?.selection ?? 0) > 0);

	async function run(tool: Tool) {
		const useSelection = onSelection && hasSelection && tool.selection !== null;
		if (tool.confirm && !(await ask({ title: tool.label, message: tool.confirm, ok: "Continue" }))) return;
		const name = useSelection ? tool.selection! : tool.all;
		await mapCommands.tool(
			name,
			useSelection && (name === "removeInvalid" || name === "removeCorpses"),
			tool.label,
		);
	}

	let spawnSeconds = $state(map.settings?.defaultSpawnTime ?? 60);
	let spawnCreature = $state("");

	async function setSpawnTimes() {
		const inSelection = onSelection && hasSelection;
		const change = await edit((id) =>
			MapService.SetSpawnTimes(id, spawnSeconds, spawnCreature.trim(), inSelection),
		);
		if (change) toast(`${change.count.toLocaleString()} ${change.count === 1 ? "creature" : "creatures"} changed.`);
	}
</script>

<Dialog title="Map tools" width={480} onclose={() => (map.dialog = null)}>
	<div class="col">
		<div class="row scope">
			<span class="t-label">Work on</span>
			<div class="m-seg">
				<button
					class:on={onSelection && hasSelection}
					disabled={!hasSelection}
					title={hasSelection ? "" : "Select an area first"}
					onclick={() => (onSelection = true)}
					>The selection{hasSelection ? ` (${map.active?.selection.toLocaleString()} tiles)` : ""}</button
				>
				<button class:on={!onSelection || !hasSelection} onclick={() => (onSelection = false)}
					>The whole map</button
				>
			</div>
		</div>
		{#each TOOLS as tool}
			{@const off = onSelection && hasSelection && tool.selection === null}
			<div class="tool" class:off>
				<span class="col grow"
					><span>{tool.label}</span><span class="t-label hint"
						>{off ? "Only for the whole map." : tool.hint}</span
					></span
				>
				<button class="t-btn" disabled={off} onclick={() => run(tool)}>Run</button>
			</div>
		{/each}
		<div class="tool">
			<span class="col grow"
				><span>Set spawn time</span><span class="t-label hint"
					>Respawn time of every creature, or only of the named one.</span
				></span
			>
			<NumberField
				value={spawnSeconds}
				min={1}
				max={86400}
				width={56}
				title="Seconds"
				onchange={(value) => (spawnSeconds = value)}
			/>
			<span class="t-label">s</span>
			<input class="t-input name" placeholder="All creatures" bind:value={spawnCreature} />
			<button class="t-btn" onclick={setSpawnTimes}>Run</button>
		</div>
		<p class="t-label hint">Every tool is one step in the undo history.</p>
	</div>
	{#snippet footer()}
		<button class="t-btn" onclick={() => (map.dialog = null)}>Close</button>
	{/snippet}
</Dialog>

<style>
	.scope {
		gap: 8px;
		margin-bottom: 4px;
	}
	.scope .m-seg {
		flex: 1;
	}
	.tool {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 4px 0;
		border-top: 1px solid rgba(255, 255, 255, 0.05);
	}
	.tool .name {
		width: 110px;
	}
	.tool.off {
		opacity: 0.55;
	}
	.col {
		gap: 0;
	}
	.hint {
		font-size: 12px;
		margin: 0;
	}
</style>
