<script lang="ts">
  // Zoom buttons in the corner of the map.
  import { mapCommands } from "../../lib/map/view/commands";
  import { map } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";

  const pct = $derived(Math.round(map.cam.zoom * 100));
</script>

<div class="zoom" role="toolbar" aria-label="Zoom">
  <button title="Zoom in (+, wheel)" onclick={() => mapCommands.zoom(1)}><Icon name="plus" /></button>
  <button class="pct" title="Normal zoom (Ctrl+0)" onclick={mapCommands.zoomReset}>{pct}%</button>
  <button title="Zoom out (-, wheel)" onclick={() => mapCommands.zoom(-1)}><Icon name="zoomout" /></button>
</div>

<style>
  .zoom {
    position: absolute;
    right: 6px;
    bottom: 10px;
    display: flex;
    flex-direction: column;
    background: rgba(20, 20, 20, 0.8);
    border: 1px solid #444;
  }
  button {
    width: 30px;
    height: 22px;
    display: grid;
    place-items: center;
    background: none;
    border: 0;
    color: var(--text);
    cursor: pointer;
    padding: 0;
  }
  button:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
  }
  .pct {
    font: var(--fs-small) / 10px var(--font-small);
    color: var(--text-dim);
    border-top: 1px solid #333;
    border-bottom: 1px solid #333;
  }
</style>
