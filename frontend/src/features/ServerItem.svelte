<script lang="ts">
	// The items.xml entry of the focused item: what the server knows about it.
	import type { XMLAttribute, XMLItem } from "../lib/api";
	import { untrack } from "svelte";
	import { SvelteSet } from "svelte/reactivity";
	import {
		applyMappedChanges,
		fillFromClient,
		mappedAttributes,
		mappedDifferences,
		type MappedValue,
	} from "../lib/itemsXml";
	import { commands } from "../lib/commands";
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

	// A value longer than this gets a box of several lines instead of an
	// input. Once there it keeps the box while edited, so the box does not
	// turn back into an input under the caret when the value gets shorter.
	const LONG_VALUE = 32;
	const wide = new SvelteSet<object>();
	// Another entry has other attributes: forget the boxes of the last one.
	$effect(() => {
		void entry;
		untrack(() => wide.clear());
	});
	// The input that turned into a box while typed in: the box takes over
	// its focus and caret.
	let handover: { target: object; caret: number } | null = null;

	function isWide(target: { value: string }) {
		return wide.has(target) || target.value.length > LONG_VALUE;
	}

	function widen(event: Event, target: { value: string }) {
		if (target.value.length <= LONG_VALUE || wide.has(target)) return;
		const input = event.currentTarget as HTMLInputElement;
		handover = { target, caret: input.selectionStart ?? target.value.length };
		wide.add(target);
	}

	function takeFocus(target: object) {
		return (box: HTMLTextAreaElement) => {
			if (handover?.target !== target) return;
			box.focus();
			box.setSelectionRange(handover.caret, handover.caret);
			handover = null;
		};
	}

	// The whole text of an input too narrow for it, on hover.
	function showWhole(event: MouseEvent) {
		const input = event.currentTarget as HTMLInputElement;
		input.title = input.scrollWidth > input.clientWidth ? input.value : "";
	}

	// An attribute value of items.xml holds no line breaks: the box only wraps.
	function noLineBreak(event: KeyboardEvent) {
		if (event.key === "Enter") event.preventDefault();
	}

	function add(list: XMLAttribute[]) {
		list.push({ key: "", value: "", children: [] });
	}
</script>

{#snippet valueBox(target: { value: string }, placeholder?: string)}
	{#if isWide(target)}
		<textarea
			class="t-input grow wide"
			{placeholder}
			bind:value={target.value}
			spellcheck="false"
			onfocus={() => wide.add(target)}
			onkeydown={noLineBreak}
			{@attach takeFocus(target)}></textarea>
	{:else}
		<input
			class="t-input grow"
			{placeholder}
			bind:value={target.value}
			spellcheck="false"
			onmouseenter={showWhole}
			oninput={(event) => widen(event, target)}
		/>
	{/if}
{/snippet}

{#snippet rows(list: XMLAttribute[], depth: number)}
	{#each list as attribute, i}
		<div class="attr" style:margin-left="{depth * 12}px">
			<input
				class="t-input key"
				placeholder="key"
				bind:value={attribute.key}
				spellcheck="false"
				onmouseenter={showWhole}
			/>
			{@render valueBox(attribute, "value")}
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
		><span class="t-label">Name</span><input
			class="t-input"
			bind:value={item.name}
			spellcheck="false"
			onmouseenter={showWhole}
		/></label
	>
	<label class="text"
		><span class="t-label">Article</span><input
			class="t-input"
			bind:value={item.article}
			spellcheck="false"
			onmouseenter={showWhole}
		/></label
	>
	<label class="text"
		><span class="t-label">Plural</span><input
			class="t-input"
			bind:value={item.plural}
			spellcheck="false"
			onmouseenter={showWhole}
		/></label
	>
	{#each item.fields ?? [] as field}
		<label class="text"><span class="t-label" title={field.key}>{field.key}</span>{@render valueBox(field)}</label>
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
		<button
			class="t-btn"
			title="Copies the attributes of this entry, to paste them into the entries of other items"
			onclick={commands.copyServerAttributes}>Copy</button
		>
		<button
			class="t-btn"
			disabled={!app.xmlClip}
			title="Replaces the attributes with the copied ones"
			onclick={commands.pasteServerAttributes}>Paste</button
		>
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
	.attr:has(.wide) {
		align-items: flex-start;
	}
	.text:has(.wide) {
		align-items: start;
	}
	.text:has(.wide) .t-label {
		padding-top: 3px;
	}
	.wide {
		height: auto;
		min-height: 34px;
		max-height: 160px;
		padding: 2px 4px;
		resize: vertical;
		field-sizing: content;
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
