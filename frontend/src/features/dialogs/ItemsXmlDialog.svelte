<script lang="ts">
	import { DialogService, ProjectService, type XMLMapping, type XMLReport } from "../../lib/api";
	import { xmlParts } from "../../lib/commands";
	import { XML_FILTERS } from "../../lib/commands/shared";
	import { FLAG_GROUPS } from "../../lib/flags";
	import { plural } from "../../lib/util/format";
	import { app, run, toast } from "../../lib/state/app.svelte";
	import Dialog from "../../lib/ui/Dialog.svelte";
	import Icon from "../../lib/ui/Icon.svelte";
	import Select from "../../lib/ui/Select.svelte";

	const xml = $derived(app.project?.info.itemsXml ?? null);
	let pending = $state<XMLReport | null>(null);
	/** An items.xml found around the client, offered while none is attached. */
	let found = $state("");
	/** The items.otb next to an items.xml read on client ids: the ids are likely server ids. */
	let otbBeside = $state("");

	$effect(() => {
		void app.project?.rev;
		if (!xml) {
			pending = null;
			otbBeside = "";
			ProjectService.FindItemsXML().then(
				(path) => (found = path),
				() => (found = ""),
			);
			return;
		}
		ProjectService.ItemsXMLPreview().then(
			(report) => (pending = report),
			() => (pending = null),
		);
		if (xml.serverIds) otbBeside = "";
		else
			ProjectService.FindItemsOTB().then(
				(path) => (otbBeside = path),
				() => (otbBeside = ""),
			);
	});

	// The properties of a client item the attribute map can write: every
	// flag, and the values the flags carry.
	const properties = FLAG_GROUPS.flatMap((group) =>
		group.flags.flatMap((flag) => [
			{ value: flag.key as string, label: flag.label, flag: true },
			...(flag.fields ?? [])
				.filter((field) => field.kind !== "mask")
				.map((field) => ({ value: field.key, label: `${flag.label}: ${field.label}`, flag: false })),
		]),
	);
	const isFlag = (key: string) => properties.find((property) => property.value === key)?.flag ?? true;

	// The rows are edited here and only loaded again when the stored map
	// changes: other changes of the client keep what is being typed.
	const stored = $derived(JSON.stringify(xml?.mapping ?? []));
	let rows = $state<XMLMapping[]>([]);
	$effect(() => {
		rows = JSON.parse(stored);
	});
	const complete = $derived(rows.filter((row) => row.client && row.key.trim()));
	const mappingDirty = $derived(JSON.stringify(complete) !== stored);

	async function attach(path: string) {
		const attached = await run("Loading items.xml", () => ProjectService.AttachItemsXML(path));
		if (attached?.info.itemsXml)
			toast(`items.xml attached: ${plural(attached.info.itemsXml.items, "item")}.`, "success");
	}

	async function choose() {
		const paths = await DialogService.OpenFiles("itemsXml", "items.xml", XML_FILTERS, false);
		if (paths?.length) await attach(paths[0]);
	}

	async function detach() {
		await run("Detaching items.xml", () => ProjectService.DetachItemsXML());
	}

	async function saveMapping() {
		const saved = await run("Saving the attribute map", () =>
			ProjectService.SetItemsXMLMapping($state.snapshot(complete)),
		);
		if (saved)
			toast(
				complete.length ? `Attribute map saved to ${xml?.mappingPath}.` : "Attribute map removed.",
				"success",
			);
	}
</script>

