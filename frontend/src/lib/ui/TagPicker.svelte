<script lang="ts">
  // Multi select with search: picked tags are small removable chips in the
  // field, typing filters a popup list (grouped, like the Select popup).
  // Enter or a click adds the highlighted tag (the best match first); with
  // allowCustom, a typed tag that is not in the list is offered last. Backspace on an empty
  // field removes the last tag.
  import { tick } from "svelte";
  import { matchTags, parseTags, type TagOption } from "../market";
  import { toCss } from "../pixelscale";

  let {
    value,
    options,
    onchange,
    max = 0,
    allowCustom = false,
    placeholder = "Search tags…",
  }: {
    value: string[];
    options: TagOption[];
    onchange: (tags: string[]) => void;
    /** Maximum number of tags (0 = no limit). */
    max?: number;
    allowCustom?: boolean;
    placeholder?: string;
  } = $props();

  const ITEM_H = 20;
  const MAX_ITEMS = 10;

  let query = $state("");
  let open = $state(false);
  let active = $state(0);
  let field: HTMLDivElement;
  let input: HTMLInputElement;
  let list = $state<HTMLDivElement>();
  let rect = $state({ left: 0, top: 0, width: 0 });

  const full = $derived(max > 0 && value.length >= max);
  const matches = $derived(matchTags(options, query, value));
  // A typed tag that is not offered can be added as is.
  const custom = $derived.by(() => {
    if (!allowCustom) return "";
    const t = parseTags(query)[0] ?? "";
    return t && !value.includes(t) && !matches.some((m) => m.tag === t) ? t : "";
  });
  type Row = { tag: string; count?: number; header?: string; custom?: boolean };
  const rows = $derived.by((): Row[] => {
    const out: Row[] = [];
    let group: string | undefined;
    for (const m of matches) {
      out.push({ tag: m.tag, count: m.count, header: m.group && m.group !== group ? m.group : undefined });
      group = m.group;
    }
    // Last, so Enter takes the best match; alone when nothing matches.
    if (custom) out.push({ tag: custom, custom: true, header: matches.length ? "Your tag" : undefined });
    return out;
  });

  function place() {
    const b = field.getBoundingClientRect();
    const r = { left: toCss(b.left), top: toCss(b.top), bottom: toCss(b.bottom), width: toCss(b.width) };
    const h = MAX_ITEMS * ITEM_H + 4;
    const up = r.bottom + h > toCss(window.innerHeight) - 4 && r.top - h > 4;
    rect = { left: r.left, top: up ? r.top - h : r.bottom, width: r.width };
  }

  async function show() {
    if (full) return;
    place();
    open = true;
    active = 0;
    await tick();
    list?.scrollTo({ top: 0 });
  }

  function add(tag: string) {
    if (!tag || value.includes(tag) || full) return;
    onchange([...value, tag]);
    query = "";
    active = 0;
    if (max > 0 && value.length + 1 >= max) open = false;
    else tick().then(place); // the field may wrap to a new line
  }

  function remove(tag: string) {
    onchange(value.filter((t) => t !== tag));
    input?.focus();
  }

  function move(delta: number) {
    if (!rows.length) return;
    active = Math.min(Math.max(active + delta, 0), rows.length - 1);
    list?.querySelector<HTMLElement>(`[data-i="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) show();
      else move(e.key === "ArrowDown" ? 1 : -1);
    } else if (e.key === "Enter" || (e.key === "," && allowCustom)) {
      e.preventDefault();
      if (open && rows[active]) add(rows[active].tag);
      else if (custom) add(custom);
    } else if (e.key === "Escape" && open) {
      e.preventDefault();
      e.stopPropagation(); // the dialog stays open
      open = false;
    } else if (e.key === "Backspace" && !query && value.length) {
      remove(value[value.length - 1]);
    } else if (e.key === "Tab") {
      open = false;
    }
  }
</script>

<svelte:window
  onmousedown={(e) => open && !list?.contains(e.target as Node) && !field.contains(e.target as Node) && (open = false)}
  onresize={() => (open = false)}
  onblur={() => (open = false)}
/>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="field t-input" class:on={open} bind:this={field} onclick={() => input.focus()}>
  {#each value as t (t)}
    <span class="chip">{t}<button type="button" class="x" title="Remove {t}" onclick={(e) => (e.stopPropagation(), remove(t))}>×</button></span>
  {/each}
  <input
    bind:this={input}
    bind:value={query}
    placeholder={full ? `${max} tags max` : value.length ? "" : placeholder}
    disabled={full}
    onfocus={show}
    oninput={() => (open ? ((active = 0), place()) : show())}
    {onkeydown}
  />
</div>

{#if open && rows.length}
  <div bind:this={list} class="popup t-popup" role="listbox" style="left:{rect.left}px;top:{rect.top}px;width:{rect.width}px;max-height:{MAX_ITEMS * ITEM_H + 4}px">
    {#each rows as r, i (r.custom ? "+" + r.tag : r.tag)}
      {#if r.header}<div class="header">{r.header}</div>{/if}
      <div
        class="item"
        class:active={i === active}
        role="option"
        aria-selected={i === active}
        tabindex="-1"
        data-i={i}
        onmouseenter={() => (active = i)}
        onmousedown={(e) => e.preventDefault()}
        onclick={() => add(r.tag)}
        onkeydown={() => {}}
      >
        <span>{#if r.custom}<span class="dim">Add&nbsp;</span>{/if}{r.tag}</span>
        {#if r.count !== undefined}<span class="count">{r.count}</span>{/if}
      </div>
    {/each}
  </div>
{/if}

<style>
  .field {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 3px;
    min-height: 20px;
    height: auto;
    padding: 2px 4px;
    cursor: text;
  }
  .field input {
    flex: 1;
    min-width: 80px;
    height: 16px;
    padding: 0;
    border: 0;
    background: none;
    outline: none;
    font: inherit;
    color: var(--text-bright);
  }
  .field input::placeholder {
    color: var(--text-dim);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    height: 16px;
    padding: 0 1px 0 5px;
    border: 1px solid var(--gold);
    background: rgba(232, 196, 106, 0.1);
    color: var(--gold);
    font: var(--fs-small) / 14px var(--font-small);
  }
  .x {
    width: 12px;
    height: 14px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--gold);
    font: inherit;
    cursor: pointer;
    opacity: 0.7;
  }
  .x:hover {
    opacity: 1;
    color: #fff;
  }
  .popup {
    position: fixed;
    z-index: 200;
    overflow-y: auto;
    padding: 1px 0;
  }
  .header {
    height: 16px;
    padding: 2px 5px 0;
    font: var(--fs-small) / 14px var(--font-small);
    color: var(--gold);
    opacity: 0.8;
  }
  .item {
    display: flex;
    justify-content: space-between;
    height: 20px;
    padding: 0 8px 0 12px;
    font: var(--fs) / 20px var(--font);
    color: #c0c0c0;
    white-space: nowrap;
    cursor: pointer;
  }
  .item.active {
    color: #f4f4f4;
    background-color: #ffffff1b;
  }
  .dim,
  .count {
    color: var(--text-dim);
  }
</style>
