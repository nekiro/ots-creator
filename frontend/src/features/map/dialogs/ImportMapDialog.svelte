<script lang="ts">
  import { DialogService } from "../../../lib/api";
  import { MapService } from "../../../lib/map/data/api";
  import { edit, map } from "../../../lib/map/state.svelte";
  import { toast } from "../../../lib/state/app.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";
  import NumberField from "../../../lib/ui/NumberField.svelte";
  import Select from "../../../lib/ui/Select.svelte";

  let path = $state("");
  let dx = $state(0);
  let dy = $state(0);
  let dz = $state(0);
  let houses = $state("merge");
  let replace = $state(false);
  let towns = $state(true);
  let busy = $state(false);

  async function browse() {
    const paths = await DialogService.OpenFiles("importMap", "Map to import", [{ name: "OTBM maps (*.otbm)", pattern: "*.otbm" }], false);
    if (paths?.length) path = paths[0];
  }

  async function run() {
    busy = true;
    const change = await edit((id) => MapService.ImportMap(id, path, { dx, dy, dz, houses, replace, towns }));
    busy = false;
    if (change) {
      toast(`Imported ${change.count.toLocaleString()} tiles.`, "success");
      map.dialog = null;
    }
  }
</script>

<Dialog title="Import map" width={460} onclose={() => (map.dialog = null)}>
  <div class="col form">
    <div class="row"><input class="t-input grow" bind:value={path} placeholder="Map file" /><button class="t-btn" onclick={browse}>Browse…</button></div>
    <div class="row">
      <span class="t-label lbl">Offset</span>
      <NumberField label="x" value={dx} min={-65535} max={65535} width={56} onchange={(value) => (dx = value)} />
      <NumberField label="y" value={dy} min={-65535} max={65535} width={56} onchange={(value) => (dy = value)} />
      <NumberField label="z" value={dz} min={-15} max={15} width={32} onchange={(value) => (dz = value)} />
    </div>
    <div class="row">
      <span class="t-label lbl">Houses</span>
      <Select grow value={houses} options={[{ value: "merge", label: "Keep their ids" }, { value: "renumber", label: "Give them new ids" }, { value: "skip", label: "Leave houses out" }]} onchange={(value) => (houses = value)} />
    </div>
    <label class="row"><input class="t-check" type="checkbox" bind:checked={replace} />Replace existing tiles (otherwise items are added)</label>
    <label class="row"><input class="t-check" type="checkbox" bind:checked={towns} />Add missing towns and waypoints</label>
    <p class="t-label hint">Spawns and creatures come along. One undo step reverts the whole import.</p>
  </div>
  {#snippet footer()}
    <button class="t-btn primary" disabled={!path || busy} onclick={run}>Import</button>
    <button class="t-btn" onclick={() => (map.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .form {
    gap: 8px;
  }
  .lbl {
    width: 60px;
    flex: none;
  }
  .hint {
    margin: 0;
    font-size: 12px;
  }
</style>
