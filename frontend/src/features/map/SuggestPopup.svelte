<script lang="ts">
  import { errorMessage } from "../../lib/api";
  import { MapService, samePos, type Position, type Suggestion, type Suggestions } from "../../lib/map/data/api";
  import { brushContext, mixBrush, notePick, rerank, selectionOf, suggestionKey, suggestionTitle } from "../../lib/map/editor/suggestions";
  import { map, selectBrush, type BrushSel } from "../../lib/map/state.svelte";
  import { toast } from "../../lib/state/app.svelte";
  import { toCss } from "../../lib/pixelscale";
  import ItemIcon from "./ItemIcon.svelte";

  // Items that fit the tile under the cursor, ranked by the map itself:
  // what lies in the same places, what goes with the brush, which grounds
  // border the place. Ctrl+click picks several for a mix brush.
  let { tile, x, y, place }: { tile: Position; x: number; y: number; place: (brush: BrushSel, tile: Position) => void } = $props();

  type ListId = "here" | "with" | "grounds";
  /** Re-asks while the map is still being learned. */
  const PENDING_POLL_MS = 600;

  let result = $state.raw<Suggestions | null>(null);
  let error = $state("");
  let chosen = $state<ListId>("here");
  let popup = $state<HTMLDivElement>();
  let pos = $state({ left: 0, top: 0 });
  /** Suggestions picked for the mix, by key. */
  let mixed = $state.raw<Suggestion[]>([]);

  const lists = $derived(
    (
      [
        { id: "here", label: "Here", caption: "Fits this tile", items: rerank(result?.here ?? []) },
        { id: "with", label: "With", caption: `Goes with ${map.brush?.label ?? "the brush"}`, items: rerank(result?.with ?? []) },
        { id: "grounds", label: "Grounds", caption: "Borders this place", items: rerank(result?.grounds ?? []) },
      ] as { id: ListId; label: string; caption: string; items: Suggestion[] }[]
    ).filter((list) => list.items.length),
  );
  const shown = $derived(lists.find((list) => list.id === chosen) ?? lists[0]);
  const canMix = $derived(shown?.id !== "grounds");

  $effect(() => {
    const id = map.activeId;
    const at = tile;
    const brush = brushContext();
    if (id === null) return;
    let stale = false;
    let timer = 0;
    const load = async () => {
      try {
        const next = await MapService.Suggest(id, at, brush);
        if (stale) return;
        result = next;
        error = "";
        if (next.pending > 0) timer = window.setTimeout(load, PENDING_POLL_MS);
      } catch (err) {
        if (!stale) error = errorMessage(err);
      }
    };
    void load();
    return () => {
      stale = true;
      clearTimeout(timer);
    };
  });

  $effect(() => {
    void result;
    void mixed;
    if (!popup) return;
    const bounds = popup.getBoundingClientRect();
    const width = toCss(bounds.width);
    const height = toCss(bounds.height);
    pos = {
      left: Math.max(0, Math.min(x + 12, toCss(window.innerWidth) - width - 2)),
      top: Math.max(0, Math.min(y + 12, toCss(window.innerHeight) - height - 2)),
    };
  });

  function close() {
    if (samePos(map.suggest, tile)) map.suggest = null;
  }

  function isMixed(suggestion: Suggestion): boolean {
    return mixed.some((entry) => suggestionKey(entry) === suggestionKey(suggestion));
  }

  function toggleMix(suggestion: Suggestion) {
    if (!canMix) return;
    mixed = isMixed(suggestion) ? mixed.filter((entry) => suggestionKey(entry) !== suggestionKey(suggestion)) : [...mixed, suggestion];
  }

  /** Shift places the suggestion on the tile and keeps the popup open. */
  function pick(suggestion: Suggestion, shift: boolean) {
    if (shown) notePick(shown.items, suggestion);
    const selection = selectionOf(suggestion);
    if (shift) {
      place(selection, tile);
      return;
    }
    selectBrush(selection);
    close();
  }

  async function paintMix() {
    const id = map.activeId;
    if (id === null || mixed.length < 2) return;
    try {
      selectBrush(await mixBrush(id, mixed));
      close();
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  function onclick(event: MouseEvent, suggestion: Suggestion) {
    if (event.ctrlKey || event.metaKey) toggleMix(suggestion);
    else pick(suggestion, event.shiftKey);
  }

  // The popup takes the digits, arrows, Enter and Esc before the map
  // shortcuts.
  $effect(() => {
    const onkeydown = (event: KeyboardEvent) => {
      if (event.altKey || event.metaKey) return;
      const take = () => {
        event.preventDefault();
        event.stopPropagation();
      };
      // The code, not the key: Shift turns the digits into symbols.
      const digit = /^Digit[1-9]$/.test(event.code) ? Number(event.code.slice(5)) : 0;
      if (event.key === "Escape") {
        take();
        close();
      } else if (digit && shown?.items[digit - 1]) {
        take();
        if (event.ctrlKey) toggleMix(shown.items[digit - 1]);
        else pick(shown.items[digit - 1], event.shiftKey);
      } else if (event.key === "Enter" && mixed.length >= 2) {
        take();
        void paintMix();
      } else if ((event.key === "ArrowLeft" || event.key === "ArrowRight") && !event.ctrlKey && lists.length > 1) {
        take();
        const at = lists.findIndex((list) => list.id === shown?.id);
        chosen = lists[(at + (event.key === "ArrowRight" ? 1 : lists.length - 1)) % lists.length].id;
      }
    };
    // Pointer events: the map cancels the mouse events of its clicks.
    const onpointerdown = (event: PointerEvent) => {
      if (popup && !popup.contains(event.target as Node)) close();
    };
    window.addEventListener("keydown", onkeydown, true);
    window.addEventListener("pointerdown", onpointerdown, true);
    return () => {
      window.removeEventListener("keydown", onkeydown, true);
      window.removeEventListener("pointerdown", onpointerdown, true);
    };
  });
</script>

<div bind:this={popup} class="t-menu suggest" role="dialog" aria-label="Suggestions" tabindex="-1" style="left:{pos.left}px;top:{pos.top}px" oncontextmenu={(event) => event.preventDefault()}>
  {#if lists.length > 1}
    <div class="t-tabs" role="tablist">
      {#each lists as list}
        <button class="t-tab" role="tab" class:on={shown?.id === list.id} aria-selected={shown?.id === list.id} title={list.caption} onclick={() => (chosen = list.id)}>{list.label}</button>
      {/each}
    </div>
  {/if}
  {#if shown}
    <div class="caption" title={shown.caption}>{shown.caption}</div>
  {/if}
  {#if error}
    <div class="m-empty">{error}</div>
  {:else if shown}
    <div class="grid">
      {#each shown.items as suggestion, i}
        <button class="m-cell" class:on={isMixed(suggestion)} title={suggestionTitle(suggestion)} onclick={(event) => onclick(event, suggestion)}>
          <ItemIcon id={suggestion.item} />
          {#if i < 9}<span class="number">{i + 1}</span>{/if}
        </button>
      {/each}
    </div>
  {:else}
    <div class="m-empty">{result ? (result.pending ? "Learning the map…" : "Nothing in the map looks like this place yet.") : "…"}</div>
  {/if}
  {#if result?.pending}<div class="note">Learning the map: {result.pending} blocks left</div>{/if}
  {#if mixed.length >= 2}
    <button class="t-btn mix" title="A brush that scatters the picked items (Enter)" onclick={paintMix}>Paint a mix of {mixed.length}</button>
  {:else if mixed.length === 1}
    <div class="note">Ctrl+click one more for a mix</div>
  {:else}
    <div class="note">Shift+click: place here</div>
    {#if canMix}<div class="note">Ctrl+click: mix several</div>{/if}
  {/if}
</div>

<style>
  /* The grid sets the width: 4 cells, their gaps and padding, the frame. */
  .suggest {
    position: fixed;
    z-index: 250;
    width: 174px;
    outline: none;
  }
  .suggest .t-tab {
    min-width: 0;
    padding: 0 4px;
  }
  .caption {
    padding: 4px 4px 0;
    color: var(--text-dim);
    font: var(--fs-small) / 12px var(--font-small);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(4, 36px);
    gap: 4px;
    padding: 4px;
    justify-content: center;
  }
  .number {
    position: absolute;
    left: 1px;
    top: 0;
    color: var(--text-dim);
    font: var(--fs-small) / 10px var(--font-small);
  }
  .note {
    padding: 0 4px 2px;
    color: var(--text-dim);
    font: var(--fs-small) / 12px var(--font-small);
  }
  .mix {
    margin: 2px 4px 4px;
  }
</style>
