<script lang="ts">
  // Queue panel of the compare window and the object viewer: the queued
  // objects and the slots of A. A click on a slot drops the next queued
  // object there.
  import { CATEGORY_NAMES, errorMessage, minId, res, ThingService, type Category } from "../../lib/api";
  import { openContextMenu, type MenuEntry } from "../../lib/menu.svelte";
  import type { DropQueue, QueueItem } from "../../lib/queue.svelte";
  import { app, toast, versions } from "../../lib/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import { loading as imgLoading } from "../../lib/ui/loading";
  import { clamp } from "../../lib/math";

  let {
    queue: q,
    category,
    onshow,
    onshowSlot,
  }: {
    queue: DropQueue<any>;
    category: Category;
    /** Shows a queued object in the window's list. */
    onshow: (key: any) => void;
    /** Shows an object of A in the window's list. */
    onshowSlot?: (id: number) => void;
  } = $props();

  const queue = $derived(q.list(category));
  const next = $derived<QueueItem | null>(queue[0] ?? null);

  // Slots of A: every id and one past the end (dropping there appends).
  let emptyOnly = $state(false);
  let empty = $state<number[] | null>(null);
  let request = 0;
  $effect(() => {
    const c = category;
    void app.rev;
    if (!emptyOnly) {
      empty = null;
      return;
    }
    const n = ++request;
    ThingService.Find(c, { content: "empty", flag: "", name: "" }).then(
      (r) => n === request && (empty = r ?? []),
      (err) => n === request && toast(errorMessage(err), "error"),
    );
  });
  const first = $derived(minId(category));
  const end = $derived(app.maxId(category) + 1);
  const count = $derived((empty ? empty.length : end - first) + 1);
  const idAt = (i: number) => (i === count - 1 ? end : empty ? empty[i] : first + i);

  let jump = $state("");
  let scrollTo = $state<number | null>(null);
  function onJump(event: KeyboardEvent) {
    if (event.key !== "Enter") return;
    const id = parseInt(jump, 10);
    if (!Number.isFinite(id)) return;
    const i = empty ? empty.findIndex((x) => x >= id) : clamp(id, first, end) - first;
    scrollTo = i < 0 ? count - 1 : i;
  }

  // Ids that just got objects light up for a moment.
  let recent = $state<Set<number>>(new Set());
  let timer: ReturnType<typeof setTimeout> | undefined;
  async function dropAt(at: number, n = 1, free = false) {
    const got = await q.drop(category, at, n, free);
    if (!got.length) return;
    recent = new Set(got);
    clearTimeout(timer);
    timer = setTimeout(() => (recent = new Set()), 1500);
  }

  let hovered = $state<number | null>(null);

  function slotMenu(event: MouseEvent, id: number) {
    const none = () => !queue.length || !!app.busy;
    const at = id === end ? 0 : id;
    const items: MenuEntry[] = [
      { label: next !== null ? `Drop ${q.label(next)} here` : "Drop next here", action: () => dropAt(at), disabled: none },
      { label: `Drop all ${queue.length} from here (free ids)`, action: () => dropAt(at === 0 ? end : at, 0, true), disabled: none },
      { label: `Drop all ${queue.length} from here (in a row)`, action: () => dropAt(at === 0 ? end : at, 0, false), disabled: none },
      { label: `Drop all ${queue.length} at the end`, action: () => dropAt(0, 0), disabled: none },
    ];
    if (id !== end && onshowSlot) items.push("-", { label: "Show in list", action: () => onshowSlot(id) });
    openContextMenu(event, items);
  }

  function queueMenu(event: MouseEvent, key: number | string) {
    openContextMenu(event, [
      { label: "Drop next", action: () => q.toFront(category, key) },
      { label: "Remove from queue", action: () => q.dequeue(category, [key]) },
      { label: "Show in list", action: () => onshow(key) },
      "-",
      { label: "Clear queue", action: () => q.clear(category) },
    ]);
  }
</script>

