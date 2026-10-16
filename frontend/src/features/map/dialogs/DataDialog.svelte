<script lang="ts">
  import { DialogService, errorMessage } from "../../../lib/api";
  import { toast } from "../../../lib/state/app.svelte";
  import { MapService, type MapDataConfig } from "../../../lib/map/data/api";
  import { loadMapData, map } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";
  import Select from "../../../lib/ui/Select.svelte";

  const data = map.data;
  let config = $state<MapDataConfig>(data?.config ? { ...data.config } : { itemsMode: "auto", otbPath: "", xmlPath: "", materialsPath: "", serverDir: "" });
  let busy = $state<string | null>(null);
  let warnings = $state<string[]>([]);

  async function file(key: "otbPath" | "xmlPath" | "materialsPath", title: string, pattern: string) {
    const paths = await DialogService.OpenFiles("mapData", title, [{ name: pattern, pattern }], false);
    if (paths?.length) config[key] = paths[0];
  }

  async function detect() {
    const dir = await DialogService.PickDirectory("mapData", "Server or data folder");
    if (!dir) return;
    const found = await MapService.DetectData(dir);
    config.otbPath = found.otbPath || config.otbPath;
    config.xmlPath = found.xmlPath || config.xmlPath;
    config.serverDir = found.serverDir || dir;
    toast(found.xmlPath ? `Found ${[found.otbPath && "items.otb", found.xmlPath && "items.xml"].filter(Boolean).join(" and ")}.` : "No items.xml found there; the folder is still used for monsters and NPCs.");
  }

  async function serverDir() {
    const dir = await DialogService.PickDirectory("mapData", "Server data folder (monsters and NPCs)");
    if (dir) config.serverDir = dir;
  }

  async function apply() {
    busy = "Loading items";
    try {
      const info = await MapService.SetData($state.snapshot(config));
      warnings = info.warnings ?? [];
      await loadMapData(true);
      toast(`${info.items.toLocaleString()} items, ${info.brushes} brushes.`, "success");
      if (!warnings.length) map.dialog = null;
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      busy = null;
    }
  }

  async function importRME() {
    const dir = await DialogService.PickDirectory("rmeData", "Remere's Map Editor data folder (with materials.xml, or the data folder with versions)");
    if (!dir) return;
    busy = "Importing brushes";
    try {
      const imported = await MapService.ImportRME(dir);
      warnings = imported.warnings ?? [];
      await loadMapData(true);
      toast(`Brushes: ${imported.data.brushes}, tilesets: ${imported.data.tilesets}, creatures: ${imported.data.creatures}.`, "success");
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      busy = null;
    }
  }

  async function importJSON() {
    const paths = await DialogService.OpenFiles("materials", "Brush file", [{ name: "Brushes (*.json)", pattern: "*.json" }], false);
    if (!paths?.length) return;
    try {
      await MapService.ImportMaterials(paths[0]);
      await loadMapData(true);
      toast("Brushes added.", "success");
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  async function exportJSON() {
    const path = await DialogService.SaveFile("materials", "Save brushes", "brushes.json", [{ name: "Brushes (*.json)", pattern: "*.json" }]);
    if (!path) return;
    try {
      await MapService.ExportMaterials(path);
      toast("Brushes saved.", "success");
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  async function reload() {
    busy = "Reloading";
    try {
      await MapService.Reload();
      await loadMapData(true);
      toast("Items reloaded from the client.", "success");
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      busy = null;
    }
  }
</script>

<Dialog title="Items and brushes" width={560} onclose={() => (map.dialog = null)}>
  <div class="col form">
    <div class="sect">
      <div class="h">Items</div>
      <p class="t-label hint">Server ids are translated to the client with items.otb. Client ids are used as they are, without items.otb. Names and types come from items.xml.</p>
      <div class="row">
        <span class="t-label lbl">Item ids</span>
        <Select grow value={config.itemsMode} options={[{ value: "auto", label: "Automatic: items.otb when set" }, { value: "otb", label: "Server ids (items.otb)" }, { value: "client", label: "Client ids (no items.otb)" }]} onchange={(value) => (config.itemsMode = value)} />
      </div>
      <div class="row"><span class="t-label lbl">items.otb</span><input class="t-input grow" bind:value={config.otbPath} placeholder="not used" /><button class="t-btn" onclick={() => file("otbPath", "items.otb", "*.otb")}>…</button></div>
      <div class="row"><span class="t-label lbl">items.xml</span><input class="t-input grow" bind:value={config.xmlPath} placeholder="names from the client" /><button class="t-btn" onclick={() => file("xmlPath", "items.xml", "*.xml")}>…</button></div>
      <div class="row"><span class="t-label lbl">Server data</span><input class="t-input grow" bind:value={config.serverDir} placeholder="monsters and NPCs" /><button class="t-btn" onclick={serverDir}>…</button></div>
      <div class="row">
        <button class="t-btn" onclick={detect}>Find in a server folder…</button>
        <button class="t-btn" onclick={reload} title="After editing objects">Reload from client</button>
      </div>
    </div>
    <div class="sect">
      <div class="h">Brushes</div>
      <div class="row"><span class="t-label lbl">Brush file</span><input class="t-input grow" bind:value={config.materialsPath} placeholder={data?.materials || "default for this client"} /><button class="t-btn" onclick={() => file("materialsPath", "Brush file", "*.json")}>…</button></div>
      <p class="t-label hint">{data?.brushes ?? 0} brushes, {data?.tilesets ?? 0} tilesets, {data?.creatures ?? 0} creatures. Brushes are kept in the editor's own format (JSON), shared by all maps of the client.</p>
      <div class="row wrap">
        <button class="t-btn primary" onclick={importRME}>Import from Remere's Map Editor…</button>
        <button class="t-btn" onclick={importJSON}>Add from file…</button>
        <button class="t-btn" onclick={exportJSON}>Save to file…</button>
      </div>
    </div>
    {#if warnings.length}
      <div class="warn t-panel">
        {#each warnings.slice(0, 60) as warning}<div>{warning}</div>{/each}
        {#if warnings.length > 60}<div class="t-label">…and {warnings.length - 60} more</div>{/if}
      </div>
    {/if}
  </div>
  {#snippet footer()}
    {#if busy}<span class="busy">{busy}…</span>{/if}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!!busy} onclick={apply}>Apply</button>
    <button class="t-btn" onclick={() => (map.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .form {
    gap: 10px;
  }
  .sect {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .h {
    color: var(--gold);
  }
  .lbl {
    width: 86px;
    flex: none;
  }
  .hint {
    margin: 0;
    font-size: 12px;
  }
  .wrap {
    flex-wrap: wrap;
  }
  .warn {
    max-height: 140px;
    overflow-y: auto;
    padding: 4px 6px;
    font-size: 12px;
    color: #e9b46a;
  }
  .busy {
    color: var(--gold);
  }
  .t-btn:not(.primary) {
    min-width: 0;
  }
</style>
