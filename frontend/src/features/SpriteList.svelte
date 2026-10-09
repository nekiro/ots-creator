<script lang="ts">
  import { openContextMenu } from "../lib/menu.svelte";
  import { spriteListMenu, spriteMenu } from "../lib/menus";
  import { SpriteService, errorMessage, res } from "../lib/api";
  import { commands } from "../lib/commands";
  import { app, toast, versions } from "../lib/state.svelte";
  import { startSpriteDrag } from "../lib/dragsprites";
  import Icon from "../lib/ui/Icon.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import VirtualGrid from "../lib/ui/VirtualGrid.svelte";
  import Select from "../lib/ui/Select.svelte";
  import { loading } from "../lib/ui/loading";

  const FILTERS = [
    { value: "", label: "All sprites" },
    { value: "unused", label: "Unused" },
    { value: "empty", label: "Empty" },
    { value: "duplicate", label: "Duplicates" },
  ];

  let mode = $state<"all" | "object">("object");
  let jump = $state("");
  let filter = $state("");
  // Ids matching the filter of the All tab; refreshed after every change.
  let found = $state<number[] | null>(null);
  $effect(() => {
    const f = filter;
    void app.rev;
    if (!f || mode !== "all" || !app.open) {
      found = null;
      return;
    }
    let stale = false;
    SpriteService.Find(f).then(
      (ids) => !stale && (found = ids ?? []),
      (e) => toast(errorMessage(e), "error"),
    );
    return () => (stale = true);
  });

  const total = $derived(app.project?.info.counts.sprites ?? 0);
  const objectIds = $derived.by(() => {
    const t = app.draft;
    if (!t) return [] as number[];
    const seen = new Set<number>();
    for (const g of t.frameGroups) for (const id of g?.sprites ?? []) if (id) seen.add(id);
    return [...seen];
  });
  const ids = $derived(mode === "object" ? objectIds : filter ? (found ?? []) : null);
  const count = $derived(ids ? ids.length : total);
  const idAt = (i: number) => (ids ? ids[i] : i + 1);
  const focusIndex = $derived.by(() => {
    const id = app.selectedSprites[0];
    if (!id) return null;
    return ids ? ids.indexOf(id) : id - 1;
  });

  function pick(id: number, e: MouseEvent) {
    if (e.ctrlKey || e.metaKey) {
      app.selectedSprites = app.selectedSprites.includes(id) ? app.selectedSprites.filter((x) => x !== id) : [...app.selectedSprites, id];
    } else if (e.shiftKey && app.selectedSprites.length && ids) {
      const a = app.selectedSprites[0];
      const [i, j] = [ids.indexOf(a), ids.indexOf(id)];
      if (i >= 0 && j >= 0) app.selectedSprites = [a, ...ids.slice(Math.min(i, j), Math.max(i, j) + 1).filter((x) => x !== a)];
    } else if (e.shiftKey && app.selectedSprites.length && !ids) {
      const a = app.selectedSprites[0];
      const [lo, hi] = a < id ? [a, id] : [id, a];
      app.selectedSprites = [a, ...Array.from({ length: hi - lo + 1 }, (_, k) => lo + k).filter((x) => x !== a)];
    } else {
      app.selectedSprites = [id];
    }
  }

  function onJump(e: KeyboardEvent) {
    if (e.key !== "Enter") return;
    const n = parseInt(jump, 10);
    if (Number.isFinite(n) && n >= 1 && n <= total) {
      mode = "all";
      filter = "";
      app.selectedSprites = [n];
    }
  }

  function onkeydown(e: KeyboardEvent, columns: number) {
    const cur = focusIndex ?? 0;
    const step: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -columns, ArrowDown: columns };
    if (e.key in step) {
      e.preventDefault();
      const i = Math.min(Math.max(cur + step[e.key], 0), count - 1);
      if (count) app.selectedSprites = [idAt(i)];
    } else if (e.key === "Delete") {
      e.preventDefault();
      void commands.removeSprites();
    }
  }

  function ondragstart(e: DragEvent, id: number) {
    startSpriteDrag(e, id, app.selectedSprites);
  }
</script>

