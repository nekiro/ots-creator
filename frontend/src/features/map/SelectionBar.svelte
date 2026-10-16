<script lang="ts">
  // Actions for the selected tiles, shown over the map while something is
  // selected.
  import { MapService } from "../../lib/map/data/api";
  import { mapCommands } from "../../lib/map/view/commands";
  import { edit, map } from "../../lib/map/state.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import { plural } from "../../lib/util/format";

  const selectedCount = $derived(map.active?.selection ?? 0);
  const activeMap = $derived(map.active);
  const size = $derived(activeMap && selectedCount ? `${activeMap.selMax.x - activeMap.selMin.x + 1}×${activeMap.selMax.y - activeMap.selMin.y + 1}${activeMap.selMin.z !== activeMap.selMax.z ? ` · floors ${activeMap.selMin.z}-${activeMap.selMax.z}` : ""}` : "");
</script>

{#if selectedCount && !map.pasting && !map.pick}
  <div class="bar" role="toolbar" aria-label="Selection">
    <span class="count" title={size}>{plural(selectedCount, "tile")}</span>
    <span class="sep"></span>
    <button title="Copy (Ctrl+C)" onclick={mapCommands.copy}><Icon name="copy" /></button>
    <button title="Cut (Ctrl+X)" onclick={mapCommands.cut}><Icon name="replace" /></button>
    <button title="Delete (Del)" onclick={mapCommands.remove}><Icon name="trash" /></button>
    <span class="sep"></span>
    <button title="Move one floor up" onclick={() => edit((id) => MapService.MoveSelection(id, 0, 0, -1))}><Icon name="arrowN" /></button>
    <button title="Move one floor down" onclick={() => edit((id) => MapService.MoveSelection(id, 0, 0, 1))}><Icon name="arrowS" /></button>
    <span class="sep"></span>
    <button class="txt" title="Redo the borders and walls (Ctrl+B)" onclick={() => mapCommands.tool("borderizeSelection")}>Borders</button>
    <button class="txt" title="Pick new random variations of the grounds" onclick={() => mapCommands.tool("randomizeSelection")}>Randomize</button>
    <button class="txt" title="Find or replace items in the selection" onclick={() => mapCommands.find({ inSelection: true })}>Find…</button>
    <button class="txt" title="Save the selection as a decoration brush" onclick={() => ((map.editBrush = { name: "", type: "fromSelection" }), (map.dialog = "brush"))}>Make brush…</button>
    <button title="More tools for the selection" onclick={() => (map.dialog = "tools")}><Icon name="tools" /></button>
    <span class="sep"></span>
    <button title="Deselect (Esc)" onclick={mapCommands.clearSelection}><Icon name="close" /></button>
  </div>
{/if}

<style>
  .bar {
    position: absolute;
    left: 50%;
    bottom: 10px;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 1px;
    padding: 3px 5px;
    background: rgba(24, 24, 24, 0.94);
    border: 1px solid #5a5a5a;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.6);
    white-space: nowrap;
    max-width: calc(100% - 80px);
    overflow: hidden;
  }
  .count {
    color: var(--gold);
    padding: 0 6px 0 2px;
  }
  .sep {
    width: 1px;
    height: 16px;
    background: #444;
    margin: 0 3px;
    flex: none;
  }
  button {
    height: 22px;
    min-width: 22px;
    display: grid;
    place-items: center;
    background: none;
    border: 1px solid transparent;
    color: var(--text);
    font: inherit;
    cursor: pointer;
    padding: 0 3px;
  }
  button:hover {
    background: rgba(255, 255, 255, 0.08);
    border-color: #555;
    color: #fff;
  }
  .txt {
    padding: 0 6px;
  }
</style>
