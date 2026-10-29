<script lang="ts">
  import { errorMessage } from "../../../lib/api";
  import { toast } from "../../../lib/state/app.svelte";
  import { MapService } from "../../../lib/map/data/api";
  import { comboOf, KEY_ACTIONS, KEY_GROUPS, keysOf, type KeyAction } from "../../../lib/map/view/keys";
  import { map } from "../../../lib/map/state.svelte";

  // Only actions that differ from their defaults are stored; each change is saved at once.
  let recording = $state<string | null>(null);

  const same = (first: string[], second: string[]) => first.length === second.length && first.every((key, i) => key === second[i]);

  async function save(keys: Record<string, string[]>) {
    try {
      await MapService.SetKeys(keys);
      map.keys = keys;
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  function withKeys(keys: Record<string, string[]>, action: KeyAction, list: string[]): Record<string, string[]> {
    const next = { ...keys };
    if (same(list, action.keys)) delete next[action.id];
    else next[action.id] = list;
    return next;
  }

  // Records the next key press for the action; a combination used by another
  // action moves here.
  function onkeydown(event: KeyboardEvent) {
    if (!recording) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.key === "Escape") {
      recording = null;
      return;
    }
    const combo = comboOf(event);
    if (!combo) return;
    const target = KEY_ACTIONS.find((action) => action.id === recording)!;
    let keys = map.keys;
    for (const action of KEY_ACTIONS) {
      const list = keysOf(action, keys);
      if (action !== target && list.includes(combo)) {
        keys = withKeys(keys, action, list.filter((key) => key !== combo));
        toast(`${combo} no longer does "${action.label}".`);
      }
    }
    const list = keysOf(target, keys);
    if (!list.includes(combo)) keys = withKeys(keys, target, [...list, combo]);
    recording = null;
    void save(keys);
  }
</script>

<svelte:window onkeydowncapture={onkeydown} />

<div class="head row">
  <span class="t-label grow">Shortcuts of the map editor. Click + and press a key; click a key to remove it. Esc, the arrows, 1–9 and mouse gestures cannot be changed.</span>
  <button class="t-btn danger" disabled={!Object.keys(map.keys).length} onclick={() => save({})}>Reset all</button>
</div>
<div class="t-panel keys">
  {#each KEY_GROUPS as group}
    <div class="sect">{group}</div>
    {#each KEY_ACTIONS.filter((action) => action.group === group) as action (action.id)}
      {@const list = keysOf(action, map.keys)}
      <div class="krow">
        <span class="klabel">{action.label}</span>
        {#each list as key}
          <button class="key" title="Remove {key}" onclick={() => save(withKeys(map.keys, action, list.filter((other) => other !== key)))}>{key}</button>
        {/each}
        {#if recording === action.id}
          <span class="key rec">Press a key…</span>
        {:else}
          <button class="key add" title="Add a key" onclick={() => (recording = action.id)}>+</button>
        {/if}
        {#if action.id in map.keys}
          <button class="reset" title="Back to {action.keys.join(', ') || 'none'}" onclick={() => save(withKeys(map.keys, action, action.keys))}>↺</button>
        {/if}
      </div>
    {/each}
  {/each}
</div>

<style>
  .head {
    align-items: flex-start;
    font-size: 12px;
  }
  .keys {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 2px 4px 6px;
  }
  .sect {
    padding: 5px 2px 1px;
    color: var(--text-dim);
    font: var(--fs-small) / 12px var(--font-small);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .krow {
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 22px;
    padding: 0 2px;
  }
  .krow:hover {
    background: rgba(255, 255, 255, 0.04);
  }
  .klabel {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .key {
    font: var(--fs-small) / 14px var(--font-small);
    color: var(--text-bright);
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid #444;
    padding: 1px 5px;
    cursor: pointer;
  }
  .key:hover:not(.rec, .add) {
    border-color: #a44;
    color: #f88;
    text-decoration: line-through;
  }
  .key.add {
    color: var(--text-dim);
  }
  .key.add:hover {
    color: var(--text-bright);
    border-color: #777;
  }
  .key.rec {
    color: var(--gold);
    border-color: var(--gold);
    cursor: default;
  }
  .reset {
    background: none;
    border: 0;
    color: var(--text-dim);
    cursor: pointer;
    padding: 0 2px;
    font: inherit;
  }
  .reset:hover {
    color: var(--text-bright);
  }
</style>
