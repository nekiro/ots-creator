<script lang="ts">
  import { MapService, type Zone } from "../../../lib/map/data/api";
  import { edit, isBrushActive, map, selectBrush } from "../../../lib/map/state.svelte";
  import { removeZone } from "../../../lib/map/editor/world";

  let { zone }: { zone: Zone } = $props();
  const brush = $derived({ kind: "zone", zone: zone.id, label: `Zone ${zone.name}` });
  const painting = $derived(isBrushActive(brush));
</script>

<div class="m-form">
  <div class="m-field"><span class="m-lbl">Name</span><input class="t-input" value={zone.name} onchange={(event) => edit((id) => MapService.SaveZone(id, { id: zone.id, name: event.currentTarget.value.trim() || zone.name }))} /></div>
  <div class="m-field"><span class="m-lbl">Id</span><span>{zone.id}</span></div>
  <p class="t-label hint">Zones mark tiles for scripts. They are saved in the zones file of the map.</p>
  <div class="acts">
    {#if painting}
      <button class="t-btn" onclick={() => (map.tool = "select")}>Stop painting</button>
    {:else}
      <button class="t-btn primary" onclick={() => selectBrush(brush)}>Paint tiles</button>
    {/if}
    <span class="grow"></span>
    <button class="t-btn danger" onclick={() => removeZone(zone.id)}>Remove</button>
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
