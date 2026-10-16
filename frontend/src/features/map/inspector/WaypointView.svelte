<script lang="ts">
  import { MapService, type Waypoint } from "../../../lib/map/api";
  import { edit } from "../../../lib/map/state.svelte";
  import { removeWaypoint, renameWaypoint } from "../../../lib/map/world";
  import PosField from "./PosField.svelte";

  let { waypoint: w, pending = false }: { waypoint: Waypoint; pending?: boolean } = $props();
</script>

<div class="m-form">
  {#if pending}
    <p class="t-label hint">Click the map to place “{w.name}”.</p>
  {:else}
    <div class="m-field"><span class="m-lbl">Name</span><input class="t-input" value={w.name} onchange={(event) => renameWaypoint(w.name, event.currentTarget.value.trim())} /></div>
    <div class="m-field">
      <span class="m-lbl">Position</span>
      <PosField value={w.pos} pickLabel="Click the new place of {w.name}" onchange={(p) => edit((id) => MapService.SaveWaypoint(id, w.name, { name: w.name, pos: p }))} />
    </div>
    <p class="t-label hint">Scripts and NPCs refer to it by its name.</p>
    <div class="acts"><span class="grow"></span><button class="t-btn danger" onclick={() => removeWaypoint(w.name)}>Remove</button></div>
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
