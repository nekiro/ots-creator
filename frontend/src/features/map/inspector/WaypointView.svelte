<script lang="ts">
  import { MapService, type Waypoint } from "../../../lib/map/data/api";
  import { edit } from "../../../lib/map/state.svelte";
  import { removeWaypoint, renameWaypoint } from "../../../lib/map/editor/world";
  import PosField from "./PosField.svelte";

  let { waypoint, pending = false }: { waypoint: Waypoint; pending?: boolean } = $props();
</script>

<div class="m-form">
  {#if pending}
    <p class="t-label hint">Click the map to place “{waypoint.name}”.</p>
  {:else}
    <div class="m-field"><span class="m-lbl">Name</span><input class="t-input" value={waypoint.name} onchange={(event) => renameWaypoint(waypoint.name, event.currentTarget.value.trim())} /></div>
    <div class="m-field">
      <span class="m-lbl">Position</span>
      <PosField value={waypoint.pos} pickLabel="Click the new place of {waypoint.name}" onchange={(position) => edit((id) => MapService.SaveWaypoint(id, waypoint.name, { name: waypoint.name, pos: position }))} />
    </div>
    <p class="t-label hint">Scripts and NPCs refer to it by its name.</p>
    <div class="acts"><span class="grow"></span><button class="t-btn danger" onclick={() => removeWaypoint(waypoint.name)}>Remove</button></div>
  {/if}
</div>

<style>
  .hint {
    margin: 0;
    font-size: 12px;
  }
  .acts {
    display: flex;
  }
  .acts .t-btn {
    min-width: 0;
    padding: 0 8px;
  }
  .grow {
    flex: 1;
  }
</style>
