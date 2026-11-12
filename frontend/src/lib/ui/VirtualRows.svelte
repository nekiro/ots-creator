<script lang="ts" generics="T">
	import { onMount, type Snippet } from "svelte";

	// A long list of rows of one height with only the rows in view (and a few
	// around) in the page. It scrolls with the closest scrolling element
	// around it, which may hold other things above and below the rows.
	let {
		items,
		row,
		around = 10,
	}: {
		items: T[];
		row: Snippet<[T, number]>;
		around?: number;
	} = $props();

	let root: HTMLDivElement;
	let scroller: HTMLElement | null = null;
	/** How far the top of the rows is above the top of the scrolling element, and its height. */
	let above = $state(0);
	let viewHeight = $state(0);
	let rowHeight = $state(22);

	const first = $derived(Math.max(0, Math.min(items.length, Math.floor(above / rowHeight) - around)));
	const last = $derived(Math.min(items.length, Math.ceil((above + viewHeight) / rowHeight) + around));

	function measure() {
		if (!scroller) return;
		above = scroller.getBoundingClientRect().top - root.getBoundingClientRect().top;
		viewHeight = scroller.clientHeight;
	}

	onMount(() => {
		for (let parent = root.parentElement; parent; parent = parent.parentElement) {
			const overflow = getComputedStyle(parent).overflowY;
			if (overflow === "auto" || overflow === "scroll") {
				scroller = parent;
				break;
			}
		}
		if (!scroller) return;
		const observer = new ResizeObserver(measure);
		observer.observe(scroller);
		observer.observe(root);
		scroller.addEventListener("scroll", measure, { passive: true });
		measure();
		return () => {
			observer.disconnect();
			scroller?.removeEventListener("scroll", measure);
		};
	});

	$effect(() => {
		// The rows are between the two spacers: all are as high as the first ones.
		if (last - first < 1) return;
		const shown = root.children;
		const one = shown[1] as HTMLElement;
		const next = last - first > 1 ? (shown[2] as HTMLElement) : null;
		const height = next ? next.offsetTop - one.offsetTop : one.offsetHeight;
		if (height > 0) rowHeight = height;
	});

	/** Scrolls as little as it takes to have the row at index in view. */
	export function reveal(index: number): void {
		if (!scroller) return;
		const rowsTop = root.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop;
		const top = rowsTop + index * rowHeight;
		if (top < scroller.scrollTop) scroller.scrollTop = top;
		else if (top + rowHeight > scroller.scrollTop + scroller.clientHeight)
			scroller.scrollTop = top + rowHeight - scroller.clientHeight;
	}
</script>

<div bind:this={root}>
	<div style:height="{first * rowHeight}px"></div>
	{#each items.slice(first, last) as item, offset (first + offset)}{@render row(item, first + offset)}{/each}
	<div style:height="{(items.length - last) * rowHeight}px"></div>
</div>
