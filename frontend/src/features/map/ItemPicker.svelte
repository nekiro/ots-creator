<script lang="ts">
  // Picks an item by name or id with a searchable dropdown of icons.
  import { toCss } from "../../lib/pixelscale";
  import { map } from "../../lib/map/state.svelte";
  import { portal } from "../../lib/ui/portal";
  import ItemIcon from "./ItemIcon.svelte";

  let {
    value = $bindable(0),
    placeholder = "Item name or id…",
    width = 0,
    onchange,
  }: { value?: number; placeholder?: string; width?: number; onchange?: (id: number) => void } = $props();

  let text = $state("");
  let open = $state(false);
  let active = $state(0);
  let input: HTMLInputElement;
  let rect = $state({ left: 0, top: 0, width: 0 });

  $effect.pre(() => {
    if (!open) text = value ? String(value) : "";
  });

  const results = $derived(open && text.trim() && map.items ? map.items.search(text, 60) : []);
  const name = $derived(value && map.items ? map.items.name(value) : "");

  function show() {
    const bounds = input.getBoundingClientRect();
    rect = { left: toCss(bounds.left), top: toCss(bounds.bottom), width: Math.max(toCss(bounds.width), 240) };
    open = true;
    active = 0;
  }

  function choose(id: number) {
    value = id;
    text = String(id);
    open = false;
    onchange?.(id);
  }

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      if (!open) show();
      active = Math.min(active + 1, results.length - 1);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      active = Math.max(active - 1, 0);
    } else if (event.key === "Enter") {
      event.preventDefault();
      if (results[active] !== undefined) choose(results[active]);
      else if (map.items?.has(Number(text))) choose(Number(text));
    } else if (event.key === "Escape" && open) {
      event.stopPropagation();
      open = false;
    }
  }
</script>

<svelte:window onmousedown={(event) => open && !(event.target as HTMLElement).closest(".ipick-pop") && event.target !== input && (open = false)} />

<span class="ipick" style={width ? `width:${width}px` : ""}>
  <span class="t-slot sm"><ItemIcon id={value} size={28} /></span>
  <span class="col grow">
    <input
      bind:this={input}
      class="t-input"
      {placeholder}
      bind:value={text}
      onfocus={show}
      oninput={() => {
        if (!open) show();
        active = 0;
      }}
      {onkeydown}
    />
    {#if name}<span class="nm" title={name}>{name}</span>{/if}
  </span>
</span>

{#if open && results.length}
  <div class="ipick-pop t-popup" use:portal style="left:{rect.left}px;top:{rect.top}px;width:{rect.width}px">
    {#each results as id, i (id)}
      <button class="opt" class:active={i === active} onmouseenter={() => (active = i)} onmousedown={(event) => (event.preventDefault(), choose(id))}>
        <ItemIcon {id} size={24} />
        <span class="on">{map.items?.name(id)}</span>
        <span class="oid">{id}</span>
      </button>
    {/each}
  </div>
{/if}

<style>
  .ipick {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }
  .sm {
    width: 30px;
    height: 30px;
    background-size: 30px 30px;
  }
  .nm {
    font: var(--fs-small) / 11px var(--font-small);
    color: var(--text-dim);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ipick-pop {
    position: fixed;
    z-index: 300;
    max-height: 300px;
    overflow-y: auto;
    padding: 1px 0;
  }
  .opt {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 6px;
    background: none;
    border: 0;
    color: #c0c0c0;
    font: inherit;
    text-align: left;
    padding: 1px 6px;
    cursor: pointer;
  }
  .opt.active {
    background: #ffffff1b;
    color: #f4f4f4;
  }
  .on {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .oid {
    color: var(--text-dim);
  }
</style>
