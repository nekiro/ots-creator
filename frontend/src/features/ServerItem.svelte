<script lang="ts">
	// The items.xml entry of the focused item: what the server knows about it.
	import type { XMLAttribute, XMLItem } from "../lib/api";
	import { untrack } from "svelte";
	import {
		applyMappedChanges,
		fillFromClient,
		mappedAttributes,
		mappedDifferences,
		type MappedValue,
	} from "../lib/itemsXml";
	import { app } from "../lib/state/app.svelte";
	import Icon from "../lib/ui/Icon.svelte";

	let { entry }: { entry: XMLItem } = $props();

	const item = $derived(entry.item);
	const serverIds = $derived(!!app.project?.info.itemsXml?.serverIds);
	// The attribute map is used on client ids only: an items.otb tells the
	// server the properties otherwise.
	const mapping = $derived(serverIds ? [] : (app.project?.info.itemsXml?.mapping ?? []));

	// A mapped property changed in the properties below changes its
	// attribute here at once; Apply stores both.
	let seen: { draft: object; mapped: Map<string, MappedValue> } | null = null;
	$effect(() => {
		const draft = app.draft;
		if (!draft || !mapping.length) {
			seen = null;
			return;
		}
		const mapped = mappedAttributes(mapping, draft.props);
		untrack(() => {
			// Another object in the draft (selected, reloaded, reverted) is no edit.
			if (seen?.draft === draft && app.xml) applyMappedChanges(app.xml.item, seen.mapped, mapped);
			seen = { draft, mapped };
		});
	});

	// Where the entry is not what the properties say: shown, never applied
	// by itself (a server can differ from the client on purpose).
	const differences = $derived(
		app.draft && mapping.length && (entry.exists || item.attributes?.length)
			? mappedDifferences(item, mapping, app.draft.props)
			: [],
	);

	function fill() {
		if (app.draft) fillFromClient(entry.item, mapping, app.draft.props, app.draft.name);
	}

	function add(list: XMLAttribute[]) {
		list.push({ key: "", value: "", children: [] });
	}
</script>

{#snippet rows(list: XMLAttribute[], depth: number)}
	{#each list as attribute, i}
		<div class="attr" style:margin-left="{depth * 12}px">
			<input class="t-input key" placeholder="key" bind:value={attribute.key} spellcheck="false" />
			<input class="t-input grow" placeholder="value" bind:value={attribute.value} spellcheck="false" />
			<button
				class="t-icon-btn"
				title="Add an attribute inside this one"
				onclick={() => add((attribute.children ??= []))}><Icon name="plus" /></button
			>
			<button class="t-icon-btn danger" title="Remove this attribute" onclick={() => list.splice(i, 1)}
				><Icon name="trash" /></button
			>
		</div>
		{#if attribute.children?.length}{@render rows(attribute.children, depth + 1)}{/if}
	{/each}
{/snippet}

<section>
	<h4
		title={serverIds
			? "The entry of the item in items.xml, by the server id the items.otb gives it"
			: "The entry of the item in items.xml, by its client id"}
	>
		Server (items.xml){#if entry.id}&nbsp;· {serverIds ? "server id" : "id"} {entry.id}{/if}
	</h4>
	{#if !entry.id}
		<div class="note">The items.otb gives this item no server id: its entry cannot be written.</div>
	{:else if !entry.exists}
		<div class="t-label small">No entry yet: compiling adds one.</div>
	{/if}
	<label class="text"
		><span class="t-label">Name</span><input class="t-input" bind:value={item.name} spellcheck="false" /></label
	>
	<label class="text"
		><span class="t-label">Article</span><input
			class="t-input"
			bind:value={item.article}
			spellcheck="false"
		/></label
	>
	<label class="text"
		><span class="t-label">Plural</span><input class="t-input" bind:value={item.plural} spellcheck="false" /></label
	>
	{#each item.fields ?? [] as field}
		<label class="text"
			><span class="t-label" title={field.key}>{field.key}</span><input
				class="t-input"
				bind:value={field.value}
				spellcheck="false"
			/></label
		>
	{/each}
	{@render rows(item.attributes ?? [], 0)}
	{#if differences.length}
		<div class="note" title="Fill from client makes the entry what the client says">
			{differences.length === 1 ? "1 attribute differs" : `${differences.length} attributes differ`} from the client:
			{#each differences as difference}<div class="diff">{difference}</div>{/each}
		</div>
	{/if}
	<div class="row buttons">
		<button class="t-btn" onclick={() => add((item.attributes ??= []))}>Add attribute</button>
		{#if mapping.length}
			<button
				class="t-btn"
				title="Sets the attributes of the attribute map to what the properties of the item say"
				onclick={fill}>{entry.exists ? "Fill from client" : "Create from client"}</button
			>
		{/if}
	</div>
</section>

<style>
	section {
		margin-bottom: 10px;
	}
	h4 {
		margin: 4px 0 6px;
		font-size: var(--fs);
		color: var(--text-bright);
		border-bottom: 1px solid #1a1a1a;
		box-shadow: 0 1px 0 #4a4a4a;
		padding-bottom: 3px;
	}
	.text {
		display: grid;
		grid-template-columns: 62px 1fr;
		align-items: center;
		gap: 6px;
		margin-bottom: 4px;
	}
	.text .t-label {
		text-align: right;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.attr {
		display: flex;
		align-items: center;
		gap: 3px;
		margin-bottom: 3px;
	}
	.key {
		width: 38%;
		min-width: 0;
	}
	.attr .grow {
		min-width: 0;
	}
	.small {
		font: var(--fs-small) / 10px var(--font-small);
		margin-bottom: 4px;
	}
	.note {
		color: var(--gold);
		font: var(--fs-small) / 10px var(--font-small);
		margin: 4px 0;
	}
	.diff {
		padding-left: 8px;
		user-select: text;
	}
	.buttons {
		gap: 4px;
		margin-top: 2px;
		flex-wrap: wrap;
	}
</style>
