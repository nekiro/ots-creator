<script lang="ts">
  import { hasImprovedAnimations } from "../lib/thing";
  import { keyStep } from "../lib/ui/listNav";
  import { anchoredRange, idRange } from "../lib/util/selection";
  import { CATEGORIES, CATEGORY_LABELS, Format, errorMessage, minId, normalizeThing, res, ThingService } from "../lib/api";
  import { commands, thingDropId } from "../lib/commands";
  import { FLAG_GROUPS, flagVisible } from "../lib/flags";
  import { openContextMenu } from "../lib/menu.svelte";
  import { objectListMenu, objectMenu } from "../lib/menus";
  import { HoverAnim } from "../lib/render/hoveranim.svelte";
  import { spriteCache } from "../lib/render/sprites";
  import { LIST_CELL } from "../lib/state/prefs.svelte";
  import { app, select, setCategory, toast, versions } from "../lib/state/app.svelte";
  import Icon from "../lib/ui/Icon.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import Select from "../lib/ui/Select.svelte";
  import ThingCanvas from "../lib/ui/ThingCanvas.svelte";
  import VirtualGrid from "../lib/ui/VirtualGrid.svelte";
  import { loading } from "../lib/ui/loading";
  import { clamp } from "../lib/util/math";

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
  let names = $state.raw<Record<number, string>>({});
  // Matching ids while a filter is active; null shows the whole range.
  let ids = $state.raw<number[] | null>(null);

  const flagOptions = $derived([
    { value: "", label: "Any flag" },
    ...FLAG_GROUPS.flatMap((group) => group.flags)
      .filter((field) => flagVisible(field.key, app.category, false))
      .map((field) => ({ value: field.key as string, label: field.label })),
  ]);
  const filtered = $derived(content !== "" || flag !== "" || name !== "");

  // A flag hidden for the new category no longer applies.
  $effect(() => {
    if (flag && !flagOptions.some((option) => option.value === flag)) flag = "";
  });

  let request = 0;
  $effect(() => {
    const category = app.category;
    const query = { content, flag, name };
    void app.rev;
    if (!app.open || !filtered) {
      ids = null;
      return;
    }
    const thisRequest = ++request;
    ThingService.Find(category, query).then(
      (found) => thisRequest === request && (ids = found ?? []),
      (err) => thisRequest === request && toast(errorMessage(err), "error"),
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
    const category = app.category;
    void app.rev;
    if (!app.open || app.project?.info.format !== Format.FormatAssets) {
      names = {};
      return;
    }
    ThingService.Names(category).then((loaded) => category === app.category && (names = (loaded ?? {}) as Record<number, string>));
  });

  function go(id: number) {
    const max = app.maxId(app.category);
    void select(clamp(id, first, max));
  }

  function goIndex(i: number) {
    if (count > 0) void select(idAt(clamp(i, 0, count - 1)));
  }

  function onJump(event: KeyboardEvent) {
    if (event.key !== "Enter") return;
    if (name) {
      if (count > 0) goIndex(0);
      return;
    }
    const typed = parseInt(jump, 10);
    if (Number.isFinite(typed)) go(typed);
  }

  // Animated thumbnail of the hovered object (animated objects only).
  const hover = new HoverAnim(spriteCache, async (id) => {
    const cat = app.category;
    const raw = await ThingService.Get(cat, id);
    return raw && cat === app.category ? normalizeThing(raw) : null;
  });

  function click(event: MouseEvent, id: number) {
    if (event.shiftKey && app.focused !== null) {
      const focusedIndex = indexOf(app.focused);
      const clickedIndex = indexOf(id);
      if (focusedIndex >= 0 && clickedIndex >= 0) {
        app.selection = anchoredRange(app.focused, idRange(focusedIndex, clickedIndex).map(idAt));
        return;
      }
    }
    void select(id, event.ctrlKey || event.metaKey);
  }

  function oncontextmenu(event: MouseEvent, id: number) {
    if (!app.selectionSet.has(id)) void select(id);
    openContextMenu(event, objectMenu(true));
  }

  /** Rows Page Up and Page Down move. */
  const PAGE_ROWS = 8;

  function onkeydown(event: KeyboardEvent, columns: number) {
    if (app.focused === null) return;
    const step = keyStep(event.key, { columns, pageRows: PAGE_ROWS });
    if (step !== null) {
      event.preventDefault();
      const current = indexOf(app.focused);
      goIndex(current < 0 ? 0 : current + step);
    } else if (event.key === "Home") {
      event.preventDefault();
      goIndex(0);
    } else if (event.key === "End") {
      event.preventDefault();
      goIndex(count - 1);
    } else if (event.key === "Delete") {
      event.preventDefault();
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
    {#each CATEGORIES as tabCategory}
      <button class="t-tab" class:on={app.category === tabCategory} onclick={() => setCategory(tabCategory)}>{CATEGORY_LABELS[tabCategory]}</button>
    {/each}
  </div>
  <div class="bar">
    <Icon name="search" />
    <input class="t-input grow" placeholder={app.project?.info.format === Format.FormatAssets ? "Go to id or search name…" : "Go to id… (Enter)"} bind:value={jump} onkeydown={onJump} />
  </div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="list t-panel" oncontextmenu={(event) => !(event.target as HTMLElement).closest(".thing") && openContextMenu(event, objectListMenu())}>
    {#if app.open}
      <VirtualGrid {count} cellWidth={LIST_CELL.width} cellHeight={48} gap={LIST_CELL.gap} scrollTo={focusIndex} {onkeydown}>
        {#snippet cell(i)}
          {@const id = idAt(i)}
          <button
            id={thingDropId(id)}
            data-file-drop-target
            class="thing"
            class:sel={app.selectionSet.has(id)}
            class:focus={app.focused === id}
            title={names[id] ? `#${id} ${names[id]}` : `#${id}`}
            onclick={(event) => click(event, id)}
            onmouseenter={() => hover.enter(id)}
            onmouseleave={() => hover.leave()}
            ondblclick={() => select(id)}
            oncontextmenu={(event) => oncontextmenu(event, id)}
          >
            <span class="t-slot">
              {#if hover.current?.id === id}
                <ThingCanvas thing={hover.current.thing} get={(sid) => spriteCache.get(sid)?.pixels} size={spriteCache.size} ready={hover.ready} group={HoverAnim.group(hover.current.thing)} fit={32} colorize={false} improved={hasImprovedAnimations(app.project?.info)} />
              {:else}
                {@const src = res.thumb(app.category, id, versions.thing(app.category, id))}
                <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:loading={src} />
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
    <Select grow value={content} options={CONTENT_OPTIONS} disabled={!app.open} onchange={(value) => (content = value)} />
    <Select grow value={flag} options={flagOptions} disabled={!app.open} maxItems={14} onchange={(value) => (flag = value)} />
    <button class="t-icon-btn" title="Clear filters" disabled={!filtered} onclick={() => ((content = ""), (flag = ""), (jump = ""))}><Icon name="close" /></button>
  </div>
  <div class="row foot">
    <button class="t-icon-btn" title="New object" disabled={!app.open} onclick={commands.newThing}><Icon name="plus" /></button>
    <button class="t-icon-btn" title="Duplicate (Ctrl+D)" disabled={app.focused === null} onclick={commands.duplicate}><Icon name="copy" /></button>
    <button class="t-icon-btn" title="Replace with object (OBD)" disabled={app.selection.length !== 1} onclick={() => commands.importObd(true)}><Icon name="replace" /></button>
    <button class="t-icon-btn danger" title="Remove (Del)" disabled={app.focused === null} onclick={commands.remove}><Icon name="trash" /></button>
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
