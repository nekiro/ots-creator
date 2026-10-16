<script lang="ts">
  // Batch tools. They work on the selection when there is one, otherwise on
  // the whole map; everything can be undone.
  import { ask } from "../../../lib/confirm.svelte";
  import { mapCommands } from "../../../lib/map/view/commands";
  import { map } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";

  interface Tool {
    label: string;
    hint: string;
    /** Tool names for the selection and the whole map; null when not available. */
    sel: string | null;
    all: string;
    confirm?: string;
  }
  const TOOLS: Tool[] = [
    { label: "Redo borders and walls", hint: "Places the borders of every ground and connects walls, carpets and tables again.", sel: "borderizeSelection", all: "borderizeMap" },
    { label: "Randomize grounds", hint: "Gives grounds new random variations of their brush.", sel: "randomizeSelection", all: "randomizeMap" },
    { label: "Remove unknown items", hint: "Items the item list does not know (a wrong items.otb or a deleted object).", sel: "removeInvalid", all: "removeInvalid" },
    { label: "Remove corpses", hint: "Corpses without attributes.", sel: "removeCorpses", all: "removeCorpses" },
    { label: "Remove empty spawns", hint: "Spawn areas with no creature in them.", sel: null, all: "removeEmptySpawns" },
    { label: "Clear tiles of missing houses", hint: "House tiles whose house does not exist any more.", sel: null, all: "clearInvalidHouses" },
    { label: "Remove tiles nobody can see", hint: "Tiles farther than the client view from any walkable tile.", sel: null, all: "removeUnreachable", confirm: "This removes whole tiles from the map. Continue?" },
    { label: "Forget changes", hint: "Marks every tile unchanged, for View › Only changed tiles.", sel: null, all: "clearModified" },
  ];

  const hasSel = $derived((map.active?.selection ?? 0) > 0);
  let onSel = $state((map.active?.selection ?? 0) > 0);

  async function run(t: Tool) {
    const useSel = onSel && hasSel && t.sel !== null;
    if (t.confirm && !(await ask({ title: t.label, message: t.confirm, ok: "Continue" }))) return;
    const name = useSel ? t.sel! : t.all;
    await mapCommands.tool(name, useSel && (name === "removeInvalid" || name === "removeCorpses"));
  }
</script>

<Dialog title="Map tools" width={480} onclose={() => (map.dialog = null)}>
  <div class="col">
    <div class="row scope">
      <span class="t-label">Work on</span>
      <div class="m-seg">
        <button class:on={onSel && hasSel} disabled={!hasSel} title={hasSel ? "" : "Select an area first"} onclick={() => (onSel = true)}>The selection{hasSel ? ` (${map.active?.selection.toLocaleString()} tiles)` : ""}</button>
        <button class:on={!onSel || !hasSel} onclick={() => (onSel = false)}>The whole map</button>
      </div>
    </div>
    {#each TOOLS as t}
      {@const off = onSel && hasSel && t.sel === null}
      <div class="tool" class:off>
        <span class="col grow"><span>{t.label}</span><span class="t-label hint">{off ? "Only for the whole map." : t.hint}</span></span>
        <button class="t-btn" disabled={off} onclick={() => run(t)}>Run</button>
      </div>
    {/each}
    <p class="t-label hint">Every tool is one step in the undo history.</p>
  </div>
  {#snippet footer()}
    <button class="t-btn" onclick={() => (map.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .scope {
    gap: 8px;
    margin-bottom: 4px;
  }
  .scope .m-seg {
    flex: 1;
  }
  .tool {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 0;
    border-top: 1px solid rgba(255, 255, 255, 0.05);
  }
  .tool.off {
    opacity: 0.55;
  }
  .col {
    gap: 0;
  }
  .hint {
    font-size: 12px;
    margin: 0;
  }
</style>
