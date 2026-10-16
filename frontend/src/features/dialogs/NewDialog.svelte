<script lang="ts">
  import { onMount } from "svelte";
  import { ProjectService, type Features, type Version } from "../../lib/api";
  import { loadVersions } from "../../lib/versions";
  import { app, run, toast } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import FeaturesEditor from "./FeaturesEditor.svelte";

  let versions = $state<Version[]>([]);
  let index = $state(0);
  let features = $state<Features>({ extended: true, transparency: false, improvedAnimations: true, frameGroups: true, spriteSize: 32 });
  const version = $derived(versions[index]);

  onMount(async () => {
    versions = await loadVersions();
    const defaultIndex = versions.findIndex((value) => value.value === 1098);
    index = defaultIndex >= 0 ? defaultIndex : 0;
  });

  async function create() {
    if (!version) return;
    const created = await run("Creating", () => ProjectService.New(version, features));
    if (created?.open) {
      app.dialog = null;
      toast(`New ${version.name} client created. Use Compile As to save it.`, "success");
    }
  }
</script>

<Dialog title="New Client" width={440} onclose={() => (app.dialog = null)}>
  <div class="col">
    <label class="row">
      <span class="t-label">Version</span>
      <Select grow value={index} options={versions.map((value, i) => ({ value: i, label: value.name }))} onchange={(value) => (index = value)} />
    </label>
    {#if version}<FeaturesEditor bind:features version={version.value} />{/if}
  </div>
  {#snippet footer()}
    <button class="t-btn primary" disabled={!version} onclick={create}>Create</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>
