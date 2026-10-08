<script lang="ts">
  import { onMount } from "svelte";
  import { DialogService, ProjectService, errorMessage, type ClientFiles, type Features, type Version } from "../../lib/api";
  import { app, run, toast } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import FeaturesEditor from "./FeaturesEditor.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import { commands } from "../../lib/commands";
  import { prefs, recentDir, shortPath } from "../../lib/prefs.svelte";
  import type { Recent } from "../../lib/api";

  const recent = $derived((prefs.settings.recent ?? []).slice(0, 5));

  function openRecent(r: Recent) {
    app.dialog = null;
    void commands.openRecent(r);
  }

  let versions = $state<Version[]>([]);
  let files = $state<ClientFiles | null>(null);
  let versionKey = $state("");
  let features = $state<Features>({ extended: false, transparency: false, improvedAnimations: false, frameGroups: false, spriteSize: 32 });
  let error = $state("");

  const keyOf = (v: Version) => `${v.value}|${v.name}|${v.datSignature}|${v.sprSignature}`;
  const version = $derived(versions.find((v) => keyOf(v) === versionKey) ?? null);
  const hex = (n: number) => "0x" + n.toString(16).toUpperCase();

  onMount(async () => {
    versions = ((await ProjectService.Versions()) ?? []).slice().reverse();
    const dropped = app.openPath;
    app.openPath = "";
    if (dropped) await inspect(dropped);
  });

  async function browse() {
    const dir = await DialogService.PickDirectory("Open client folder");
    if (!dir) return;
    await inspect(dir);
  }

  async function inspect(dir: string) {
    error = "";
    try {
      files = await ProjectService.Inspect(dir);
      features = { ...files.features };
      if (files.detected) versionKey = keyOf(files.detected);
      else error = `Unknown signatures (dat ${hex(files.datSignature)}, spr ${hex(files.sprSignature)}). Choose the version manually.`;
    } catch (e) {
      files = null;
      error = errorMessage(e);
    }
  }

  async function open() {
    if (!files || !version) return;
    const f = files;
    const st = await run("Loading client", () =>
      ProjectService.Open({ datPath: f.datPath, sprPath: f.sprPath, version: f.detected ? null : version, features }),
    );
    if (st?.open) {
      app.dialog = null;
      toast(`Loaded ${version.name}: ${st.info.counts.items - 99} items, ${st.info.counts.sprites} sprites.`, "success");
    }
  }
</script>

<Dialog title="Open Client" width={500} onclose={() => (app.dialog = null)}>
  <div class="col">
    {#if !files}
      <div class="empty t-panel">
        <Icon name="open" size={22} />
        <span>Choose the client folder with <strong>Tibia.dat</strong> and <strong>Tibia.spr</strong> (or an OTClient <strong>.otfi</strong>).</span>
        <button class="t-btn primary" onclick={browse}>Browse…</button>
      </div>
      {#if recent.length}
        <div class="t-label">Recent clients</div>
        <div class="recent t-panel">
          {#each recent as r (r.datPath)}
            <button class="ritem" title={r.datPath} onclick={() => openRecent(r)}>
              <span class="rver">{r.version.name}</span>
              <span class="rdir">{shortPath(recentDir(r), 56)}</span>
            </button>
          {/each}
        </div>
      {/if}
    {/if}
    {#if files}
      <div class="paths t-panel">
        <span class="t-label">dat</span><span class="path" title={files.datPath}>{files.datPath}</span>
        <span class="t-label">spr</span><span class="path" title={files.sprPath}>{files.sprPath}</span>
        {#if files.hasOtfi}<span class="note">Features loaded from .otfi</span>{/if}
      </div>
      <label class="row">
        <span class="t-label">Version</span>
        <Select grow value={versionKey} options={versions.map((v) => ({ value: keyOf(v), label: v.name }))} onchange={(v) => (versionKey = v)} />
        {#if files.detected}<span class="ok">detected</span>{/if}
      </label>
      {#if version}<FeaturesEditor bind:features version={version.value} />{/if}
    {/if}
    {#if error}<div class="err">{error}</div>{/if}
  </div>
  {#snippet footer()}
    {#if files}<button class="t-btn" onclick={browse}>Browse…</button>{/if}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!files || !version || !!app.busy} onclick={open}>Open</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
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
  .ok {
    color: var(--green);
  }
  .err {
    color: #ff9c9c;
  }
</style>
