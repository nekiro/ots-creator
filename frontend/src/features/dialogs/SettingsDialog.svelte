<script lang="ts">
  import { app, toast } from "../../lib/state/app.svelte";
  import { IMAGE_FORMATS, LIST_COLUMNS, prefs, removeRecent, savePrefs } from "../../lib/state/prefs.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Select from "../../lib/ui/Select.svelte";

  const settings = $derived(prefs.settings);
  const recent = $derived(settings.recent?.length ?? 0);
  const mcpURL = $derived(`http://127.0.0.1:${settings.mcpPort}/mcp`);

  function copyMCPCommand() {
    void navigator.clipboard
      ?.writeText(`claude mcp add --transport http ots-creator ${mcpURL}`)
      .then(() => toast("Copied the command that connects Claude Code."));
  }
</script>

<Dialog title="Settings" width={420} onclose={() => (app.dialog = null)}>
  <div class="col">
    <h4>Startup</h4>
    <label class="row"><input class="t-check" type="checkbox" checked={settings.checkUpdates} onchange={(event) => savePrefs({ checkUpdates: event.currentTarget.checked })} />Check for updates on startup</label>
    <label class="row"><input class="t-check" type="checkbox" checked={settings.reopenLast} onchange={(event) => savePrefs({ reopenLast: event.currentTarget.checked })} />Reopen the last client on startup</label>

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

    <h4>AI agents</h4>
    <label class="row" title="A local MCP server: agents such as Claude Code draw on the open maps, and you see it here">
      <input class="t-check" type="checkbox" checked={settings.mcpServer} onchange={(event) => savePrefs({ mcpServer: event.currentTarget.checked })} />Let AI agents edit maps (MCP server)
    </label>
    <div class="row">
      <NumberField label="Port" value={settings.mcpPort} min={1024} max={65535} width={56} onchange={(value) => savePrefs({ mcpPort: value })} />
      <span class="t-label grow">{mcpURL}</span>
      <button class="t-btn" onclick={copyMCPCommand}>Copy command</button>
    </div>
    <p class="t-label note">Only programs on this computer can connect. Copy command gives the line that adds the server to Claude Code.</p>

    <h4>Recent clients</h4>
    <div class="row">
      <span class="t-label grow">{recent ? `${recent} remembered` : "None yet"}</span>
      <button class="t-btn danger" disabled={!recent} onclick={() => removeRecent(null)}>Clear list</button>
    </div>
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  h4 {
    margin: 6px 0 0;
    color: var(--text-bright);
    font-size: var(--fs);
  }
  h4:first-child {
    margin-top: 0;
  }
  .lbl {
    width: 110px;
  }
  .note {
    margin: 0;
    font: var(--fs-small) / 10px var(--font-small);
  }
</style>
