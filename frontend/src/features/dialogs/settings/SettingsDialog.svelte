<script lang="ts">
  // Settings of both editors in one place; it opens at the section of the
  // open editor. Every change is saved at once.
  import { onMount } from "svelte";
  import { app, toast, type SettingsSection } from "../../../lib/state/app.svelte";
  import { IMAGE_FORMATS, LIST_COLUMNS, prefs, removeRecent, savePrefs } from "../../../lib/state/prefs.svelte";
  import { KEY_ACTIONS } from "../../../lib/map/view/keys";
  import { loadEditorPrefs, map } from "../../../lib/map/state.svelte";
  import NumberField from "../../../lib/ui/NumberField.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";
  import Select from "../../../lib/ui/Select.svelte";
  import KeySettings from "./KeySettings.svelte";
  import MapSettings, { MAP_OPTIONS } from "./MapSettings.svelte";

  const SECTIONS: { id: SettingsSection; group: string; label: string; editor?: "objects" | "map"; words: string }[] = [
    { id: "general", group: "Application", label: "General", words: "startup check for updates reopen last client recent clients clear list" },
    { id: "agents", group: "Application", label: "AI agents", words: "mcp server port claude code agents copy command" },
    {
      id: "objects",
      group: "Editors",
      label: "Object editor",
      editor: "objects",
      words: "layout objects per row export image format png bmp jpg object files otobj obd objectbuilder sheet background magenta transparent market admin token",
    },
    { id: "map", group: "Editors", label: "Map editor", editor: "map", words: `${MAP_OPTIONS.map((option) => option.label).join(" ")} default spawn time marked items` },
    { id: "keys", group: "Editors", label: "Map shortcuts", editor: "map", words: `keyboard keys hotkeys shortcuts ${KEY_ACTIONS.map((action) => action.label).join(" ")}` },
  ];

  let section = $state<SettingsSection>(app.settingsSection ?? (map.mode === "map" ? "map" : "objects"));
  let query = $state("");

  const shown = $derived.by(() => {
    const words = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
    return SECTIONS.filter((entry) => {
      const text = `${entry.label} ${entry.words}`.toLowerCase();
      return words.every((word) => text.includes(word));
    });
  });

  // A search that hides the shown section moves to the first match.
  $effect(() => {
    if (shown.length && !shown.some((entry) => entry.id === section)) section = shown[0].id;
  });

  onMount(() => {
    void loadEditorPrefs();
    return () => (app.settingsSection = null);
  });

  const settings = $derived(prefs.settings);
  const recent = $derived(settings.recent?.length ?? 0);
  const mcpURL = $derived(`http://127.0.0.1:${settings.mcpPort}/mcp`);

  function copyMCPCommand() {
    void navigator.clipboard
      ?.writeText(`claude mcp add --transport http ots-creator ${mcpURL}`)
      .then(() => toast("Copied the command that connects Claude Code."));
  }
</script>

