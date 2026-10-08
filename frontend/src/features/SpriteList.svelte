<script lang="ts">
  import { openContextMenu } from "../lib/menu.svelte";
  import { spriteMenu } from "../lib/menus";
  import { res } from "../lib/api";
  import { commands } from "../lib/commands";
  import { app, versions } from "../lib/state.svelte";
  import { startSpriteDrag } from "../lib/dragsprites";
  import Icon from "../lib/ui/Icon.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import VirtualGrid from "../lib/ui/VirtualGrid.svelte";

  let mode = $state<"all" | "object">("object");
  let jump = $state("");

  const total = $derived(app.project?.info.counts.sprites ?? 0);
  const objectIds = $derived.by(() => {
    const t = app.draft;
    if (!t) return [] as number[];
    const seen = new Set<number>();
    for (const g of t.frameGroups) for (const id of g?.sprites ?? []) if (id) seen.add(id);
    return [...seen];
  });
  const ids = $derived(mode === "object" ? objectIds : null);
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
  <div class="list t-panel">
    {#if app.open && count > 0}
      <VirtualGrid {count} cellWidth={36} cellHeight={48} gap={2} scrollTo={focusIndex} {onkeydown}>
        {#snippet cell(i)}
          {@const id = idAt(i)}
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
            <span class="t-slot"><img class="pixel" src={res.spritePng(id, versions.sprite(id))} alt="" loading="lazy" decoding="async" draggable="false" /></span>
            <span class="id">{id}</span>
          </button>
        {/snippet}
      </VirtualGrid>
    {:else if app.open}
      <div class="empty t-label">{mode === "object" ? "This object has no sprites yet." : "No sprites."}</div>
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
