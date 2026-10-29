<script lang="ts" module>
  import type { EditorSettings } from "../../../lib/map/data/api";

  export const MAP_OPTIONS: { key: keyof EditorSettings; label: string; hint: string }[] = [
    { key: "automagic", label: "Automatic borders and walls", hint: "Ground brushes place borders, walls connect to their neighbors, carpets and tables join." },
    { key: "eraserLeaveUnique", label: "Eraser keeps items with ids or text", hint: "Items with an action id, unique id, text or contents survive the eraser." },
    { key: "houseRemoveItems", label: "House brush removes loose items", hint: "Moveable items without attributes are removed from new house tiles." },
    { key: "autoDoorId", label: "Number house doors", hint: "Doors on house tiles get the lowest free door id of the house." },
    { key: "doodadEraseLike", label: "Doodad eraser only removes its own items", hint: "Ctrl with a doodad brush only removes items of that brush." },
    { key: "autoCreateSpawn", label: "Creatures create a spawn", hint: "A creature placed outside every spawn gets a spawn of radius 1." },
    { key: "rawReplaceBottom", label: "Raw walls replace walls", hint: "Placing a bottom item (wall) removes the bottom items already on the tile." },
    { key: "countU16", label: "Item counts as 16-bit numbers", hint: "For servers that read the count attribute as uint16 (stacks above 255). Maps open and save in this format; standard servers cannot read them." },
  ];
</script>

<script lang="ts">
  import { errorMessage } from "../../../lib/api";
  import { toast } from "../../../lib/state/app.svelte";
  import { MapService } from "../../../lib/map/data/api";
  import { map } from "../../../lib/map/state.svelte";
  import NumberField from "../../../lib/ui/NumberField.svelte";

  /** "459, 460" -> [459, 460]: valid item ids, each once (null: none given, the defaults). */
  function parseItemIds(text: string): number[] | null {
    const ids = text.split(/[\s,;]+/).map(Number).filter((id) => Number.isInteger(id) && id > 0 && id <= 0xffff);
    return ids.length ? [...new Set(ids)] : null;
  }

  // Each change is saved at once and applies to the open maps.
  async function change(patch: Partial<EditorSettings>) {
    if (!map.settings) return;
    const next = { ...map.settings, ...patch };
    try {
      await MapService.SetEditorSettings(next);
      map.settings = next;
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }
</script>

{#if map.settings}
  {@const settings = map.settings}
  <h4>Editing</h4>
  {#each MAP_OPTIONS as option}
    <label class="opt">
      <input class="t-check" type="checkbox" checked={!!settings[option.key]} onchange={(event) => change({ [option.key]: event.currentTarget.checked })} />
      <span class="col"><span>{option.label}</span><span class="t-label hint">{option.hint}</span></span>
    </label>
  {/each}

  <h4>Spawns</h4>
  <div class="row"><span>Default spawn time</span><NumberField value={settings.defaultSpawnTime} min={1} max={86400} width={60} onchange={(value) => change({ defaultSpawnTime: value })} /><span class="t-label">seconds</span></div>

  <h4>Marked items</h4>
  <input class="t-input" placeholder="Default: 459, 460 (items.otb) or 469, 470 (client ids)" value={(settings.markedItems ?? []).join(", ")} onchange={(event) => change({ markedItems: parseItemIds(event.currentTarget.value) })} />
  <span class="t-label hint">Item ids covered with a square in their minimap color (View > Marked items), such as invisible stairs and holes. Empty: the defaults of the item data.</span>
{:else}
  <span class="t-label">Loading…</span>
{/if}

<style>
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
</style>
