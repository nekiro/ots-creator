<script lang="ts">
	import { Category, res } from "../../lib/api";
	import { map } from "../../lib/map/state.svelte";
	import { versions } from "../../lib/state/app.svelte";
	import { loading } from "../../lib/ui/loading";

	let {
		id = 0,
		outfit = 0,
		size = 32,
		crisp = false,
	}: {
		id?: number;
		outfit?: number;
		size?: number;
		/** Grow the image only by whole multiples, so its pixels stay sharp; bigger images still shrink to fit. */
		crisp?: boolean;
	} = $props();

	const client = $derived(id && map.items ? map.items.client[id] : 0);
	const src = $derived(
		outfit
			? res.thumb(Category.CategoryOutfit, outfit, versions.thing(Category.CategoryOutfit, outfit))
			: client
				? res.thumb(Category.CategoryItem, client, versions.thing(Category.CategoryItem, client))
				: "",
	);

	let natural = $state<{ width: number; height: number } | null>(null);
	$effect.pre(() => {
		void src;
		natural = null;
	});
	const shown = $derived.by(() => {
		if (!crisp || !natural) return null;
		const fit = Math.min(size / natural.width, size / natural.height);
		const scale = fit >= 1 ? Math.floor(fit) : fit;
		return { width: Math.round(natural.width * scale), height: Math.round(natural.height * scale) };
	});

	/** Reads the image size once it is there (cached images may be loaded already). */
	function measure(img: HTMLImageElement) {
		if (!crisp) return;
		const read = () => {
			if (img.naturalWidth) natural = { width: img.naturalWidth, height: img.naturalHeight };
		};
		if (img.complete) read();
		img.addEventListener("load", read);
		return { destroy: () => img.removeEventListener("load", read) };
	}
</script>

<span class="icon t-loader" style="width:{size}px;height:{size}px">
	{#if src}
		<img
			class="pixel"
			{src}
			alt=""
			loading="lazy"
			decoding="async"
			draggable="false"
			use:loading={src}
			use:measure
			style={shown
				? `width:${shown.width}px;height:${shown.height}px`
				: `max-width:${size}px;max-height:${size}px${crisp ? ";opacity:0" : ""}`}
		/>
	{:else if id}
		<span class="missing" title="Unknown item">?</span>
	{/if}
</span>

<style>
	.icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: none;
		position: relative;
	}
	img {
		object-fit: contain;
	}
	.missing {
		color: #d66;
		font-weight: bold;
	}
</style>