<Dialog title="Server Items (items.xml)" width={680} onclose={() => (app.dialog = null)}>
	<div class="col">
		<p>
			The items.xml of the server names the items and gives them their attributes. With one attached, each item
			shows its entry in the properties, where it can be edited. This is optional. Compiling then also writes the
			edited entries and adds an entry for every new item; the rest of the file is kept as it is.
		</p>
		{#if xml}
			<div class="info t-panel">
				<div class="path" title={xml.path}>{xml.path}</div>
				<div class="t-label">
					{plural(xml.items, "item")} ·
					{xml.serverIds
						? "server ids, translated with the attached items.otb"
						: "client ids (no items.otb attached)"}
				</div>
			</div>
			{#if otbBeside}
				<div class="warn">
					An items.otb is next to this file, so its ids are probably server ids, but they are read as client
					ids.
					<button class="t-btn" onclick={() => (app.dialog = "itemsOtb")}>Attach the items.otb…</button>
				</div>
			{/if}
			{#if pending}
				{#if pending.error}
					<div class="warn">{pending.error}</div>
				{:else if xmlParts(pending)}
					<div>Next compile:{xmlParts(pending).replace(/[()]/g, "")}.</div>
				{:else}
					<div class="t-label">Compiling changes nothing in the items.xml.</div>
				{/if}
			{/if}
			<div class="map">
				<h4>Attribute map</h4>
				{#if xml.serverIds}
					<div class="t-label">
						On server ids the items.otb tells the server the properties of the client items; the attribute
						map is only used on client ids.
					</div>
				{:else}
					<div class="t-label">
						A server on client ids has no items.otb to read the properties of the client items from. The
						properties listed here are written to items.xml as attributes instead: for every new item, and
						for items whose properties change. A flag writes its value (1 when empty) while it is set; a
						number writes itself. "not" writes the value while the property is not set.
					</div>
					{#each rows as row, i}
						<div class="row maprow">
							<Select
								value={row.client}
								options={properties}
								placeholder="Client property…"
								width={190}
								onchange={(value) => (row.client = value)}
							/>
							<label
								class="row not"
								title="Write the attribute while the property is NOT set (moveable of unmoveable)"
								><input class="t-check" type="checkbox" bind:checked={row.not} />not</label
							>
							<span class="t-label">→</span>
							<input
								class="t-input grow"
								placeholder="attribute key"
								bind:value={row.key}
								spellcheck="false"
							/>
							<input
								class="t-input value"
								placeholder={isFlag(row.client) ? "1" : "its value"}
								title="The value written; empty writes 1 for a flag and the number of a value"
								bind:value={row.value}
								spellcheck="false"
							/>
							<input
								class="t-input aliases"
								placeholder="aliases"
								title="Other keys the server reads as this one, separated by commas (blocking of blocksolid): an entry that has one keeps it"
								value={(row.aliases ?? []).join(", ")}
								onchange={(event) =>
									(row.aliases = event.currentTarget.value
										.split(",")
										.map((alias) => alias.trim())
										.filter(Boolean))}
								spellcheck="false"
							/>
							<button class="t-icon-btn danger" title="Remove" onclick={() => rows.splice(i, 1)}
								><Icon name="trash" /></button
							>
						</div>
					{/each}
					<div class="row">
						<button class="t-btn" onclick={() => rows.push({ client: "", key: "", value: "", not: false })}
							>Add property</button
						>
						<span class="grow"></span>
						<button class="t-btn" disabled={!mappingDirty || !!app.busy} onclick={saveMapping}
							>Save map</button
						>
					</div>
					<div class="t-label path" title={xml.mappingPath}>Stored in {xml.mappingPath}</div>
				{/if}
			</div>
		{:else}
			<div class="t-label">No items.xml is attached.</div>
			{#if found}
				<div class="info t-panel">
					<div class="path" title={found}>Found {found}</div>
					<button class="t-btn" disabled={!!app.busy} onclick={() => attach(found)}>Attach it</button>
				</div>
			{/if}
		{/if}
	</div>
	{#snippet footer()}
		{#if xml}
			<button class="t-btn" disabled={!!app.busy} onclick={detach}>Detach</button>
		{/if}
		<span class="grow"></span>
		<button class="t-btn primary" disabled={!!app.busy} onclick={choose}
			>{xml ? "Change…" : "Choose items.xml…"}</button
		>
		<button class="t-btn" onclick={() => (app.dialog = null)}>Close</button>
	{/snippet}
</Dialog>

<style>
	p {
		margin: 0;
	}
	.info {
		padding: 6px 8px;
		user-select: text;
	}
	.path {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.warn {
		color: var(--gold);
	}
	h4 {
		margin: 4px 0 6px;
		font-size: var(--fs);
		color: var(--text-bright);
		border-bottom: 1px solid #1a1a1a;
		box-shadow: 0 1px 0 #4a4a4a;
		padding-bottom: 3px;
	}
	.map {
		display: flex;
		flex-direction: column;
		gap: 4px;
		max-height: 300px;
		overflow-y: auto;
	}
	.maprow {
		gap: 4px;
	}
	.maprow .grow {
		min-width: 0;
	}
	.value {
		width: 70px;
	}
	.aliases {
		width: 110px;
	}
</style>
