<script lang="ts">
  import { onMount } from "svelte";
  import { CompareService, DialogService, Format, ProjectService, errorMessage, type ClientFiles, type Features, type Version } from "../../lib/api";
  import { app, run, toast } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import FeaturesEditor from "./FeaturesEditor.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import { commands, openedNote } from "../../lib/commands";
  import { prefs, recentDir, shortPath } from "../../lib/state/prefs.svelte";
  import type { Recent } from "../../lib/api";
  import { OTB_FILTERS } from "../../lib/commands/shared";
  import { keyOfVersion, loadVersions } from "../../lib/versions";

  const recent = $derived((prefs.settings.recent ?? []).slice(0, 5));

  async function pickRecent(client: Recent) {
    if (client.format === Format.FormatAssets) return openRecent(client);
    await inspect(client.datPath);
    if (!files) return;
    features = { ...client.features };
    if (!isAssets && versions.some((value) => keyOfVersion(value) === keyOfVersion(client.version))) {
      versionKey = keyOfVersion(client.version);
      error = "";
    }
  }

  function openRecent(client: Recent) {
    if (other) {
      void openOther(client);
      return;
    }
    app.dialog = null;
    void commands.openRecent(client);
  }

  async function openOther(client: Recent) {
    const format = client.format === Format.FormatAssets ? Format.FormatAssets : Format.FormatDat;
    const request = { format, datPath: client.datPath, sprPath: client.sprPath, version: client.version, features: client.features, otbPath: null };
    if ((await run("Loading client", () => CompareService.Open(request)))?.open) close();
  }

  let versions = $state<Version[]>([]);
  let files = $state<ClientFiles | null>(null);
  let versionKey = $state("");
  let features = $state<Features>({ extended: false, transparency: false, improvedAnimations: false, frameGroups: false, spriteSize: 32 });
  let error = $state("");
  let otbPath = $state("");
  /** Set once the user picked or cleared the items.otb; until then the one
   * remembered for the client is used. */
  let otbChosen = $state(false);

  const isAssets = $derived(files?.format === Format.FormatAssets);
  const version = $derived(isAssets ? files!.detected : (versions.find((value) => keyOfVersion(value) === versionKey) ?? null));
  const hex = (value: number) => "0x" + value.toString(16).toUpperCase();

  onMount(async () => {
    versions = await loadVersions();
    const dropped = app.openPath;
    app.openPath = "";
    if (dropped) await inspectAndOpen(dropped);
  });

  async function browse() {
    const dir = await DialogService.PickDirectory(other ? "compareClient" : "openClient", "Open client folder");
    if (dir) await inspectAndOpen(dir);
  }

  async function inspectAndOpen(dir: string) {
    await inspect(dir);
    if (files?.detected && (isAssets || files.hasOtfi)) await open();
  }

  async function browseOtb() {
    const paths = await DialogService.OpenFiles("itemsOtb", "items.otb", OTB_FILTERS, false);
    if (paths?.length) {
      otbPath = paths[0];
      otbChosen = true;
    }
  }

  async function inspect(dir: string) {
    error = "";
    try {
      files = await ProjectService.Inspect(dir);
      features = { ...files.features };
      if (!otbChosen) otbPath = files.otbPath;
      if (files.detected) versionKey = keyOfVersion(files.detected);
      else error = `Unknown signatures (dat ${hex(files.datSignature)}, spr ${hex(files.sprSignature)}). Choose the version manually.`;
    } catch (err) {
      files = null;
      error = errorMessage(err);
    }
  }

  // Loads the second client of the compare window instead of the main one.
  const other = app.openOther;

  function close() {
    app.openOther = false;
    app.dialog = other ? "compare" : null;
  }

  async function open() {
    if (!files || !version) return;
    const chosen = files;
    // The compared client never syncs an items.otb.
    const request = { format: chosen.format, datPath: chosen.datPath, sprPath: chosen.sprPath, version: chosen.detected ? null : version, features, otbPath: other || !otbChosen ? null : otbPath };
    const opened = await run("Loading client", () => (other ? CompareService.Open(request) : ProjectService.Open(request)));
    if (opened?.open) {
      close();
      openedNote(opened);
      toast(`Loaded ${version.name}: ${opened.info.counts.items - 99} items, ${opened.info.counts.sprites} sprites.`, "success");
    }
  }
