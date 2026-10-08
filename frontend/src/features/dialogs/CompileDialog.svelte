<script lang="ts">
  import { onMount } from "svelte";
  import { DialogService, ProjectService, type Features, type Version } from "../../lib/api";
  import { applyDraft, app, run, toast } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import FeaturesEditor from "./FeaturesEditor.svelte";

  const info = app.project!.info;
  let versions = $state<Version[]>([]);
  let versionKey = $state("");
  let features = $state<Features>({ ...info.features });
  let dir = $state(info.datPath ? info.datPath.replace(/[\\/][^\\/]*$/, "") : "");
  let name = $state(info.datPath ? info.datPath.replace(/^.*[\\/]/, "").replace(/\.dat$/i, "") : "Tibia");
  let writeOtfi = $state(true);
  let warnings = $state<string[]>([]);

  const keyOf = (v: Version) => `${v.value}|${v.name}|${v.datSignature}|${v.sprSignature}`;
  const version = $derived(versions.find((v) => keyOf(v) === versionKey) ?? null);
  const sep = $derived(dir.includes("\\") ? "\\" : "/");

  onMount(async () => {
    versions = ((await ProjectService.Versions()) ?? []).slice().reverse();
    versionKey = keyOf(info.version);
  });

  $effect(() => {
    const v = version;
    if (!v) return;
    ProjectService.Warnings(v).then((w) => (warnings = w ?? []));
  });

  async function browse() {
    const d = await DialogService.PickDirectory("Output folder");
    if (d) dir = d;
  }

  async function compile() {
    if (!version || !dir || !name) return;
    if (app.dirty) await applyDraft();
    const base = dir + sep + name;
    const ok = await run("Compiling", async () => {
      await ProjectService.CompileAs({ datPath: base + ".dat", sprPath: base + ".spr", version, features, writeOtfi });
      return true;
    });
    if (ok) {
      app.dialog = null;
      toast(`Compiled ${version.name} to ${base}.dat/.spr`, "success");
    }
  }
</script>

<Dialog title="Compile As" width={520} onclose={() => (app.dialog = null)}>
  <div class="col">
    <label class="row">
      <span class="t-label lbl">Folder</span>
      <input class="t-input grow" bind:value={dir} placeholder="Choose an output folder" />
      <button class="t-btn" onclick={browse}>Browse…</button>
    </label>
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
    {#if warnings.length}
      <div class="warn">
        <div>{warnings.length} object(s) use flags that {version?.name} cannot store; they will be dropped:</div>
        <div class="list t-panel">
          {#each warnings.slice(0, 200) as w}<div>{w}</div>{/each}
          {#if warnings.length > 200}<div>…</div>{/if}
        </div>
      </div>
    {/if}
  </div>
  {#snippet footer()}
    <button class="t-btn primary" disabled={!version || !dir || !name || !!app.busy} onclick={compile}>Compile</button>
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
