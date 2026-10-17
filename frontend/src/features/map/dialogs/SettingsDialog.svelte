<script lang="ts">
  import { onDestroy } from "svelte";
  import { errorMessage } from "../../../lib/api";
  import { toast } from "../../../lib/state/app.svelte";
  import { MapService, type EditorSettings } from "../../../lib/map/data/api";
  import { comboOf, KEY_ACTIONS, KEY_GROUPS, keysOf, type KeyAction } from "../../../lib/map/view/keys";
  import { map } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";
  import NumberField from "../../../lib/ui/NumberField.svelte";

  let settings = $state<EditorSettings>(
    map.settings
      ? { ...map.settings }
      : { automagic: true, eraserLeaveUnique: true, houseRemoveItems: true, autoDoorId: true, doodadEraseLike: false, defaultSpawnTime: 60, autoCreateSpawn: true, rawReplaceBottom: false, countU16: false, markedItems: null },
  );
  let tab = $state(map.settingsTab);
  onDestroy(() => (map.settingsTab = "general"));

  const OPTIONS: { key: keyof EditorSettings; label: string; hint: string }[] = [
    { key: "automagic", label: "Automatic borders and walls", hint: "Ground brushes place borders, walls connect to their neighbors, carpets and tables join." },
    { key: "eraserLeaveUnique", label: "Eraser keeps items with ids or text", hint: "Items with an action id, unique id, text or contents survive the eraser." },
    { key: "houseRemoveItems", label: "House brush removes loose items", hint: "Moveable items without attributes are removed from new house tiles." },
    { key: "autoDoorId", label: "Number house doors", hint: "Doors on house tiles get the lowest free door id of the house." },
    { key: "doodadEraseLike", label: "Doodad eraser only removes its own items", hint: "Ctrl with a doodad brush only removes items of that brush." },
    { key: "autoCreateSpawn", label: "Creatures create a spawn", hint: "A creature placed outside every spawn gets a spawn of radius 1." },
    { key: "rawReplaceBottom", label: "Raw walls replace walls", hint: "Placing a bottom item (wall) removes the bottom items already on the tile." },
    { key: "countU16", label: "Item counts as 16-bit numbers", hint: "For servers that read the count attribute as uint16 (stacks above 255). Maps open and save in this format; standard servers cannot read them." },
  ];

  /** "459, 460" -> [459, 460]: valid item ids, each once (null: none given, the defaults). */
  function parseItemIds(text: string): number[] | null {
    const ids = text.split(/[\s,;]+/).map(Number).filter((id) => Number.isInteger(id) && id > 0 && id <= 0xffff);
    return ids.length ? [...new Set(ids)] : null;
  }

  // Keyboard: only actions that differ from their defaults are stored.
  let keys = $state<Record<string, string[]>>({ ...map.keys });
  let recording = $state<string | null>(null);

  const same = (first: string[], second: string[]) => first.length === second.length && first.every((key, i) => key === second[i]);

  function setKeys(action: KeyAction, list: string[]) {
    const next = { ...keys };
    if (same(list, action.keys)) delete next[action.id];
    else next[action.id] = list;
    keys = next;
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
    for (const action of KEY_ACTIONS) {
      const list = keysOf(action, keys);
      if (action !== target && list.includes(combo)) {
        setKeys(action, list.filter((key) => key !== combo));
        toast(`${combo} no longer does "${action.label}".`);
      }
    }
    const list = keysOf(target, keys);
    if (!list.includes(combo)) setKeys(target, [...list, combo]);
    recording = null;
  }

  async function save() {
    try {
      await MapService.SetEditorSettings($state.snapshot(settings));
      await MapService.SetKeys($state.snapshot(keys));
      map.settings = { ...settings };
      map.keys = $state.snapshot(keys);
      map.dialog = null;
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }
</script>

<svelte:window onkeydowncapture={onkeydown} />

<Dialog title="Map editor settings" width={520} onclose={() => (map.dialog = null)}>
  <div class="t-tabs tabs" role="tablist">
    <button class="t-tab" role="tab" class:on={tab === "general"} aria-selected={tab === "general"} onclick={() => (tab = "general")}>General</button>
    <button class="t-tab" role="tab" class:on={tab === "keys"} aria-selected={tab === "keys"} onclick={() => (tab = "keys")}>Keyboard</button>
  </div>
  {#if tab === "general"}
    <div class="col form">
      {#each OPTIONS as option}
        <label class="opt">
          <input class="t-check" type="checkbox" checked={!!settings[option.key]} onchange={(event) => ((settings as unknown as Record<string, unknown>)[option.key] = event.currentTarget.checked)} />
          <span class="col"><span>{option.label}</span><span class="t-label hint">{option.hint}</span></span>
        </label>
      {/each}
      <div class="row"><span>Default spawn time</span><NumberField value={settings.defaultSpawnTime} min={1} max={86400} width={60} onchange={(value) => (settings.defaultSpawnTime = value)} /><span class="t-label">seconds</span></div>
      <label class="col">
        <span>Marked items</span>
        <input class="t-input" placeholder="Default: 459, 460 (items.otb) or 469, 470 (client ids)" value={(settings.markedItems ?? []).join(", ")} onchange={(event) => (settings.markedItems = parseItemIds(event.currentTarget.value))} />
        <span class="t-label hint">Item ids covered with a square in their minimap color (View > Marked items), such as invisible stairs and holes. Empty: the defaults of the item data.</span>
      </label>
    </div>
  {:else}
    <div class="t-label hint">Click + and press a key. Click a key to remove it. Esc, the arrows, 1–9 and mouse gestures cannot be changed.</div>
    <div class="t-panel keys">
      {#each KEY_GROUPS as group}
        <div class="m-sect">{group}</div>
        {#each KEY_ACTIONS.filter((action) => action.group === group) as action (action.id)}
          {@const list = keysOf(action, keys)}
          <div class="krow">
            <span class="klabel">{action.label}</span>
            {#each list as key}
              <button class="key" title="Remove {key}" onclick={() => setKeys(action, list.filter((other) => other !== key))}>{key}</button>
            {/each}
            {#if recording === action.id}
              <span class="key rec">Press a key…</span>
            {:else}
              <button class="key add" title="Add a key" onclick={() => (recording = action.id)}>+</button>
            {/if}
            {#if action.id in keys}
              <button class="reset" title="Back to {action.keys.join(', ') || 'none'}" onclick={() => setKeys(action, action.keys)}>↺</button>
            {/if}
          </div>
        {/each}
      {/each}
    </div>
  {/if}
  {#snippet footer()}
    {#if tab === "keys"}
      <button class="t-btn danger" disabled={!Object.keys(keys).length} onclick={() => (keys = {})}>Reset all keys</button>
      <span class="grow"></span>
    {/if}
    <button class="t-btn primary" onclick={save}>Save</button>
    <button class="t-btn" onclick={() => (map.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .tabs {
    margin-bottom: 8px;
  }
  .form {
    gap: 8px;
  }
  .opt {
    display: flex;
    gap: 8px;
    align-items: flex-start;
    cursor: pointer;
  }
  .opt .t-check {
    margin-top: 2px;
  }
  .col {
    gap: 0;
  }
  .hint {
    font-size: 12px;
  }
  .keys {
    margin-top: 4px;
    height: 380px;
    overflow-y: auto;
    padding: 2px 4px 6px;
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
  .grow {
    flex: 1;
  }
</style>