<div class="drop col">
  <div class="row head">
    <strong>Queue</strong>
    <span class="t-label">{queue.length ? `${queue.length} ${CATEGORY_NAMES[category]}(s) ${q.src.from}` : "empty"}</span>
    <span class="grow"></span>
    <button class="t-icon-btn" title="Clear the queue" disabled={!queue.length} onclick={() => q.clear(category)}><Icon name="close" /></button>
  </div>
  <div class="queue t-panel">
    {#if !queue.length}
      <div class="t-label hint">{q.src.hint}</div>
    {:else}
      {#each queue as it, i (it.key)}
        {@const src = q.thumb(it)}
        {@const label = q.label(it)}
        <button class="q" class:next={i === 0} title={i === 0 ? `${label} drops next (right click for more)` : `${label}: click to drop next`} onclick={() => q.toFront(category, it.key)} oncontextmenu={(event) => queueMenu(event, it.key)}>
          <span class="t-slot"><img class="pixel" {src} alt="" decoding="async" draggable="false" use:imgLoading={src} /></span>
          <span class="id">{typeof it.key === "number" ? it.key : label}</span>
        </button>
      {/each}
    {/if}
  </div>

  <div class="row head">
    <strong>Slots of A</strong>
    <span class="grow"></span>
    <label class="row filter" title="Show only objects without sprites"><input class="t-check" type="checkbox" bind:checked={emptyOnly} />Empty only</label>
  </div>
  <input class="t-input" placeholder="Go to id… (Enter)" bind:value={jump} onkeydown={onJump} />
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="slots t-panel" onmouseleave={() => (hovered = null)}>
    <VirtualGrid {count} cellWidth={36} cellHeight={48} gap={2} {scrollTo}>
      {#snippet cell(i)}
        {@const id = idAt(i)}
        {@const isEnd = id === end}
        <button
          class="slot"
          class:end={isEnd}
          class:recent={recent.has(id)}
          class:armed={next !== null && hovered === id}
          title={isEnd ? "New object at the end of A" : `#${id}`}
          disabled={!!app.busy}
          onclick={() => dropAt(isEnd ? 0 : id)}
          onmouseenter={() => (hovered = id)}
          oncontextmenu={(event) => slotMenu(event, id)}
        >
          <span class="t-slot">
            {#if next !== null && hovered === id}
              {@const src = q.thumb(next)}
              <img class="pixel ghost" {src} alt="" decoding="async" draggable="false" />
            {:else if isEnd}
              <Icon name="plus" />
            {:else}
              {@const src = res.thumb(category, id, versions.thing(category, id))}
              <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:imgLoading={src} />
            {/if}
          </span>
          <span class="id">{isEnd ? "new" : id}</span>
        </button>
      {/snippet}
    </VirtualGrid>
  </div>
</div>

<style>
  .drop {
    gap: 4px;
    min-width: 0;
    min-height: 0;
  }
  .head {
    gap: 6px;
  }
  strong {
    color: var(--gold);
  }
  .queue {
    display: flex;
    flex-wrap: wrap;
    align-content: flex-start;
    gap: 2px;
    height: 58px;
    padding: 3px;
    overflow-y: auto;
  }
  .hint {
    margin: auto;
    padding: 0 8px;
    text-align: center;
  }
  .slots {
    flex: 1;
    min-height: 0;
    display: flex;
    padding: 2px;
  }
  .q,
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
  .q :global(*),
  .slot :global(*) {
    pointer-events: none;
  }
  .q:hover,
  .slot:hover {
    background: rgba(255, 255, 255, 0.05);
  }
  .q.next {
    border-color: var(--gold);
    color: var(--text-bright);
  }
  .slot.armed {
    border-color: #8fd18f;
    background: rgba(143, 209, 143, 0.15);
  }
  .slot.recent {
    border-color: var(--gold);
    background: rgba(255, 210, 90, 0.18);
  }
  .slot.end {
    color: #8fd18f;
  }
  .ghost {
    opacity: 0.75;
  }
  .t-slot img {
    max-width: 32px;
    max-height: 32px;
    object-fit: contain;
  }
  .id {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: var(--fs-small) / 12px var(--font-small);
    text-shadow: 1px 1px 0 #000;
  }
  .filter {
    gap: 4px;
    cursor: pointer;
    user-select: none;
  }
</style>
