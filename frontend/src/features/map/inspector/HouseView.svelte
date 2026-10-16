<script lang="ts">
  import type { HouseInfo } from "../../../lib/map/data/api";
  import { mapCommands } from "../../../lib/map/view/commands";
  import { isBrushActive, map, selectBrush } from "../../../lib/map/state.svelte";
  import { goHouse, paintHouse, removeHouse, saveHouse, selectHouseTiles, towns } from "../../../lib/map/editor/world";
  import NumberField from "../../../lib/ui/NumberField.svelte";
  import Select from "../../../lib/ui/Select.svelte";
  import PosField from "./PosField.svelte";

  let { house }: { house: HouseInfo } = $props();
  const townOptions = $derived([{ value: 0, label: "None" }, ...(map.active ? towns() : []).map((town) => ({ value: town.id, label: town.name }))]);
  const painting = $derived(isBrushActive({ kind: "house", house: house.id, label: "" }));
</script>

<div class="m-form">
  <div class="m-field"><span class="m-lbl">Name</span><input class="t-input" value={house.name} onchange={(event) => saveHouse({ ...house, name: event.currentTarget.value.trim() || house.name })} /></div>
  <div class="m-field">
    <span class="m-lbl">Town</span>
    <Select grow value={house.townId} options={townOptions} onchange={(value) => saveHouse({ ...house, townId: value })} />
    {#if house.townId}<button class="t-btn sm" onclick={() => mapCommands.focus({ kind: "town", id: house.townId })}>Open</button>{/if}
  </div>
  <div class="m-field">
    <span class="m-lbl">Rent</span>
    <NumberField value={house.rent} min={0} max={100000000} width={90} onchange={(value) => saveHouse({ ...house, rent: value })} />
    <label class="row"><input class="t-check" type="checkbox" checked={house.guildhall} onchange={(event) => saveHouse({ ...house, guildhall: event.currentTarget.checked })} />Guildhall</label>
  </div>
  <div class="m-field"><span class="m-lbl">Entrance</span><PosField value={house.exit} pickLabel="Click the tile in front of the door of {house.name}" onchange={(position) => saveHouse({ ...house, exit: position })} /></div>
  <div class="m-field"><span class="m-lbl">Size</span><span>{house.size ? `${house.size} tiles` : "No tiles yet"}</span><span class="t-label">#{house.id}</span></div>

  <div class="paint" class:on={painting}>
    {#if painting}
      <span>Painting the tiles of {house.name}. <kbd>Ctrl</kbd> removes tiles, <kbd>Esc</kbd> stops.</span>
      <button class="t-btn sm" onclick={() => (map.tool = "select")}>Done</button>
    {:else}
      <span>Paint the floor of the house on the map.</span>
      <button class="t-btn sm primary" onclick={() => paintHouse(house)}>Paint tiles</button>
    {/if}
  </div>

  <div class="acts">
    <button class="t-btn" onclick={() => goHouse(house)}>Go to</button>
    <button class="t-btn" disabled={!house.size} onclick={() => selectHouseTiles(house)}>Select tiles</button>
    <button class="t-btn" title="Click the tile in front of the door" onclick={() => selectBrush({ kind: "houseExit", house: house.id, label: `${house.name} entrance` })}>Place entrance</button>
    <span class="grow"></span>
    <button class="t-btn danger" onclick={() => removeHouse(house)}>Remove</button>
  </div>
</div>

<style>
  .paint {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border: 1px solid #444;
    background: rgba(0, 0, 0, 0.2);
    font-size: 12px;
    color: var(--text-dim);
  }
  .paint.on {
    border-color: var(--gold);
    color: var(--text);
  }
  .paint span {
    flex: 1;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
  }
  .acts .t-btn,
  .sm {
    min-width: 0;
    padding: 0 8px;
  }
  .grow {
    flex: 1;
  }
</style>
