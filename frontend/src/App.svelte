<script lang="ts">
  import { Events } from "@wailsio/runtime";
  import { onMount } from "svelte";
  import AboutDialog from "./features/dialogs/AboutDialog.svelte";
  import CompileDialog from "./features/dialogs/CompileDialog.svelte";
  import BusyOverlay from "./lib/ui/BusyOverlay.svelte";
  import NewDialog from "./features/dialogs/NewDialog.svelte";
  import OpenDialog from "./features/dialogs/OpenDialog.svelte";
  import UpdateDialog from "./features/dialogs/UpdateDialog.svelte";
  import { initUpdates } from "./lib/state/updates.svelte";
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
  import ReorderDialog from "./features/tools/ReorderDialog.svelte";
  import ReplaceRefsDialog from "./features/tools/ReplaceRefsDialog.svelte";
  import ExportSpritesDialog from "./features/tools/ExportSpritesDialog.svelte";
  import ItemsOtbDialog from "./features/dialogs/ItemsOtbDialog.svelte";
  import ShareDialog from "./features/market/ShareDialog.svelte";
  import { listPanelWidth, loadPrefs, prefs } from "./lib/state/prefs.svelte";
  import { setMinLayoutWidth } from "./lib/pixelscale";
  import StartScreen from "./features/StartScreen.svelte";
  import SpriteList from "./features/SpriteList.svelte";
  import StatusBar from "./features/StatusBar.svelte";
  import ThingList from "./features/ThingList.svelte";
  import { commands, handlePaste, handleShortcut } from "./lib/commands";
  import { followAgents } from "./lib/map/agents";
  import { app, initState } from "./lib/state/app.svelte";
  import { readPositive, writeStored } from "./lib/state/storage";
  import Splitter from "./lib/ui/Splitter.svelte";
  import ContextMenu from "./lib/ui/ContextMenu.svelte";
  import ConfirmDialog from "./lib/ui/ConfirmDialog.svelte";
  import PromptDialog from "./lib/ui/PromptDialog.svelte";
  import { WindowService, type FilesDropped } from "./lib/api";
  import Toasts from "./lib/ui/Toasts.svelte";
  import MapEditor from "./features/map/MapEditor.svelte";
  import { map } from "./lib/map/state.svelte";
  import { mapShortcut } from "./lib/map/view/keys";

  let group = $state(0);
  let mapHover = $state("");

  // In the map editor only map shortcuts apply (Ctrl+S saves the map, not the client).
  function onkeydown(event: KeyboardEvent) {
    if (map.mode === "map") {
      // The play view takes the keys itself.
      if (!app.dialog && !map.dialog && !map.play) mapShortcut(event);
      return;
    }
    handleShortcut(event);
  }

  const SPRITES_KEY = "layout:sprites-height";
  const SPRITES_MIN = 120;
  const EDITOR_MIN = 266; // preview window minimum (stage + outfit controls) plus the splitter
  let center = $state<HTMLDivElement>();
  let spritesHeight = $state(loadHeight());

  function loadHeight(): number {
    return readPositive(SPRITES_KEY) ?? 260;
  }

  function resizeSprites(dy: number) {
    const max = (center?.clientHeight ?? 800) - EDITOR_MIN;
    spritesHeight = Math.round(Math.max(SPRITES_MIN, Math.min(max, spritesHeight - dy)));
  }

  function saveHeight() {
    writeStored(SPRITES_KEY, String(spritesHeight));
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
    const offDrop = Events.On("app:files-dropped", (event: { data: FilesDropped | null }) =>
      void commands.drop(event.data?.paths ?? [], event.data?.target ?? "", group),
    );
    const offAgents = followAgents();
    return () => {
      offClose();
      offDrop();
      offAgents();
    };
  });

  // Without a client only the start screen is shown; it opens the editor asked for.
  $effect(() => {
    if (!app.open) map.mode = "objects";
  });

  const listWidth = $derived(listPanelWidth(prefs.settings.listColumns || 7));
  $effect(() => setMinLayoutWidth(listWidth + 740));
</script>

<svelte:window {onkeydown} />
<svelte:document onpaste={(event) => map.mode === "objects" && handlePaste(event, group)} />

<div class="shell" data-file-drop-target>
  <MenuBar />
  {#if !app.open}
    <main><StartScreen /></main>
  {:else if map.mode === "map"}
    <MapEditor bind:hoverText={mapHover} />
  {:else}
  <main style="--list-w:{listWidth}px">
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
  </main>
  {/if}
  <StatusBar {mapHover} />
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
{#if app.dialog === "compare" && app.open}<CompareDialog />{/if}
{#if app.dialog === "reorder" && app.open}<ReorderDialog />{/if}
{#if app.dialog === "replaceRefs" && app.open && app.selectedSprites.length}<ReplaceRefsDialog />{/if}
{#if app.dialog === "exportSprites" && app.open}<ExportSpritesDialog />{/if}
{#if app.dialog === "itemsOtb" && app.open}<ItemsOtbDialog />{/if}
{#if app.dialog === "share" && app.shareTarget && (app.open || app.shareTarget.kind === "file")}<ShareDialog />{/if}
<ConfirmDialog />
<PromptDialog />
<BusyOverlay />
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
    gap: 0;
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
</style>
