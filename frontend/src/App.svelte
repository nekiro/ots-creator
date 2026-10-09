<script lang="ts">
  import { Events } from "@wailsio/runtime";
  import { onMount } from "svelte";
  import AboutDialog from "./features/dialogs/AboutDialog.svelte";
  import CompileDialog from "./features/dialogs/CompileDialog.svelte";
  import NewDialog from "./features/dialogs/NewDialog.svelte";
  import OpenDialog from "./features/dialogs/OpenDialog.svelte";
  import UpdateDialog from "./features/dialogs/UpdateDialog.svelte";
  import { initUpdates } from "./lib/updates.svelte";
  import MenuBar from "./features/MenuBar.svelte";
  import Preview from "./features/Preview.svelte";
  import Properties from "./features/Properties.svelte";
  import BulkEdit from "./features/BulkEdit.svelte";
  import SettingsDialog from "./features/dialogs/SettingsDialog.svelte";
  import DurationsDialog from "./features/tools/DurationsDialog.svelte";
  import ObdViewerDialog from "./features/tools/ObdViewerDialog.svelte";
  import OptimizeDialog from "./features/tools/OptimizeDialog.svelte";
  import SlicerDialog from "./features/tools/SlicerDialog.svelte";
  import SheetExportDialog from "./features/tools/SheetExportDialog.svelte";
  import MarketDialog from "./features/market/MarketDialog.svelte";
  import CompareDialog from "./features/tools/CompareDialog.svelte";
  import ReplaceRefsDialog from "./features/tools/ReplaceRefsDialog.svelte";
  import ExportSpritesDialog from "./features/tools/ExportSpritesDialog.svelte";
  import ShareDialog from "./features/market/ShareDialog.svelte";
  import { listPanelWidth, loadPrefs, prefs, recentDir, removeRecent, shortPath } from "./lib/prefs.svelte";
  import { setMinLayoutWidth } from "./lib/pixelscale";
  import SpriteList from "./features/SpriteList.svelte";
  import StatusBar from "./features/StatusBar.svelte";
  import ThingList from "./features/ThingList.svelte";
  import { commands, handlePaste, handleShortcut } from "./lib/commands";
  import { app, initState } from "./lib/state.svelte";
  import Splitter from "./lib/ui/Splitter.svelte";
  import ContextMenu from "./lib/ui/ContextMenu.svelte";
  import ConfirmDialog from "./lib/ui/ConfirmDialog.svelte";
  import { WindowService, type FilesDropped } from "./lib/api";
  import Toasts from "./lib/ui/Toasts.svelte";

  let group = $state(0);

  // Height of the sprite list; the splitter trades it with the preview.
  const SPRITES_KEY = "layout:sprites-height";
  const SPRITES_MIN = 120;
  const EDITOR_MIN = 266; // preview window minimum (stage + outfit controls) plus the splitter
  let center = $state<HTMLDivElement>();
  let spritesHeight = $state(loadHeight());

  function loadHeight(): number {
    try {
      const v = Number(localStorage.getItem(SPRITES_KEY));
      if (v > 0) return v;
    } catch {
      /* storage unavailable */
    }
    return 260;
  }

  function resizeSprites(dy: number) {
    const max = (center?.clientHeight ?? 800) - EDITOR_MIN;
    spritesHeight = Math.round(Math.max(SPRITES_MIN, Math.min(max, spritesHeight - dy)));
  }

  function saveHeight() {
    try {
      localStorage.setItem(SPRITES_KEY, String(spritesHeight));
    } catch {
      /* ignore */
    }
  }

  // The backend holds window closes while edits are unapplied.
  $effect(() => {
    const dirty = app.dirty;
    WindowService.SetUnapplied(dirty).catch(() => {});
  });

  onMount(() => {
    void (async () => {
      await loadPrefs();
      await initState();
      void initUpdates(prefs.settings.checkUpdates);
      const last = prefs.settings.recent?.[0];
      if (prefs.settings.reopenLast && last && !app.open) await commands.openRecent(last);
    })();
    const offClose = Events.On("app:close-requested", () => void commands.quit());
    const offDrop = Events.On("app:files-dropped", (e: { data: FilesDropped | null }) =>
      void commands.drop(e.data?.paths ?? [], e.data?.target ?? "", group),
    );
    return () => {
      offClose();
      offDrop();
    };
  });

  const recent = $derived((prefs.settings.recent ?? []).slice(0, 6));

  // The object list panel is as wide as its chosen number of columns.
  const listWidth = $derived(listPanelWidth(prefs.settings.listColumns || 7));
  $effect(() => setMinLayoutWidth(listWidth + 740));
</script>

<svelte:window onkeydown={handleShortcut} />
<svelte:document onpaste={(e) => handlePaste(e, group)} />