</script>

<Dialog title={other ? "Open Client to Compare" : "Open Client"} width={500} onclose={close}>
  <div class="col">
    {#if !files}
      <div class="empty t-panel">
        <Icon name="open" size={22} />
        <span
          >Choose the client folder with <strong>Tibia.dat</strong> and <strong>Tibia.spr</strong> (or an OTClient <strong>.otfi</strong>), or the
          <strong>assets</strong> folder of Tibia 12+.</span
        >
        <button class="t-btn primary" onclick={browse}>Browse…</button>
      </div>
      {#if recent.length}
        <div class="t-label">Recent clients</div>
        <div class="recent t-panel">
          {#each recent as client (client.datPath)}
            <button class="ritem" title={`${client.datPath}
Click to choose features, double click to open as before`} onclick={() => pickRecent(client)} ondblclick={() => openRecent(client)}>
              <span class="rver">{client.version.name}</span>
              <span class="rdir">{shortPath(recentDir(client), 56)}</span>
            </button>
          {/each}
        </div>
      {/if}
    {/if}
    {#if files && isAssets}
      <div class="paths t-panel">
        <span class="t-label">assets</span><span class="path" title={files.datPath}>{files.datPath}</span>
        <span class="note">Protobuf appearances and sprite sheets, client {files.detected?.name}</span>
      </div>
    {:else if files}
      <div class="paths t-panel">
        <span class="t-label">dat</span><span class="path" title={files.datPath}>{files.datPath}</span>
        <span class="t-label">spr</span><span class="path" title={files.sprPath}>{files.sprPath}</span>
        {#if files.hasOtfi}
          <span class="note">Features loaded from .otfi</span>
        {:else}
          <span class="note dim">No .otfi: the dat and spr do not store these settings, so the defaults of the version are set. Check them below (Transparency is off by default).</span>
        {/if}
      </div>
      <label class="row">
        <span class="t-label">Version</span>
        <Select grow value={versionKey} options={versions.map((value) => ({ value: keyOfVersion(value), label: value.name }))} onchange={(value) => (versionKey = value)} />
        {#if files.detected}<span class="ok">detected</span>{/if}
      </label>
      <div class="t-label">Features</div>
      <FeaturesEditor bind:features version={version?.value ?? 0} />
    {/if}
    {#if !other}
      <div class="row" title="Optional. Servers that map server ids to client ids with an items.otb (TFS, OTServ) can keep it in sync: compiling the client then updates it too.">
        <span class="t-label">items.otb</span>
        <input class="t-input grow" readonly value={otbPath} placeholder="None (optional)" />
        {#if otbPath}<button class="t-btn" title="Do not use an items.otb" onclick={() => ((otbPath = ""), (otbChosen = true))}>Clear</button>{/if}
        <button class="t-btn" onclick={browseOtb}>Browse…</button>
      </div>
    {/if}
    {#if error}<div class="err">{error}</div>{/if}
  </div>
  {#snippet footer()}
    {#if files}<button class="t-btn" onclick={browse}>Browse…</button>{/if}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!files || !version || !!app.busy} onclick={open}>Open</button>
    <button class="t-btn" onclick={close}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    padding: 18px 16px;
    text-align: center;
    color: var(--text);
  }
  .empty strong {
    color: var(--text-bright);
  }
  .recent {
    padding: 2px;
  }
  .ritem {
    width: 100%;
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
  .ritem:hover {
    background: rgba(255, 255, 255, 0.06);
    color: var(--text-bright);
  }
  .rver {
    color: var(--gold);
    min-width: 34px;
  }
  .rdir {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .paths {
    padding: 6px 8px;
    user-select: text;
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2px 8px;
  }
  .path {
    color: var(--text-bright);
    overflow-wrap: anywhere;
  }
  .note {
    grid-column: 1 / -1;
    color: var(--green);
  }
  .note.dim {
    color: var(--text-dim);
  }
  .ok {
    color: var(--green);
  }
  .err {
    color: #ff9c9c;
  }
</style>
