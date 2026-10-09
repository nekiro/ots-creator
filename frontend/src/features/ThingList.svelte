<script lang="ts">
  import { CATEGORIES, CATEGORY_LABELS, Category, Format, errorMessage, minId, normalizeThing, res, ThingService, type Thing } from "../lib/api";
  import { commands, thingDropId } from "../lib/commands";
  import { FLAG_GROUPS, flagVisible } from "../lib/flags";
  import { openContextMenu } from "../lib/menu.svelte";
  import { objectListMenu, objectMenu } from "../lib/menus";
  import { spriteCache } from "../lib/render/sprites";
  import { LIST_CELL } from "../lib/prefs.svelte";
  import { app, select, setCategory, toast, versions } from "../lib/state.svelte";
  import Icon from "../lib/ui/Icon.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import Select from "../lib/ui/Select.svelte";
  import ThingCanvas from "../lib/ui/ThingCanvas.svelte";
  import VirtualGrid from "../lib/ui/VirtualGrid.svelte";

  const CONTENT_OPTIONS = [
    { value: "", label: "All objects" },
    { value: "used", label: "With sprites" },
    { value: "empty", label: "Empty only" },
  ];

  let content = $state("");
  let flag = $state("");
  // Text in the search box filters by name; a number jumps to an id.
  let jump = $state("");
  const name = $derived(/^\s*\d*\s*$/.test(jump) ? "" : jump.trim());
  // Names of the named objects of the category (asset clients).
  let names = $state<Record<number, string>>({});
  // Matching ids while a filter is active; null shows the whole range.
  let ids = $state<number[] | null>(null);

  const flagOptions = $derived([
    { value: "", label: "Any flag" },
    ...FLAG_GROUPS.flatMap((g) => g.flags)
      .filter((f) => flagVisible(f.key, app.category, false))
      .map((f) => ({ value: f.key as string, label: f.label })),
  ]);
  const filtered = $derived(content !== "" || flag !== "" || name !== "");

  // A flag hidden for the new category no longer applies.
  $effect(() => {
    if (flag && !flagOptions.some((o) => o.value === flag)) flag = "";
  });

  let request = 0;
  $effect(() => {
    const c = app.category;
    const f = { content, flag, name };
    void app.rev;
    if (!app.open || !filtered) {
      ids = null;
      return;
    }
    const n = ++request;
    ThingService.Find(c, f).then(
      (r) => n === request && (ids = r ?? []),
      (e) => n === request && toast(errorMessage(e), "error"),
    );
  });

  const first = $derived(minId(app.category));
  const total = $derived(Math.max(0, app.maxId(app.category) - first + 1));
  const count = $derived(ids ? ids.length : total);
  const idAt = (i: number) => (ids ? ids[i] : first + i);
  const indexOf = (id: number) => (ids ? ids.indexOf(id) : id - first);
  const focusIndex = $derived.by(() => {
    if (app.focused === null) return null;
    const i = indexOf(app.focused);
    return i < 0 ? null : i;
  });
  $effect(() => {
    const c = app.category;
    void app.rev;
    if (!app.open || app.project?.info.format !== Format.FormatAssets) {
      names = {};
      return;
    }
    ThingService.Names(c).then((n) => c === app.category && (names = (n ?? {}) as Record<number, string>));
  });

  function go(id: number) {
    const max = app.maxId(app.category);
    void select(Math.min(Math.max(id, first), max));
  }

  function goIndex(i: number) {
    if (count > 0) void select(idAt(Math.min(Math.max(i, 0), count - 1)));
  }

  function onJump(e: KeyboardEvent) {
    if (e.key !== "Enter") return;
    if (name) {
      if (count > 0) goIndex(0);
      return;
    }
    const n = parseInt(jump, 10);
    if (Number.isFinite(n)) go(n);
  }

  // Animated thumbnail of the hovered object (animated objects only).
  let hover = $state<{ id: number; thing: Thing } | null>(null);
  let hoverReady = $state(0);
  let hoverId: number | null = null;
  let hoverTimer: ReturnType<typeof setTimeout> | undefined;

  function enter(id: number) {
    hoverId = id;
    clearTimeout(hoverTimer);
    hoverTimer = setTimeout(async () => {
      const cat = app.category;
      try {
        const raw = await ThingService.Get(cat, id);
        if (!raw || hoverId !== id || cat !== app.category) return;
        const t = normalizeThing(raw);
        const g = t.frameGroups[t.category === Category.CategoryOutfit && t.frameGroups.length > 1 ? 1 : 0];
        if (!g || g.frames < 2) return;
        await spriteCache.load(g.sprites);
        if (hoverId !== id) return;
        hover = { id, thing: t };
        hoverReady++;
      } catch {
        /* thumbnail stays static */
      }
    }, 150);
  }

  function leave() {
    hoverId = null;
    clearTimeout(hoverTimer);
    hover = null;
  }

  function click(e: MouseEvent, id: number) {
    if (e.shiftKey && app.focused !== null) {
      const a = indexOf(app.focused);
      const b = indexOf(id);
      if (a >= 0 && b >= 0) {
        const [lo, hi] = a < b ? [a, b] : [b, a];
        const range = Array.from({ length: hi - lo + 1 }, (_, k) => idAt(lo + k));
        app.selection = [app.focused, ...range.filter((x) => x !== app.focused)];
        return;
      }
    }
    void select(id, e.ctrlKey || e.metaKey);
  }

  function oncontextmenu(e: MouseEvent, id: number) {
    if (!app.selection.includes(id)) void select(id);
    openContextMenu(e, objectMenu(true));
  }

  function onkeydown(e: KeyboardEvent, columns: number) {
    if (app.focused === null) return;
    const step: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -columns, ArrowDown: columns, PageUp: -columns * 8, PageDown: columns * 8 };
    if (e.key in step) {
      e.preventDefault();
      const i = indexOf(app.focused);
      goIndex(i < 0 ? 0 : i + step[e.key]);
    } else if (e.key === "Home") {
      e.preventDefault();
      goIndex(0);
    } else if (e.key === "End") {
      e.preventDefault();
      goIndex(count - 1);
    } else if (e.key === "Delete") {
      e.preventDefault();
      void commands.remove();
    }
  }
