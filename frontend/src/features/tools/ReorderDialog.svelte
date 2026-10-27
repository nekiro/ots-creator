<script lang="ts">
  import { CATEGORIES, CATEGORY_LABELS, CATEGORY_NAMES, Category, minId, res, ThingService } from "../../lib/api";
  import { app, run, toast, versions } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import { loading as imgLoading } from "../../lib/ui/loading";
  import { clamp } from "../../lib/util/math";
  import { clickSelection, idRange } from "../../lib/util/selection";

  let category = $state<Category>(app.category);
  let insert = $state(false);
  let updateRefs = $state(true);

  const first = $derived(minId(category));
  const end = $derived(app.maxId(category) + 1);
  const count = $derived(end - first + 1);
  const idAt = (index: number) => first + index;

  let selected = $state<number[]>([]);
  let anchor: number | null = null;
  let held = $state<number[]>([]);
  let over = $state<number | null>(null);
  let dragging = $state<number[]>([]);
  let recent = $state<Set<number>>(new Set());
  let scrollTo = $state<number | null>(null);

  const moving = $derived(dragging.length ? dragging : held);
  const selectedSet = $derived(new Set(selected));
  const heldSet = $derived(new Set(held));
  const landing = $derived.by(() => {
    const out = new Set<number>();
    if (over === null || insert) return out;
    for (let i = 0; i < moving.length; i++) out.add(over + i);
    return out;
  });

  function setCategory(next: Category) {
    category = next;
    selected = [];
    held = [];
    anchor = null;
    scrollTo = 0;
  }

  function click(event: MouseEvent, id: number) {
    if (held.length) {
      void move(held, id);
      return;
    }
    if (id === end) return;
    const next = clickSelection(event, selected, anchor, id, (from) => idRange(from, id));
    selected = next.selection;
    anchor = next.anchor;
  }

  function cut() {
    if (!selected.length) return;
    held = [...selected].sort((left, right) => left - right);
    toast(`Click the slot where ${held.length > 1 ? `#${held[0]} and the others go` : `#${held[0]} goes`} (Esc cancels).`, "info", 3500, "reorder");
  }

  function onkeydown(event: KeyboardEvent) {
    const mod = event.ctrlKey || event.metaKey;
    if (event.key === "Escape" && held.length) {
      event.preventDefault();
      event.stopPropagation();
      held = [];
      over = null;
    } else if (mod && event.key.toLowerCase() === "x") {
      event.preventDefault();
      cut();
    } else if (mod && event.key.toLowerCase() === "a") {
      event.preventDefault();
      selected = Array.from({ length: end - first }, (_, k) => first + k);
    }
  }

  function dragstart(event: DragEvent, id: number) {
    if (id === end || !event.dataTransfer) return;
    if (!selectedSet.has(id)) {
      selected = [id];
      anchor = id;
    }
    dragging = [...selected].sort((left, right) => left - right);
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", dragging.join(","));
  }

  function dragend() {
    dragging = [];
    over = null;
  }

  function drop(event: DragEvent, id: number) {
    event.preventDefault();
    const ids = dragging;
    dragend();
    if (ids.length) void move(ids, id);
  }

  async function move(ids: number[], at: number, insertMode = insert) {
    if (!ids.length) return;
    const movedCount = ids.length;
    const moved = await run("Moving", () => ThingService.Move(category, ids, at, { insert: insertMode, updateRefs: updateRefs && category === Category.CategoryItem }));
    held = [];
    over = null;
    if (!moved) return;
    selected = moved;
    anchor = moved[0] ?? null;
    recent = new Set(moved);
    setTimeout(() => (recent = new Set()), 1500);
    const low = Math.min(...moved);
    const high = Math.max(...moved);
    toast(`Moved ${movedCount} ${CATEGORY_NAMES[category]}(s) to ${low === high ? `#${low}` : `#${low}-#${high}`}. Ctrl+Z undoes it.`, "success", 3500, "reorder");
  }

  function swapTwo() {
    if (selected.length !== 2) return;
    const [firstId, secondId] = selected;
    void move([firstId], secondId, false);
  }

  let target = $state("");
  function moveTo(event?: KeyboardEvent) {
    if (event && event.key !== "Enter") return;
    const at = parseInt(target, 10);
    if (!Number.isFinite(at) || !selected.length) return;
    void move([...selected].sort((left, right) => left - right), at);
  }

  let jump = $state("");
  function onJump(event: KeyboardEvent) {
    if (event.key !== "Enter") return;
    const id = parseInt(jump, 10);
    if (!Number.isFinite(id)) return;
    scrollTo = clamp(id, first, end) - first;
  }

  $effect(() => {
    void app.rev;
    if (selected.some((id) => id >= end)) selected = selected.filter((id) => id < end);
  });
</script>

<Dialog title="Reorder Objects" width={640} onclose={() => (app.dialog = null)}>
  <div class="col reorder">
    <div class="row">
      <div class="t-tabs">
        {#each CATEGORIES as tabCategory}
          <button class="t-tab" class:on={category === tabCategory} onclick={() => setCategory(tabCategory)}>{CATEGORY_LABELS[tabCategory]}</button>
        {/each}
      </div>
      <span class="grow"></span>
      <input class="t-input jump" placeholder="Go to id… (Enter)" bind:value={jump} onkeydown={onJump} />
    </div>
    <div class="row">
      <Select
        width={150}
        value={insert}
        options={[
          { value: false, label: "Swap with target" },
          { value: true, label: "Insert and shift" },
        ]}
        title="Swap: the objects at the target take the freed ids. Insert: the objects go before the target and the ones between shift."
        onchange={(value) => (insert = value)}
      />
      <label class="row check" class:off={category !== Category.CategoryItem} title="Change item ids stored in items (market trade as and show as, former object, NPC currency) to the new ids">
        <input class="t-check" type="checkbox" bind:checked={updateRefs} disabled={category !== Category.CategoryItem} />Update item references
      </label>
      <span class="grow"></span>
      <span class="t-label hint">Drag, or Ctrl+X and click a slot.</span>
    </div>

    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="slots t-panel" class:holding={held.length > 0} onmouseleave={() => !dragging.length && (over = null)}>
      <VirtualGrid {count} cellWidth={36} cellHeight={48} gap={2} {scrollTo} {onkeydown}>
        {#snippet cell(i)}
          {@const id = idAt(i)}
          {@const isEnd = id === end}
          <button
            class="slot"
            class:sel={selectedSet.has(id)}
            class:held={heldSet.has(id)}
            class:landing={landing.has(id)}
            class:insert={insert && over === id && moving.length > 0}
            class:recent={recent.has(id)}
            class:end={isEnd}
            title={isEnd ? "After the last object" : `#${id}`}
            draggable={!isEnd && !held.length}
            disabled={!!app.busy}
            onclick={(event) => click(event, id)}
            onmouseenter={() => held.length && (over = id)}
            ondragstart={(event) => dragstart(event, id)}
            ondragend={dragend}
            ondragover={(event) => {
              if (!dragging.length) return;
              event.preventDefault();
              over = id;
            }}
            ondrop={(event) => drop(event, id)}
          >
            <span class="t-slot">
              {#if isEnd}
                <Icon name="plus" />
              {:else}
                {@const src = res.thumb(category, id, versions.thing(category, id))}
                <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:imgLoading={src} />
              {/if}
            </span>
            <span class="id">{isEnd ? "end" : id}</span>
          </button>
        {/snippet}
      </VirtualGrid>
    </div>

    <div class="row actions">
      <span class="t-label">
        {#if held.length}{held.length} cut · click a slot{:else if selected.length}{selected.length} selected{:else}Select objects (Ctrl, Shift, Ctrl+A).{/if}
      </span>
      <span class="grow"></span>
      <button class="t-btn" title="Swap the two selected objects" disabled={selected.length !== 2 || !!app.busy} onclick={swapTwo}>Swap two</button>
      <button class="t-btn" title="Cut the selection, then click the slot where it goes (Ctrl+X)" disabled={!selected.length || !!app.busy} onclick={cut}>Cut</button>
      <span class="t-sep v"></span>
      <input class="t-input to" placeholder="Move to id" bind:value={target} onkeydown={moveTo} disabled={!selected.length} />
      <button class="t-btn" disabled={!selected.length || !target || !!app.busy} onclick={() => moveTo()}>Move</button>
    </div>
  </div>
  {#snippet footer()}
    <span class="t-label note">Servers find items by client id (items.otb, items.xml): update them after moving items.</span>
    <span class="grow"></span>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .reorder {
    gap: 6px;
  }
  .jump {
    width: 140px;
  }
  .check {
    gap: 4px;
    white-space: nowrap;
    cursor: pointer;
    user-select: none;
  }
  .check.off {
    opacity: 0.5;
  }
  .hint,
  .note {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .slots {
    display: flex;
    height: 380px;
    padding: 3px;
  }
  .slot {
    width: 36px;
    height: 48px;
    padding: 1px;
    border: 1px solid transparent;
    background: none;
    display: flex;
    flex-direction: column;
    align-items: center;
    font: inherit;
    color: var(--text-dim);
    cursor: pointer;
  }
  .slot :global(*) {
    pointer-events: none;
  }
  .slot:hover {
    background: rgba(255, 255, 255, 0.05);
  }
  .holding .slot {
    cursor: crosshair;
  }
  .slot.sel {
    background: var(--select);
    color: var(--text-bright);
  }
  .slot.held {
    border: 1px dashed var(--gold);
    opacity: 0.6;
  }
  .slot.landing {
    border-color: #8fd18f;
    background: rgba(143, 209, 143, 0.15);
  }
  .slot.insert {
    border-left: 3px solid var(--gold);
  }
  .slot.recent {
    border-color: var(--gold);
    background: rgba(255, 210, 90, 0.18);
  }
  .slot.end {
    color: #8fd18f;
  }
  .t-slot img {
    max-width: 32px;
    max-height: 32px;
    object-fit: contain;
  }
  .id {
    font: var(--fs-small) / 12px var(--font-small);
    text-shadow: 1px 1px 0 #000;
  }
  .actions {
    gap: 4px;
  }
  .actions > .t-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .to {
    width: 90px;
  }
  .t-sep.v {
    width: 1px;
    align-self: stretch;
    margin: 0 4px;
  }
</style>
