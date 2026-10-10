<script lang="ts">
  import { Browser, Window } from "@wailsio/runtime";
  import { commands } from "../lib/commands";
  import { checkForUpdates, REPO_URL, updates } from "../lib/updates.svelte";
  import { getMode, SCALE_MODES, setMode } from "../lib/pixelscale";
  import { app } from "../lib/state.svelte";
  import type { MenuEntry, MenuItem } from "../lib/menu.svelte";
  import { editMenu, objectMenu, spriteMenu, toolsMenu } from "../lib/menus";
  import { prefs, recentDir, shortPath } from "../lib/prefs.svelte";
  import Icon from "../lib/ui/Icon.svelte";
  import MenuEntries from "../lib/ui/MenuEntries.svelte";

  const noProject = () => !app.open;
  // Compiling only makes sense with changes, or for a client never saved.
  const nothingToCompile = () => !app.open || !(app.project?.info.changed || app.dirty || !app.project?.info.datPath);

  // The maximize button turns into "restore" while the window is maximized.
  let maximised = $state(false);
  async function syncMaximised() {
    try {
      maximised = await Window.IsMaximised();
    } catch {
      /* not running inside Wails */
    }
  }
  $effect(() => {
    void syncMaximised();
  });

  const recentItems = $derived(
    (prefs.settings.recent ?? []).slice(0, 5).map((r): MenuEntry => ({
      label: `${r.version.name} · ${shortPath(recentDir(r), 42)}`,
      action: () => commands.openRecent(r),
    })),
  );

  const menus: { label: string; items: MenuEntry[] }[] = $derived([
    {
      label: "File",
      items: [
        { label: "New client…", keys: "Ctrl+N", action: commands.create },
        { label: "Open client…", keys: "Ctrl+O", action: () => commands.open() },
        ...(recentItems.length ? (["-", ...recentItems] as MenuEntry[]) : []),
        "-",
        { label: "Compile", keys: "Ctrl+S", action: commands.compile, disabled: nothingToCompile },
        { label: "Compile as…", keys: "Ctrl+Shift+S", action: commands.compileAs, disabled: noProject },
        "-",
        { label: "Settings…", action: () => (app.dialog = "settings") },
        "-",
        { label: "Close client", action: commands.close, disabled: noProject },
      ],
    },
    {
      label: "Edit",
      items: editMenu(),
    },
    {
      label: "Object",
      items: objectMenu(),
    },
    {
      label: "Sprites",
      items: spriteMenu(),
    },
    {
      label: "Tools",
      items: toolsMenu(),
    },
    {
      label: "View",
      items: SCALE_MODES.map((m) => ({
        label: m.label,
        checked: () => scaleMode === m.value,
        action: () => {
          scaleMode = m.value;
          void setMode(m.value);
        },
      })),
    },
    {
      label: "Help",
      items: [
        { label: "Check for updates…", action: () => checkForUpdates(true), disabled: () => updates.checking || updates.version === "" },
        { label: "Project on GitHub", action: () => Browser.OpenURL(REPO_URL) },
        "-",
        { label: "About OTS Creator", action: () => (app.dialog = "about") },
      ],
    },
  ]);

  let openMenu = $state<number | null>(null);
  let scaleMode = $state(getMode());

  function pick(item: MenuItem) {
    openMenu = null;
    if (!item.disabled?.()) item.action();
  }
</script>

<svelte:window onresize={syncMaximised} onmousedown={(e) => !(e.target as HTMLElement).closest(".menubar") && (openMenu = null)} />

