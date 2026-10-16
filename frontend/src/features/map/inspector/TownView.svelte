<script lang="ts">
  import type { Town } from "../../../lib/map/api";
  import { mapCommands } from "../../../lib/map/commands";
  import { map } from "../../../lib/map/state.svelte";
  import { addHouse, houses, removeTown, saveTown } from "../../../lib/map/world";
  import Icon from "../../../lib/ui/Icon.svelte";
  import PosField from "./PosField.svelte";

  let { town: t }: { town: Town } = $props();
  const list = $derived(map.active ? houses().filter((h) => h.townId === t.id) : []);
  const tiles = $derived(list.reduce((n, h) => n + h.size, 0));
</script>

<div class="m-form">
  <div class="m-field"><span class="m-lbl">Name</span><input class="t-input" value={t.name} onchange={(event) => saveTown({ ...t, name: event.currentTarget.value.trim() || t.name })} /></div>
  <div class="m-field"><span class="m-lbl">Temple</span><PosField value={t.temple} pickLabel="Click the temple of {t.name}" onchange={(p) => saveTown({ ...t, temple: p })} /></div>
  <div class="m-field"><span class="m-lbl">Id</span><span>{t.id}</span></div>

  <div class="m-sect">Houses <span class="grow"></span>{list.length} · {tiles} sqm</div>
  <div class="t-panel list">
    {#each list as h (h.id)}
      <button class="m-row" onclick={() => mapCommands.focus({ kind: "house", id: h.id })}>
        <span class="ico"><Icon name="home" size={12} /></span><span class="m-name">{h.name}</span><span class="m-meta">{h.size} sqm</span>
      </button>
    {:else}
      <div class="m-empty">No houses yet.</div>
    {/each}
  </div>
  <div class="acts">
    <button class="t-btn primary" onclick={() => addHouse(t.id)}>New house</button>
    <span class="grow"></span>
    <button class="t-btn danger" onclick={() => removeTown(t)}>Remove town</button>
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
