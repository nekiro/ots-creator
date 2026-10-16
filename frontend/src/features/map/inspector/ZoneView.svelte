<script lang="ts">
  import { MapService, type Zone } from "../../../lib/map/data/api";
  import { edit, isBrushActive, map, selectBrush } from "../../../lib/map/state.svelte";
  import { removeZone } from "../../../lib/map/editor/world";

  let { zone: z }: { zone: Zone } = $props();
  const sel = $derived({ kind: "zone", zone: z.id, label: `Zone ${z.name}` });
  const painting = $derived(isBrushActive(sel));
</script>

<div class="m-form">
  <div class="m-field"><span class="m-lbl">Name</span><input class="t-input" value={z.name} onchange={(event) => edit((id) => MapService.SaveZone(id, { id: z.id, name: event.currentTarget.value.trim() || z.name }))} /></div>
  <div class="m-field"><span class="m-lbl">Id</span><span>{z.id}</span></div>
  <p class="t-label hint">Zones mark tiles for scripts. They are saved in the zones file of the map.</p>
  <div class="acts">
    {#if painting}
      <button class="t-btn" onclick={() => (map.tool = "select")}>Stop painting</button>
    {:else}
      <button class="t-btn primary" onclick={() => selectBrush(sel)}>Paint tiles</button>
    {/if}
    <span class="grow"></span>
    <button class="t-btn danger" onclick={() => removeZone(z.id)}>Remove</button>
  </div>
</div>

<style>
  .hint {
    margin: 0;
    font-size: 12px;
  }
  .acts {
    display: flex;
    gap: 3px;
  }
  .acts .t-btn {
    min-width: 0;
    padding: 0 8px;
  }
  .grow {
    flex: 1;
  }
</style>
