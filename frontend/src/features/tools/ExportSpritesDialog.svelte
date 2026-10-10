<script lang="ts">
  // Exports every sprite of the client: one file per sprite, or sprite
  // sheets that keep the ids in order (left to right, top to bottom).
  // Closing the window while the export runs leaves it running.
  import { DialogService } from "../../lib/api";
  import { commands } from "../../lib/commands";
  import { IMAGE_FORMATS, prefs } from "../../lib/prefs.svelte";
  import { app, toast } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";
  import ProgressBar from "../../lib/ui/ProgressBar.svelte";
  import Select from "../../lib/ui/Select.svelte";

  const total = app.project?.info.counts.sprites ?? 0;
  const size = app.project?.info.features.spriteSize ?? 32;

  let sheets = $state(false);
  let format = $state(prefs.settings.exportFormat);
  let skipEmpty = $state(true);
  let columns = $state(16);
  let rows = $state(16);
  let transparent = $state(prefs.settings.sheetBackground === "transparent");

  const running = $derived(app.stoppable);
  const percent = $derived(app.progress && app.progress.total > 0 ? (app.progress.done / app.progress.total) * 100 : 0);
  const perSheet = $derived(columns * rows);
  const sheetCount = $derived(Math.ceil(total / Math.max(1, perSheet)));

  async function exportAll() {
    const dir = await DialogService.PickDirectory("exportAllSprites", "Export all sprites to");
    if (dir) await commands.exportAllSpritesTo(dir, { sheets, format, skipEmpty, columns, rows, transparent });
  }

  function close() {
    app.dialog = null;
    if (running && !app.stopping) toast("The export keeps running in the background. Stop it from the status bar.");
  }
</script>

<Dialog title="Export All Sprites" width={400} onclose={close}>
  <div class="col" class:off={running}>
    <div class="t-tabs">
      <button class="t-tab" class:on={!sheets} onclick={() => (sheets = false)}>Single files</button>
      <button class="t-tab" class:on={sheets} onclick={() => (sheets = true)}>Sprite sheets</button>
    </div>
    <div class="row">
      <span class="t-label label">Format</span>
      <Select grow value={format} options={IMAGE_FORMATS} onchange={(v) => (format = v)} />
    </div>
    {#if sheets}
      <div class="row">
        <span class="t-label label">Sprites</span>
        <NumberField value={columns} min={1} max={64} width={50} title="Sprites per row" onchange={(v) => (columns = v)} />
        <span class="t-label">×</span>
        <NumberField value={rows} min={1} max={64} width={50} title="Rows per sheet" onchange={(v) => (rows = v)} />
        <span class="t-label">= {columns * size}×{rows * size} px</span>
      </div>
      <label class="row" title={format === "png" ? "" : "Only PNG keeps transparency"}
        ><input class="t-check" type="checkbox" bind:checked={transparent} disabled={format !== "png"} />Transparent background (otherwise magenta)</label
      >
      <p class="t-label">
        {sheetCount.toLocaleString()} sheet(s) named <code>sprites_first-last.{format}</code>. Ids stay in order, empty sprites stay as gaps.
      </p>
    {:else}
      <label class="row"><input class="t-check" type="checkbox" bind:checked={skipEmpty} />Skip empty sprites</label>
      <p class="t-label">{total.toLocaleString()} sprite(s) as <code>id.{format}</code>.</p>
    {/if}
  </div>
  {#snippet footer()}
    {#if running}
      <div class="grow center">
        <ProgressBar
          {percent}
          label={app.stopping ? "Stopping…" : app.progress ? `${app.progress.done.toLocaleString()} / ${app.progress.total.toLocaleString()}` : "Exporting…"}
        />
      </div>
      <button class="t-btn" title="Close this window, the export keeps running" onclick={close}>Hide</button>
      <button class="t-btn danger" disabled={app.stopping} onclick={commands.stopTask}>Stop</button>
    {:else}
      <span class="grow"></span>
      <button class="t-btn primary" disabled={!!app.busy || total === 0} onclick={exportAll}>Export…</button>
      <button class="t-btn" onclick={close}>Cancel</button>
    {/if}
  {/snippet}
</Dialog>

<style>
  .label {
    width: 52px;
  }
  p {
    margin: 0;
  }
  .center {
    align-self: center;
  }
  .off {
    opacity: 0.5;
    pointer-events: none;
  }
</style>
