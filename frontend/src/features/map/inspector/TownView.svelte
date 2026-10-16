<script lang="ts">
  import type { Town } from "../../../lib/map/data/api";
  import { mapCommands } from "../../../lib/map/view/commands";
  import { map } from "../../../lib/map/state.svelte";
  import { addHouse, houses, removeTown, saveTown } from "../../../lib/map/editor/world";
  import Icon from "../../../lib/ui/Icon.svelte";
  import PosField from "./PosField.svelte";

  let { town }: { town: Town } = $props();
  const list = $derived(map.active ? houses().filter((house) => house.townId === town.id) : []);
  const tiles = $derived(list.reduce((sum, house) => sum + house.size, 0));
</script>

<div class="m-form">
  <div class="m-field"><span class="m-lbl">Name</span><input class="t-input" value={town.name} onchange={(event) => saveTown({ ...town, name: event.currentTarget.value.trim() || town.name })} /></div>
  <div class="m-field"><span class="m-lbl">Temple</span><PosField value={town.temple} pickLabel="Click the temple of {town.name}" onchange={(position) => saveTown({ ...town, temple: position })} /></div>
  <div class="m-field"><span class="m-lbl">Id</span><span>{town.id}</span></div>

  <div class="m-sect">Houses <span class="grow"></span>{list.length} · {tiles} sqm</div>
  <div class="t-panel list">
    {#each list as house (house.id)}
      <button class="m-row" onclick={() => mapCommands.focus({ kind: "house", id: house.id })}>
        <span class="ico"><Icon name="home" size={12} /></span><span class="m-name">{house.name}</span><span class="m-meta">{house.size} sqm</span>
      </button>
    {:else}
      <div class="m-empty">No houses yet.</div>
    {/each}
  </div>
  <div class="acts">
    <button class="t-btn primary" onclick={() => addHouse(town.id)}>New house</button>
    <span class="grow"></span>
    <button class="t-btn danger" onclick={() => removeTown(town)}>Remove town</button>
  </div>
</div>

<style>
  .list {
    max-height: 240px;
    overflow-y: auto;
    padding: 2px;
  }
  .ico {
    width: 20px;
    display: grid;
    place-items: center;
    color: var(--text-dim);
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