<Dialog title="Settings" width={580} onclose={() => (app.dialog = null)}>
  <div class="layout">
    <nav class="col nav">
      <input class="t-input" type="search" placeholder="Search" bind:value={query} />
      {#each ["Application", "Editors"] as group}
        {@const entries = shown.filter((entry) => entry.group === group)}
        {#if entries.length}
          <div class="group">{group}</div>
          {#each entries as entry}
            <button class="entry" class:on={section === entry.id} onclick={() => (section = entry.id)}>
              {entry.label}
              {#if entry.editor === map.mode}<span class="here" title="The editor you have open">●</span>{/if}
            </button>
          {/each}
        {/if}
      {/each}
      {#if !shown.length}<span class="t-label none">No setting matches.</span>{/if}
    </nav>

    <div class="col pane">
      {#if section === "general"}
        <h4>Startup</h4>
        <label class="row"><input class="t-check" type="checkbox" checked={settings.checkUpdates} onchange={(event) => savePrefs({ checkUpdates: event.currentTarget.checked })} />Check for updates on startup</label>
        <label class="row"><input class="t-check" type="checkbox" checked={settings.reopenLast} onchange={(event) => savePrefs({ reopenLast: event.currentTarget.checked })} />Reopen the last client on startup</label>

        <h4>Recent clients</h4>
        <div class="row">
          <span class="t-label grow">{recent ? `${recent} remembered` : "None yet"}</span>
          <button class="t-btn danger" disabled={!recent} onclick={() => removeRecent(null)}>Clear list</button>
        </div>
      {:else if section === "agents"}
        <h4>MCP server</h4>
        <label class="row" title="A local MCP server: agents such as Claude Code draw on the open maps, and you see it here">
          <input class="t-check" type="checkbox" checked={settings.mcpServer} onchange={(event) => savePrefs({ mcpServer: event.currentTarget.checked })} />Let AI agents edit maps (MCP server)
        </label>
        <div class="row">
          <NumberField label="Port" value={settings.mcpPort} min={1024} max={65535} width={56} onchange={(value) => savePrefs({ mcpPort: value })} />
          <span class="t-label grow">{mcpURL}</span>
          <button class="t-btn" onclick={copyMCPCommand}>Copy command</button>
        </div>
        <p class="t-label note">Only programs on this computer can connect. Copy command gives the line that adds the server to Claude Code.</p>
      {:else if section === "objects"}
        <h4>Layout</h4>
        <div class="row">
          <NumberField
            label="Objects per row"
            value={settings.listColumns}
            min={LIST_COLUMNS.min}
            max={LIST_COLUMNS.max}
            width={40}
            onchange={(value) => savePrefs({ listColumns: value })}
          />
          <span class="t-label">({LIST_COLUMNS.min}-{LIST_COLUMNS.max}, the object list resizes to fit)</span>
        </div>

        <h4>Export</h4>
        <label class="row">
          <span class="t-label lbl">Image format</span>
          <Select grow value={settings.exportFormat} options={IMAGE_FORMATS} onchange={(value) => savePrefs({ exportFormat: value })} />
        </label>
        <label class="row" title="OTOBJ is readable and editable (JSON and PNG); OBD opens in ObjectBuilder">
          <span class="t-label lbl">Object files</span>
          <Select
            grow
            value={settings.objectFormat}
            options={[
              { value: "otobj", label: "OTOBJ (.otobj)" },
              { value: "obd", label: "ObjectBuilder (.obd)" },
            ]}
            onchange={(value) => savePrefs({ objectFormat: value })}
          />
        </label>
        <label class="row">
          <span class="t-label lbl">Sheet background</span>
          <Select
            grow
            value={settings.sheetBackground}
            options={[
              { value: "magenta", label: "Magenta (ObjectBuilder)" },
              { value: "transparent", label: "Transparent" },
            ]}
            onchange={(value) => savePrefs({ sheetBackground: value })}
          />
        </label>
        <p class="t-label note">BMP and JPG have no transparency; empty pixels become magenta.</p>

        <h4>Market</h4>
        <label class="row">
          <span class="t-label lbl">Admin token</span>
          <input
            class="t-input grow"
            type="password"
            autocomplete="off"
            placeholder="Only for market moderators"
            value={settings.marketAdminToken}
            onchange={(event) => savePrefs({ marketAdminToken: event.currentTarget.value.trim() })}
          />
        </label>
        <p class="t-label note">With the admin token every market entry can be deleted.</p>
      {:else if section === "map"}
        <MapSettings />
      {:else if map.settings}
        <KeySettings />
      {:else}
        <span class="t-label">Loading…</span>
      {/if}
    </div>
  </div>
  {#snippet footer()}
    <span class="t-label grow">Changes are saved at once.</span>
    <button class="t-btn primary" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .layout {
    display: grid;
    grid-template-columns: 130px 1fr;
    gap: 10px;
    height: 360px;
    margin-bottom: 6px;
  }
  .nav {
    gap: 1px;
    min-height: 0;
    padding-right: 8px;
    border-right: 1px solid var(--line-dark);
    box-shadow: 1px 0 0 #3a3a3a;
  }
  .nav .t-input {
    margin-bottom: 4px;
  }
  .group {
    padding: 8px 4px 2px;
    color: var(--text-dim);
    font: var(--fs-small) / 12px var(--font-small);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .entry {
    display: flex;
    align-items: center;
    gap: 6px;
    font: inherit;
    color: var(--text);
    text-align: left;
    padding: 3px 6px;
    background: none;
    border: 0;
    cursor: pointer;
    text-shadow: 1px 1px 0 #000;
  }
  .entry:hover {
    color: var(--text-bright);
    background: rgba(255, 255, 255, 0.05);
  }
  .entry.on {
    color: var(--gold);
    background: var(--select);
  }
  .here {
    margin-left: auto;
    color: var(--green);
    font-size: 8px;
  }
  .none {
    padding: 6px 4px;
  }
  .pane {
    min-height: 0;
    min-width: 0;
    overflow-y: auto;
    padding-right: 4px;
  }
  /* Global: the map sections are their own components. */
  .pane :global(h4) {
    margin: 6px 0 0;
    color: var(--text-bright);
    font-size: var(--fs);
  }
  .pane > :global(h4:first-child) {
    margin-top: 0;
  }
  .lbl {
    width: 100px;
  }
  .note {
    margin: 0;
    font: var(--fs-small) / 10px var(--font-small);
  }
</style>
