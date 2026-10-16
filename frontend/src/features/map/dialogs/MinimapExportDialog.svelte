<script lang="ts">
  import { DialogService, errorMessage } from "../../../lib/api";
  import { MapService } from "../../../lib/map/api";
  import { map } from "../../../lib/map/state.svelte";
  import { toast } from "../../../lib/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";

  let floors = $state<boolean[]>(Array.from({ length: 16 }, (_, z) => z === map.floor));
  let busy = $state(false);

  async function run() {
    const dir = await DialogService.PickDirectory("exportMinimap", "Folder for the minimap images");
    if (!dir || map.activeId === null) return;
    busy = true;
    try {
      const files = await MapService.ExportMinimap(map.activeId, dir, floors.map((on, z) => (on ? z : -1)).filter((z) => z >= 0));
      toast(`Wrote ${files?.length ?? 0} images.`, "success");
      map.dialog = null;
    } catch (err) {
      toast(errorMessage(err), "error");
    } finally {
      busy = false;
    }
  }
</script>

<Dialog title="Export minimap" width={400} onclose={() => (map.dialog = null)}>
  <div class="col">
    <p class="t-label">One PNG per floor, one pixel per tile, in the minimap colors of the items.</p>
    <div class="floors">
      {#each floors as _, z}
        <label class="row"><input class="t-check" type="checkbox" bind:checked={floors[z]} />Floor {z}</label>
      {/each}
    </div>
    <div class="row">
      <button class="t-btn" onclick={() => (floors = floors.map(() => true))}>All</button>
      <button class="t-btn" onclick={() => (floors = floors.map((_, z) => z <= 7))}>Above ground</button>
      <button class="t-btn" onclick={() => (floors = floors.map(() => false))}>None</button>
    </div>
  </div>
  {#snippet footer()}
    <button class="t-btn primary" disabled={busy || !floors.some(Boolean)} onclick={run}>Export…</button>
    <button class="t-btn" onclick={() => (map.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  p {
    margin: 0;
  }
  .floors {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 3px;
  }
</style>
