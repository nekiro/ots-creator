<script lang="ts">
  import { onMount } from "svelte";
  import { DialogService, Format, ProjectService, type Features, type Version } from "../../lib/api";
  import { applyDraft, app, run, toast } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import FeaturesEditor from "./FeaturesEditor.svelte";

  const info = app.project!.info;
  const fromAssets = info.format === Format.FormatAssets;
  let format = $state<Format>(fromAssets ? Format.FormatAssets : Format.FormatDat);
  const toAssets = $derived(format === Format.FormatAssets);
  let versions = $state<Version[]>([]);
  let versionKey = $state("");
  let features = $state<Features>({ ...info.features });
  // An asset folder is the output itself; dat/spr go into a folder.
  let dir = $state(fromAssets ? info.datPath : info.datPath ? info.datPath.replace(/[\\/][^\\/]*$/, "") : "");
  let name = $state(!fromAssets && info.datPath ? info.datPath.replace(/^.*[\\/]/, "").replace(/\.dat$/i, "") : "Tibia");
  let writeOtfi = $state(true);
  let warnings = $state<string[]>([]);

  const keyOf = (v: Version) => `${v.value}|${v.name}|${v.datSignature}|${v.sprSignature}`;
  const version = $derived(versions.find((v) => keyOf(v) === versionKey) ?? null);
  const sep = $derived(dir.includes("\\") ? "\\" : "/");

  onMount(async () => {
    versions = ((await ProjectService.Versions()) ?? []).slice().reverse();
    // Assets have no dat signature; offer the newest dat version.
    versionKey = fromAssets ? (versions[0] ? keyOf(versions[0]) : "") : keyOf(info.version);
  });

  $effect(() => {
    const v = toAssets ? info.version : version;
    if (!v) return;
    ProjectService.Warnings(format, v).then((w) => (warnings = w ?? []));
  });

  async function browse() {
    const d = await DialogService.PickDirectory("Output folder");
    if (d) dir = d;
  }

  async function compileAssets() {
    if (app.dirty) await applyDraft();
    // The status bar shows the compile while it runs.
    app.dialog = null;
    const ok = await run("Compiling", async () => {
      await ProjectService.CompileAs({ format, datPath: dir, sprPath: "", version: info.version, features: info.features, writeOtfi: false });
      return true;
    });
    if (ok) {
      toast(`Compiled assets to ${dir}`, "success");
    }
  }

  async function compile() {
    if (toAssets) return compileAssets();
    if (!version || !dir || !name) return;
    if (app.dirty) await applyDraft();
    const base = dir + sep + name;
    app.dialog = null;
    const ok = await run("Compiling", async () => {
      await ProjectService.CompileAs({ format, datPath: base + ".dat", sprPath: base + ".spr", version, features, writeOtfi });
      return true;
    });
    if (ok) {
      toast(`Compiled ${version.name} to ${base}.dat/.spr`, "success");
    }
  }
</script>

<Dialog title="Compile As" width={520} onclose={() => (app.dialog = null)}>
  <div class="col">
    <label class="row">
      <span class="t-label lbl">Format</span>
      <Select
        grow
        value={format}
        options={[
          { value: Format.FormatDat, label: "Tibia.dat + Tibia.spr" },
          { value: Format.FormatAssets, label: "Assets folder (Tibia 12+, protobuf)" },
        ]}
        onchange={(v) => (format = v as Format)}
      />
    </label>
    <label class="row">
      <span class="t-label lbl">Folder</span>
      <input class="t-input grow" bind:value={dir} placeholder="Choose an output folder" />
      <button class="t-btn" onclick={browse}>Browse…</button>
    </label>
    {#if toAssets}
      <div class="t-label">
        Writes the appearances file, sheets for new sprites and catalog-content.json; existing sheets are kept{fromAssets
          ? ""
          : ". A new folder only gets the sprites and appearances, not the map or static data"}.
      </div>
    {:else}
      <label class="row">
        <span class="t-label lbl">File name</span>
        <input class="t-input grow" bind:value={name} />
        <span class="t-label">.dat / .spr</span>
      </label>
      <label class="row">
        <span class="t-label lbl">Version</span>
        <Select grow value={versionKey} options={versions.map((v) => ({ value: keyOf(v), label: v.name }))} onchange={(v) => (versionKey = v)} />
      </label>
      {#if version}<FeaturesEditor bind:features version={version.value} sizeLocked />{/if}
      <label class="row"><input class="t-check" type="checkbox" bind:checked={writeOtfi} />Write OTClient feature file (.otfi)</label>
    {/if}
    {#if warnings.length}
      <div class="warn">
        <div>
          {warnings.length} object(s) use what {toAssets ? "assets" : version?.name} cannot store; flags are dropped{toAssets
            ? ", objects above 2x2 tiles stop the compilation"
            : ""}:
        </div>
        <div class="list t-panel">
          {#each warnings.slice(0, 200) as w}<div>{w}</div>{/each}
          {#if warnings.length > 200}<div>…</div>{/if}
        </div>
      </div>
    {/if}
  </div>
  {#snippet footer()}
    <button class="t-btn primary" disabled={!dir || (!toAssets && (!version || !name)) || !!app.busy} onclick={compile}>Compile</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .lbl {
    width: 60px;
  }
  .warn {
    color: var(--gold);
  }
  .list {
    margin-top: 4px;
    max-height: 120px;
    overflow: auto;
    padding: 4px 6px;
    color: var(--text);
    user-select: text;
  }
</style>