<nav class="menubar" ondblclick={(e) => e.target === e.currentTarget || (e.target as HTMLElement).matches(".spacer, .logo") ? Window.ToggleMaximise() : null}>
  <img class="logo" src="/icon.png" alt="" width="16" height="16" draggable="false" />
  {#each menus as m, i}
    <div class="menu">
      <button
        class="top"
        class:on={openMenu === i}
        onmousedown={() => (openMenu = openMenu === i ? null : i)}
        onmouseenter={() => openMenu !== null && (openMenu = i)}>{m.label}</button
      >
      {#if openMenu === i}
        <div class="drop t-menu" role="menu">
          <MenuEntries items={m.items} onpick={pick} />
        </div>
      {/if}
    </div>
  {/each}

  <div class="toolbar">
    <button class="t-icon-btn" title="Open client (Ctrl+O)" onclick={() => commands.open()}><Icon name="open" /></button>
    <button class="t-icon-btn" title="Compile (Ctrl+S)" disabled={nothingToCompile()} onclick={commands.compile}><Icon name="save" /></button>
    <span class="t-vsep"></span>
    <button class="t-icon-btn" title="Undo (Ctrl+Z)" disabled={!app.project?.info.canUndo} onclick={commands.undo}><Icon name="undo" /></button>
    <button class="t-icon-btn" title="Redo (Ctrl+Y)" disabled={!app.project?.info.canRedo} onclick={commands.redo}><Icon name="redo" /></button>
    <span class="t-vsep"></span>
    <button class="t-icon-btn" title="Import objects (Ctrl+I)" disabled={!app.open} onclick={() => commands.importObd()}><Icon name="import" /></button>
    <button class="t-icon-btn" title="Export selected objects (Ctrl+E)" disabled={!app.open || app.focused === null} onclick={() => commands.exportObd()}><Icon name="export" /></button>
    <span class="t-vsep"></span>
    <button class="t-btn market" title="Browse objects and sprites shared by other users" onclick={commands.market}><Icon name="market" />Market</button>
  </div>
  <div class="spacer"></div>
  <!-- Frameless window: OTClient miniwindow buttons instead of the native frame. -->
  <div class="winctl">
    <button class="wbtn min" title="Minimize" aria-label="Minimize" onclick={() => Window.Minimise()}></button>
    <button
      class="wbtn"
      class:max={!maximised}
      class:restore={maximised}
      title={maximised ? "Restore" : "Maximize"}
      aria-label={maximised ? "Restore" : "Maximize"}
      onclick={() => Window.ToggleMaximise()}
    ></button>
    <button class="wbtn close" title="Close" aria-label="Close" onclick={() => Window.Close()}></button>
  </div>
</nav>

<style>
  .menubar {
    height: 31px;
    flex: none;
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 8px;
    background: url("../assets/ui/panel_map.png");
    border-bottom: 1px solid #111;
    box-shadow: 0 1px 0 #444 inset;
    position: relative;
    z-index: 20;
    /* Drag the frameless window by the bar; controls opt out below. */
    --wails-draggable: drag;
    user-select: none;
  }
  .menu,
  .toolbar,
  .winctl {
    --wails-draggable: no-drag;
  }
  .menu {
    position: relative;
  }
  .top {
    font: inherit;
    color: var(--text-bright);
    background: none;
    border: 1px solid transparent;
    height: 22px;
    padding: 0 9px;
    cursor: pointer;
    text-shadow: 1px 1px 0 #000;
  }
  .top:hover,
  .top.on {
    background: rgba(255, 255, 255, 0.08);
    border-color: #555 #222 #222 #555;
  }
  .drop {
    position: absolute;
    top: 25px;
    left: 0;
    min-width: 200px;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    margin-left: 14px;
  }
  .market {
    display: flex;
    align-items: center;
    gap: 5px;
    height: 23px;
    padding: 0 10px 0 7px;
    color: var(--gold);
    text-shadow: 1px 1px 0 #000;
  }
  .market :global(svg) {
    width: 16px;
    height: 16px;
  }
  .toolbar .t-vsep {
    /* .t-vsep stretches; a fixed height would sit at the top. */
    align-self: center;
    height: 18px;
    margin: 0 4px;
  }
  .spacer {
    flex: 1;
  }
  .logo {
    flex: none;
    margin-right: 4px;
  }
  .winctl {
    display: flex;
    gap: 3px;
    margin-left: 12px;
  }
  /* window_buttons.png (tools/icon/winbuttons.py, from OTClient's
     miniwindow_buttons.png): 14x14 cells, columns minimize / maximize /
     restore / close, rows idle / hover / pressed */
  .wbtn {
    width: 14px;
    height: 14px;
    padding: 0;
    border: 0;
    background: url("../assets/ui/window_buttons.png") no-repeat;
    image-rendering: pixelated;
    cursor: pointer;
  }
  .wbtn.min {
    background-position: 0 0;
  }
  .wbtn.max {
    background-position: -14px 0;
  }
  .wbtn.restore {
    background-position: -28px 0;
  }
  .wbtn.close {
    background-position: -42px 0;
  }
  .wbtn:hover {
    background-position-y: -14px;
  }
  .wbtn:active {
    background-position-y: -28px;
  }
</style>
