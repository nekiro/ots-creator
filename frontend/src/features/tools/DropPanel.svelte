<script lang="ts">
  // Right side of the compare window: the queue of objects of B and the
  // slots of A. A click on a slot drops the next queued object there.
  import { CATEGORY_NAMES, errorMessage, minId, res, ThingService, type Category } from "../../lib/api";
  import { openContextMenu, type MenuEntry } from "../../lib/menu.svelte";
  import { clearQueue, dequeue, drop, queued, toFront } from "../../lib/queue.svelte";
  import { app, otherVersions, toast, versions } from "../../lib/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import VirtualGrid from "../../lib/ui/VirtualGrid.svelte";
  import { loading as imgLoading } from "../../lib/ui/loading";

  let { category, onshow }: { category: Category; onshow: (id: number) => void } = $props();

  const queue = $derived(queued(category));
  const next = $derived(queue[0] ?? null);

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
      (e) => n === request && toast(errorMessage(e), "error"),
    );
  });
  const first = $derived(minId(category));
  const end = $derived(app.maxId(category) + 1);
  const count = $derived((empty ? empty.length : end - first) + 1);
  const idAt = (i: number) => (i === count - 1 ? end : empty ? empty[i] : first + i);

  let jump = $state("");
  let scrollTo = $state<number | null>(null);
  function onJump(e: KeyboardEvent) {
    if (e.key !== "Enter") return;
    const id = parseInt(jump, 10);
    if (!Number.isFinite(id)) return;
    const i = empty ? empty.findIndex((x) => x >= id) : Math.min(Math.max(id, first), end) - first;
    scrollTo = i < 0 ? count - 1 : i;
  }

  // Ids that just got objects light up for a moment.
  let recent = $state<Set<number>>(new Set());
  let timer: ReturnType<typeof setTimeout> | undefined;
  async function dropAt(at: number, n = 1, free = false) {
    const got = await drop(category, at, n, free);
    if (!got.length) return;
    recent = new Set(got);
    clearTimeout(timer);
    timer = setTimeout(() => (recent = new Set()), 1500);
  }

  let hovered = $state<number | null>(null);

  function slotMenu(e: MouseEvent, id: number) {
    const none = () => !queue.length || !!app.busy;
    const at = id === end ? 0 : id;
    const items: MenuEntry[] = [
      { label: next !== null ? `Drop #${next} here` : "Drop next here", action: () => dropAt(at), disabled: none },
      { label: `Drop all ${queue.length} from here (free ids)`, action: () => dropAt(at === 0 ? end : at, 0, true), disabled: none },
      { label: `Drop all ${queue.length} from here (in a row)`, action: () => dropAt(at === 0 ? end : at, 0, false), disabled: none },
      { label: `Drop all ${queue.length} at the end`, action: () => dropAt(0, 0), disabled: none },
    ];
    if (id !== end) items.push("-", { label: "Show in list", action: () => onshow(id) });
    openContextMenu(e, items);
  }

  function queueMenu(e: MouseEvent, id: number) {
    openContextMenu(e, [
      { label: "Drop next", action: () => toFront(category, id) },
      { label: "Remove from queue", action: () => dequeue(category, [id]) },
      { label: "Show in list", action: () => onshow(id) },
      "-",
      { label: "Clear queue", action: () => clearQueue(category) },
    ]);
  }
</script>

<div class="drop col">
  <div class="row head">
    <strong>Queue</strong>
    <span class="t-label">{queue.length ? `${queue.length} ${CATEGORY_NAMES[category]}(s) from B` : "empty"}</span>
    <span class="grow"></span>
    <button class="t-icon-btn" title="Clear the queue" disabled={!queue.length} onclick={() => clearQueue(category)}><Icon name="close" /></button>
  </div>
  <div class="queue t-panel">
    {#if !queue.length}
      <div class="t-label hint">Double click objects of B to queue them.</div>
    {:else}
      {#each queue as id, i (id)}
        {@const src = res.otherThumb(category, id, otherVersions.thing(category, id))}
        <button class="q" class:next={i === 0} title={i === 0 ? `#${id} drops next (right click for more)` : `#${id}: click to drop next`} onclick={() => toFront(category, id)} oncontextmenu={(e) => queueMenu(e, id)}>
          <span class="t-slot"><img class="pixel" {src} alt="" decoding="async" draggable="false" use:imgLoading={src} /></span>
          <span class="id">{id}</span>
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
          oncontextmenu={(e) => slotMenu(e, id)}
        >
          <span class="t-slot">
            {#if next !== null && hovered === id}
              {@const src = res.otherThumb(category, next, otherVersions.thing(category, next))}
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
    padding: 3px;
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
    font: var(--fs-small) / 12px var(--font-small);
    text-shadow: 1px 1px 0 #000;
  }
  .filter {
    gap: 4px;
    cursor: pointer;
    user-select: none;
  }
</style>
