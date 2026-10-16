<script lang="ts">
  // Floors 0-15 at the edge of the map: the current floor, floors with
  // tiles, and the ground floor (7).
  import { mapCommands } from "../../lib/map/commands";
  import { map } from "../../lib/map/state.svelte";

  const used = $derived(map.active?.floors ?? 0);
  const floors = Array.from({ length: 16 }, (_, z) => z);

  function wheel(event: WheelEvent) {
    event.preventDefault();
    event.stopPropagation();
    mapCommands.floor(map.floor + (event.deltaY > 0 ? 1 : -1));
  }
</script>

<div class="ladder" role="toolbar" aria-label="Floors" onwheel={wheel}>
  {#each floors as z}
    <button
      class:cur={z === map.floor}
      class:used={(used >> z) & 1}
      class:ground={z === 7}
      title="Floor {z}{z === 7 ? ' (ground)' : z < 7 ? ' (above ground)' : ' (underground)'}{(used >> z) & 1 ? '' : ' · empty'} · PageUp/PageDown"
      onclick={() => mapCommands.floor(z)}>{z}</button
    >
  {/each}
</div>

<style>
  .ladder {
    position: absolute;
    right: 6px;
    top: 50%;
    transform: translateY(-50%);
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: 2px;
    background: rgba(20, 20, 20, 0.72);
    border: 1px solid #444;
  }
  button {
    width: 20px;
    height: 15px;
    padding: 0;
    background: none;
    border: 1px solid transparent;
    color: #6a6a6a;
    font: var(--fs-small) / 12px var(--font-small);
    cursor: pointer;
  }
  button.used {
    color: var(--text);
  }
  button.ground {
    border-bottom-color: #3c6b3c;
  }
  button:hover {
    background: rgba(255, 255, 255, 0.1);
  }
  button.cur {
    background: var(--gold);
    color: #000;
    border-color: var(--gold);
  }
</style>
