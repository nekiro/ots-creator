<script lang="ts">
  import { fmtPos, parsePos, type Position } from "../../../lib/map/data/api";
  import { goTo } from "../../../lib/map/view/commands";
  import { pickPosition } from "../../../lib/map/editor/world";
  import Icon from "../../../lib/ui/Icon.svelte";

  let { value, pickLabel, onchange }: { value: Position | null | undefined; pickLabel: string; onchange: (position: Position) => void } = $props();
  const set = $derived(!!value && (value.x > 0 || value.y > 0));
</script>

<div class="pos">
  <input
    class="t-input grow"
    value={set ? fmtPos(value) : ""}
    placeholder="not set"
    onchange={(event) => {
      const position = parsePos(event.currentTarget.value);
      if (position) onchange(position);
    }}
  />
  <button class="t-icon-btn" title="Pick on the map" onclick={() => pickPosition(pickLabel, onchange)}><Icon name="picker" /></button>
  <button class="t-icon-btn" title="Go there" disabled={!set} onclick={() => value && goTo(value)}><Icon name="arrowE" /></button>
</div>

<style>
  .pos {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 2px;
    align-items: center;
  }
  .grow {
    flex: 1;
    min-width: 0;
  }
</style>