</script>

<MiniWindow title="Objects" class="things">
  {#snippet actions()}
    <span class="count">
      {#if app.selection.length > 1}{app.selection.length} selected · {/if}{filtered ? `${count.toLocaleString()} / ${total.toLocaleString()}` : total.toLocaleString()}
    </span>
  {/snippet}
  <div class="t-tabs">
    {#each CATEGORIES as c}
      <button class="t-tab" class:on={app.category === c} onclick={() => setCategory(c)}>{CATEGORY_LABELS[c]}</button>
    {/each}
  </div>
  <div class="bar">
    <Icon name="search" />
    <input class="t-input grow" placeholder={app.project?.info.format === Format.FormatAssets ? "Go to id or search name…" : "Go to id… (Enter)"} bind:value={jump} onkeydown={onJump} />
  </div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="list t-panel" oncontextmenu={(e) => !(e.target as HTMLElement).closest(".thing") && openContextMenu(e, objectListMenu())}>
    {#if app.open}
      <VirtualGrid {count} cellWidth={LIST_CELL.width} cellHeight={48} gap={LIST_CELL.gap} scrollTo={focusIndex} {onkeydown}>
        {#snippet cell(i)}
          {@const id = idAt(i)}
          <button
            id={thingDropId(id)}
            data-file-drop-target
            class="thing"
            class:sel={app.selection.includes(id)}
            class:focus={app.focused === id}
            title={names[id] ? `#${id} ${names[id]}` : `#${id}`}
            onclick={(e) => click(e, id)}
            onmouseenter={() => enter(id)}
            onmouseleave={leave}
            ondblclick={() => select(id)}
            oncontextmenu={(e) => oncontextmenu(e, id)}
          >
            <span class="t-slot">
              {#if hover?.id === id}
                <ThingCanvas thing={hover.thing} get={(sid) => spriteCache.get(sid)?.pixels} size={spriteCache.size} ready={hoverReady} group={hover.thing.frameGroups.length > 1 ? 1 : 0} fit={32} colorize={false} improved={app.project?.info.format === Format.FormatAssets || !!app.project?.info.features.improvedAnimations} />
              {:else}
                <img class="pixel" src={res.thumb(app.category, id, versions.thing(app.category, id))} alt="" loading="lazy" decoding="async" draggable="false" />
              {/if}
            </span>
            <span class="id">{id}</span>
          </button>
        {/snippet}
      </VirtualGrid>
    {/if}
    {#if app.open && filtered && count === 0}
      <div class="none t-label">No objects match the filter.</div>
    {/if}
  </div>
  <div class="row filters">
    <Select grow value={content} options={CONTENT_OPTIONS} disabled={!app.open} onchange={(v) => (content = v)} />
    <Select grow value={flag} options={flagOptions} disabled={!app.open} maxItems={14} onchange={(v) => (flag = v)} />
    <button class="t-icon-btn" title="Clear filters" disabled={!filtered} onclick={() => ((content = ""), (flag = ""), (jump = ""))}><Icon name="close" /></button>
  </div>
  <div class="row foot">
    <button class="t-icon-btn" title="New object" disabled={!app.open} onclick={commands.newThing}><Icon name="plus" /></button>
    <button class="t-icon-btn" title="Duplicate (Ctrl+D)" disabled={app.focused === null} onclick={commands.duplicate}><Icon name="copy" /></button>
    <button class="t-icon-btn" title="Replace with object (OBD)" disabled={app.selection.length !== 1} onclick={() => commands.importObd(true)}><Icon name="replace" /></button>
    <button class="t-icon-btn" title="Remove (Del)" disabled={app.focused === null} onclick={commands.remove}><Icon name="trash" /></button>
    <span class="grow"></span>
    <button class="t-icon-btn" title="Import OBD (Ctrl+I)" disabled={!app.open} onclick={() => commands.importObd()}><Icon name="import" /></button>
    <button class="t-icon-btn" title="Export OBD (Ctrl+E)" disabled={app.focused === null} onclick={() => commands.exportObd()}><Icon name="export" /></button>
  </div>
</MiniWindow>

<style>
  .count {
    color: var(--text-dim);
    font-weight: normal;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 6px 2px;
    color: var(--text-dim);
  }
  .list {
    flex: 1;
    min-height: 0;
    display: flex;
    padding: 3px;
  }
  .thing {
    width: 36px;
    height: 48px;
    padding: 1px;
    border: 1px solid transparent;
    background: none;
    display: flex;
    flex-direction: column;
    align-items: center;
    cursor: pointer;
    font: inherit;
    color: var(--text-dim);
  }
  /* Clicks always target the button: the hover animation swaps the img for
     a canvas, and a press that starts on a removed element never clicks. */
  .thing :global(*) {
    pointer-events: none;
  }
  .thing:hover {
    background: rgba(255, 255, 255, 0.05);
  }
  .thing.sel {
    background: var(--select);
    color: var(--text-bright);
  }
  .thing.focus {
    border-color: #b6b6b6;
    color: #fff;
  }
  /* A file dragged over an object (Wails marks the drop target). */
  .thing:global(.file-drop-target-active) {
    border-color: var(--gold);
    background: rgba(255, 210, 90, 0.15);
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
  .list {
    position: relative;
  }
  .none {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
  }
  .filters {
    margin-top: 4px;
    gap: 3px;
  }
  .foot {
    margin-top: 4px;
    gap: 2px;
  }
</style>