<MiniWindow title="Sprites" class="sprites">
  {#snippet actions()}
    <span class="count">{total.toLocaleString()}</span>
  {/snippet}
  <div class="row top">
    <div class="t-tabs tabs">
      <button class="t-tab" class:on={mode === "object"} onclick={() => (mode = "object")}>Object</button>
      <button class="t-tab" class:on={mode === "all"} onclick={() => (mode = "all")}>All</button>
    </div>
    <input class="t-input grow" placeholder="Sprite id… (Enter)" bind:value={jump} onkeydown={onJump} inputmode="numeric" />
  </div>
  {#if mode === "all"}
    <div class="row filter">
      <Select grow value={filter} options={FILTERS} disabled={!app.open} onchange={(v) => ((filter = v), (app.selectedSprites = []))} />
      {#if filter && found}
        <span class="t-label">{found.length.toLocaleString()}</span>
        <button class="t-btn" title="Select every sprite in the list" disabled={!found.length} onclick={() => (app.selectedSprites = [...found!])}>All</button>
      {/if}
    </div>
  {/if}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="list t-panel" oncontextmenu={(e) => !(e.target as HTMLElement).closest(".spr") && openContextMenu(e, spriteListMenu())}>
    {#if app.open && count > 0}
      <VirtualGrid {count} cellWidth={36} cellHeight={48} gap={2} scrollTo={focusIndex} {onkeydown}>
        {#snippet cell(i)}
          {@const id = idAt(i)}
          {@const src = res.spritePng(id, versions.sprite(id))}
          <button
            class="spr"
            class:sel={app.selectedSprites.includes(id)}
            draggable="true"
            ondragstart={(e) => ondragstart(e, id)}
            onclick={(e) => pick(id, e)}
            oncontextmenu={(e) => {
              if (!app.selectedSprites.includes(id)) app.selectedSprites = [id];
              openContextMenu(e, spriteMenu(true));
            }}
            title="Sprite #{id}: drag onto a preview tile (a multi-selection fills the following slots)"
          >
            <span class="t-slot"><img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:loading={src} /></span>
            <span class="id">{id}</span>
          </button>
        {/snippet}
      </VirtualGrid>
    {:else if app.open}
      <div class="empty t-label">{mode === "object" ? "This object has no sprites yet." : filter ? "No sprites match the filter." : "No sprites."}</div>
    {/if}
  </div>
  <div class="row foot">
    <button class="t-icon-btn" title="Import images as sprites" disabled={!app.open} onclick={commands.importSprites}><Icon name="image" /></button>
    <button class="t-icon-btn" title="Replace selected sprite" disabled={!app.selectedSprites.length} onclick={commands.replaceSprite}><Icon name="import" /></button>
    <button class="t-icon-btn" title="Export selected sprites" disabled={!app.selectedSprites.length} onclick={commands.exportSprites}><Icon name="export" /></button>
    <button class="t-icon-btn" title="Clear selected sprites (Del)" disabled={!app.selectedSprites.length} onclick={commands.removeSprites}><Icon name="trash" /></button>
    <span class="grow"></span>
    {#if app.selectedSprites.length > 1}<span class="t-label">{app.selectedSprites.length} selected</span>{/if}
  </div>
</MiniWindow>

<style>
  .count {
    color: var(--text-dim);
    font-weight: normal;
  }
  .top {
    margin: 2px 2px 6px;
  }
  .filter {
    gap: 4px;
    margin: 0 2px 6px;
  }
  .tabs {
    width: 120px;
  }
  .list {
    flex: 1;
    min-height: 0;
    display: flex;
    padding: 3px;
  }
  .spr {
    width: 36px;
    height: 48px;
    padding: 1px;
    border: 1px solid transparent;
    background: none;
    display: flex;
    flex-direction: column;
    align-items: center;
    cursor: grab;
    font: inherit;
    color: var(--text-dim);
  }
  .spr :global(*) {
    pointer-events: none;
  }
  .spr:hover {
    background: rgba(255, 255, 255, 0.05);
  }
  .spr.sel {
    background: var(--select-strong);
    border-color: var(--blue);
    color: #fff;
  }
  .t-slot img {
    width: 32px;
    height: 32px;
  }
  .id {
    font: var(--fs-small) / 12px var(--font-small);
    text-shadow: 1px 1px 0 #000;
  }
  .empty {
    margin: auto;
  }
  .foot {
    margin-top: 4px;
    gap: 2px;
  }
</style>