<div class="shell" data-file-drop-target>
  <MenuBar />
  <main style="--list-w:{listWidth}px">
    {#if app.open}
      <div class="left">
        <ThingList />
      </div>
      <div class="center" bind:this={center} style="--sprites-h:{spritesHeight}px">
        <section class="t-window editor">
          <div class="t-window-title">Preview</div>
          <Preview bind:group />
        </section>
        <Splitter ondrag={resizeSprites} onend={saveHeight} />
        <SpriteList />
      </div>
      <div class="right">
        {#if app.selection.length > 1}<BulkEdit />{:else}<Properties bind:group />{/if}
      </div>
    {:else}
      <div class="welcome">
        <section class="t-window card">
          <div class="t-window-title">OTS Creator</div>
          <img class="pixel" src="/icon.png" alt="" width="48" height="48" />
          <p>Open a client (Tibia.dat + Tibia.spr) or start a new one.</p>
          <div class="row">
            <button class="t-btn primary" onclick={() => commands.open()}>Open client…</button>
            <button class="t-btn" onclick={commands.create}>New client…</button>
          </div>
          {#if recent.length}
            <div class="recent">
              <div class="t-label rtitle">Recent clients</div>
              <div class="t-panel rlist">
                {#each recent as r (r.datPath)}
                  <div class="ritem">
                    <button class="ropen" title={r.datPath} onclick={() => commands.openRecent(r)}>
                      <span class="rver">{r.version.name}</span>
                      <span class="rdir">{shortPath(recentDir(r), 44)}</span>
                    </button>
                    <button class="rdel" title="Remove from list" aria-label="Remove from list" onclick={() => removeRecent(r)}>×</button>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
          <p class="t-label hint"><kbd>Ctrl+O</kbd> open · <kbd>Ctrl+N</kbd> new · drop a client folder, .dat or .obd here</p>
        </section>
      </div>
    {/if}
  </main>
  <StatusBar />
</div>

{#if app.dialog === "open"}<OpenDialog />{/if}
{#if app.dialog === "new"}<NewDialog />{/if}
{#if app.dialog === "compile" && app.open}<CompileDialog />{/if}
{#if app.dialog === "about"}<AboutDialog />{/if}
{#if app.dialog === "update"}<UpdateDialog />{/if}
{#if app.dialog === "settings"}<SettingsDialog />{/if}
{#if app.dialog === "optimize" && app.open}<OptimizeDialog />{/if}
{#if app.dialog === "durations" && app.open}<DurationsDialog />{/if}
{#if app.dialog === "slicer" && app.open}<SlicerDialog />{/if}
{#if app.dialog === "obd"}<ObdViewerDialog />{/if}
{#if app.dialog === "sheet" && app.open && app.focused !== null}<SheetExportDialog />{/if}
{#if app.dialog === "market"}<MarketDialog />{/if}
{#if app.dialog === "compare" && app.open && app.other?.open}<CompareDialog />{/if}
{#if app.dialog === "replaceRefs" && app.open && app.selectedSprites.length}<ReplaceRefsDialog />{/if}
{#if app.dialog === "exportSprites" && app.open}<ExportSpritesDialog />{/if}
{#if app.dialog === "share" && app.shareTarget && (app.open || app.shareTarget.kind === "file")}<ShareDialog />{/if}
<ConfirmDialog />
<Toasts />
<ContextMenu />

<style>
  .shell {
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  main {
    flex: 1;
    min-height: 0;
    display: grid;
    /* The object list keeps its width; the editor keeps at least 460px and
       the properties panel gives up width first. The sum (with gaps) is
       MIN_LAYOUT.width in pixelscale.ts. */
    /* --list-w fits the chosen objects per row (Settings). */
    grid-template-columns: var(--list-w, 300px) minmax(460px, 1fr) minmax(250px, 280px);
    min-width: calc(var(--list-w, 300px) + 734px); /* 460 + 250 + gaps */
    gap: 6px;
    padding: 6px;
  }
  .left,
  .right,
  .center {
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .left :global(.things),
  .right :global(.props) {
    flex: 1;
  }
  .center {
    gap: 0;
  }
  .center :global(.sprites) {
    flex: none;
    height: var(--sprites-h);
    min-height: 120px;
    max-height: calc(100% - 266px); /* EDITOR_MIN */
  }
  .editor {
    flex: 1;
    min-height: 260px;
    padding: 8px 10px 8px;
  }
  .welcome {
    grid-column: 1 / -1;
    display: grid;
    place-items: center;
  }
  .card {
    width: 400px;
    padding: 16px 20px 14px;
    align-items: center;
    text-align: center;
    gap: 10px;
  }
  .card p {
    margin: 0;
  }
  .hint {
    font: var(--fs-small) / 10px var(--font-small);
  }
  .recent {
    width: 100%;
    text-align: left;
  }
  .rtitle {
    margin-bottom: 3px;
  }
  .rlist {
    padding: 2px;
  }
  .ritem {
    display: flex;
    align-items: center;
  }
  .ritem:hover {
    background: rgba(255, 255, 255, 0.06);
  }
  .ropen {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 8px;
    padding: 3px 6px;
    background: none;
    border: 0;
    font: inherit;
    color: var(--text);
    text-align: left;
    cursor: pointer;
  }
  .ropen:hover .rdir {
    color: var(--text-bright);
  }
  .rver {
    color: var(--gold);
    flex: none;
    min-width: 34px;
  }
  .rdir {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rdel {
    flex: none;
    width: 18px;
    background: none;
    border: 0;
    color: var(--text-dim);
    font: inherit;
    cursor: pointer;
    visibility: hidden;
  }
  .ritem:hover .rdel {
    visibility: visible;
  }
  .rdel:hover {
    color: #ff9c9c;
  }
</style>
