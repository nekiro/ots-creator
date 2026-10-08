<script lang="ts" module>
  export interface Option<T> {
    value: T;
    label: string;
    disabled?: boolean;
  }
</script>

<script lang="ts" generics="V">
  // OTClient QtComboBox (combobox_new, centered text) with a dark popup list
  // like the Tibia client. The popup is position:fixed so scrolling
  // containers and dialogs never clip it.
  import { tick } from "svelte";
  import { toCss } from "../pixelscale";

  let {
    value,
    options,
    onchange,
    disabled = false,
    placeholder = "Choose…",
    width = 0,
    grow = false,
    maxItems = 12,
    title = "",
  }: {
    value: V;
    options: Option<V>[];
    onchange: (v: V) => void;
    disabled?: boolean;
    placeholder?: string;
    width?: number;
    grow?: boolean;
    maxItems?: number;
    title?: string;
  } = $props();

  const ITEM_H = 20;
  let open = $state(false);
  let active = $state(-1);
  let btn: HTMLButtonElement;
  let list = $state<HTMLDivElement>();
  let rect = $state({ left: 0, top: 0, width: 0, up: false });

  const selected = $derived(options.findIndex((o) => o.value === value));
  const label = $derived(selected >= 0 ? options[selected].label : placeholder);

  async function show() {
    if (disabled || open) return;
    const b = btn.getBoundingClientRect();
    const r = { left: toCss(b.left), top: toCss(b.top), bottom: toCss(b.bottom), width: toCss(b.width) };
    const h = Math.min(options.length, maxItems) * ITEM_H + 4;
    const up = r.bottom + h > toCss(window.innerHeight) - 4 && r.top - h > 4;
    rect = { left: r.left, top: up ? r.top - h : r.bottom, width: r.width, up };
    active = Math.max(selected, 0);
    open = true;
    await tick();
    list?.querySelector<HTMLElement>(`[data-i="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }

  function hide() {
    open = false;
  }

  function pick(i: number) {
    const o = options[i];
    if (!o || o.disabled) return;
    hide();
    btn.focus();
    if (o.value !== value) onchange(o.value);
  }

  function move(delta: number) {
    if (!options.length) return;
    let i = active;
    for (let k = 0; k < options.length; k++) {
      i = Math.min(Math.max(i + delta, 0), options.length - 1);
      if (!options[i].disabled) break;
    }
    active = i;
    list?.querySelector<HTMLElement>(`[data-i="${i}"]`)?.scrollIntoView({ block: "nearest" });
  }

  function onkeydown(e: KeyboardEvent) {
    if (disabled) return;
    const keys: Record<string, () => void> = {
      ArrowDown: () => (open ? move(1) : show()),
      ArrowUp: () => (open ? move(-1) : show()),
      PageDown: () => open && move(maxItems),
      PageUp: () => open && move(-maxItems),
      Home: () => open && move(-options.length),
      End: () => open && move(options.length),
      Enter: () => (open ? pick(active) : show()),
      " ": () => (open ? pick(active) : show()),
      Escape: () => open && hide(),
      Tab: () => hide(),
    };
    const fn = keys[e.key];
    if (fn) {
      if (e.key !== "Tab") e.preventDefault();
      if (e.key === "Escape" && open) e.stopPropagation();
      fn();
      return;
    }
    // Type to jump: first option starting with the typed character.
    if (e.key.length === 1 && !e.ctrlKey && !e.metaKey) {
      const ch = e.key.toLowerCase();
      const start = (open ? active : selected) + 1;
      for (let k = 0; k < options.length; k++) {
        const i = (start + k) % options.length;
        if (options[i].label.toLowerCase().startsWith(ch) && !options[i].disabled) {
          if (open) {
            active = i;
            list?.querySelector<HTMLElement>(`[data-i="${i}"]`)?.scrollIntoView({ block: "nearest" });
          } else if (options[i].value !== value) onchange(options[i].value);
          break;
        }
      }
    }
  }
</script>

<svelte:window
  onmousedown={(e) => open && !list?.contains(e.target as Node) && !btn.contains(e.target as Node) && hide()}
  onresize={hide}
  onblur={hide}
/>

<button
  bind:this={btn}
  type="button"
  class="combo"
  class:on={open}
  class:grow
  style={width ? `width:${width}px` : ""}
  {disabled}
  {title}
  aria-haspopup="listbox"
  aria-expanded={open}
  onmousedown={(e) => {
    if (e.button !== 0) return;
    e.preventDefault();
    btn.focus();
    open ? hide() : show();
  }}
  {onkeydown}
>
  <span class="text" class:placeholder={selected < 0}>{label}</span>
</button>

{#if open}
  <div
    bind:this={list}
    class="popup t-popup"
    role="listbox"
    style="left:{rect.left}px;top:{rect.top}px;min-width:{rect.width}px;max-height:{maxItems * ITEM_H + 4}px"
  >
    {#each options as o, i}
      <div
        class="item"
        class:active={i === active}
        class:sel={i === selected}
        class:disabled={o.disabled}
        role="option"
        aria-selected={i === selected}
        tabindex="-1"
        data-i={i}
        onmouseenter={() => !o.disabled && (active = i)}
        onmousedown={(e) => e.preventDefault()}
        onclick={() => pick(i)}
        onkeydown={() => {}}
      >
        {o.label}
      </div>
    {/each}
  </div>
{/if}

<style>
  /* QtComboBox: image-border 2, image-border-right 22, text centered (offset -11) */
  .combo {
    font: var(--fs) var(--font);
    color: #c0c0c0;
    height: 20px;
    min-width: 91px;
    padding: 0;
    display: inline-flex;
    align-items: center;
    background: transparent;
    border-style: solid;
    border-width: 2px 22px 2px 2px;
    border-image: url("../../assets/ui/qcombo-idle.png") 2 22 2 2 fill / 2px 22px 2px 2px repeat;
    image-rendering: pixelated;
    cursor: pointer;
    outline: none;
  }
  .combo.grow {
    flex: 1;
    min-width: 0;
  }
  .combo.on,
  .combo:active:not(:disabled) {
    border-image-source: url("../../assets/ui/qcombo-on.png");
  }
  .combo.on .text,
  .combo:active:not(:disabled) .text {
    transform: translate(1px, 1px);
  }
  .combo:hover:not(:disabled),
  .combo:focus-visible {
    color: #f4f4f4;
  }
  .combo:disabled {
    border-image-source: url("../../assets/ui/qcombo-disabled.png");
    color: #888888;
    cursor: default;
  }
  .text {
    flex: 1;
    min-width: 0;
    padding: 0 2px;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .placeholder {
    color: var(--text-dim);
  }
  .popup {
    position: fixed;
    z-index: 200;
    overflow-y: auto;
    padding: 1px 0;
  }
  /* QtComboBoxPopupMenuButton: height 20, text-offset 5 0 */
  .item {
    height: 20px;
    padding: 0 8px 0 5px;
    font: var(--fs) / 20px var(--font);
    color: #c0c0c0;
    white-space: nowrap;
    cursor: pointer;
  }
  .item.sel {
    color: #f4f4f4;
  }
  .item.active {
    color: #f4f4f4;
    background-color: #ffffff1b;
  }
  .item.disabled {
    color: #888888;
    cursor: default;
  }
</style>
