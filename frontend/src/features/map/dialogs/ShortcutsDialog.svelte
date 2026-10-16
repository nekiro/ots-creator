<script lang="ts">
  import { FIXED_KEYS, KEY_ACTIONS, KEY_GROUPS, keysOf } from "../../../lib/map/view/keys";
  import { map } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";

  // Changeable keys first (as currently set), then the fixed mouse gestures.
  const groups = $derived(
    KEY_GROUPS.map((g) => ({
      title: g,
      rows: [
        ...KEY_ACTIONS.filter((a) => a.group === g).map((a) => [keysOf(a).join(", ") || "—", a.label]),
        ...FIXED_KEYS.filter((f) => f.group === g).map((f) => [f.keys, f.label]),
      ],
    })),
  );
</script>

<Dialog title="Map editor shortcuts" width={880} onclose={() => (map.dialog = null)}>
  <!-- Key and action on one line, groups flowing over two balanced columns. -->
  <div class="cols">
    {#each groups as g}
      <div class="grp">
        <div class="t-label h">{g.title}</div>
        {#each g.rows as [k, v]}
          <kbd>{k}</kbd><span>{v}</span>
        {/each}
      </div>
    {/each}
  </div>
  {#snippet footer()}
    <button class="t-btn" onclick={() => ((map.settingsTab = "keys"), (map.dialog = "settings"))}>Change keys…</button>
    <span class="grow"></span>
    <button class="t-btn" onclick={() => (map.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .cols {
    columns: 2;
    column-gap: 24px;
    padding: 2px 4px;
  }
  .grp {
    break-inside: avoid;
    display: grid;
    grid-template-columns: 170px 1fr;
    gap: 2px 10px;
    align-items: baseline;
    margin-bottom: 10px;
  }
  .h {
    grid-column: 1 / -1;
    color: var(--gold);
    margin-bottom: 1px;
  }
  kbd {
    color: var(--text-bright);
    font: inherit;
    border: 0;
    padding: 0;
    background: none;
  }
  .grp span {
    color: var(--text-dim);
    font-size: 12px;
  }
  .grow {
    flex: 1;
  }
</style>
