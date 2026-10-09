<script lang="ts">
  import { app } from "../../lib/state.svelte";
  import { IMAGE_FORMATS, LIST_COLUMNS, prefs, removeRecent, savePrefs } from "../../lib/prefs.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Select from "../../lib/ui/Select.svelte";

  const s = $derived(prefs.settings);
  const recent = $derived(s.recent?.length ?? 0);
</script>

<Dialog title="Settings" width={420} onclose={() => (app.dialog = null)}>
  <div class="col">
    <h4>Startup</h4>
    <label class="row"><input class="t-check" type="checkbox" checked={s.checkUpdates} onchange={(e) => savePrefs({ checkUpdates: e.currentTarget.checked })} />Check for updates on startup</label>
    <label class="row"><input class="t-check" type="checkbox" checked={s.reopenLast} onchange={(e) => savePrefs({ reopenLast: e.currentTarget.checked })} />Reopen the last client on startup</label>

    <h4>Layout</h4>
    <div class="row">
      <NumberField
        label="Objects per row"
        value={s.listColumns}
        min={LIST_COLUMNS.min}
        max={LIST_COLUMNS.max}
        width={40}
        onchange={(v) => savePrefs({ listColumns: v })}
      />
      <span class="t-label">({LIST_COLUMNS.min}-{LIST_COLUMNS.max}, the object list resizes to fit)</span>
    </div>

    <h4>Export</h4>
    <label class="row">
      <span class="t-label lbl">Image format</span>
      <Select grow value={s.exportFormat} options={IMAGE_FORMATS} onchange={(v) => savePrefs({ exportFormat: v })} />
    </label>
    <label class="row" title="OTOBJ is readable and editable (JSON and PNG); OBD opens in ObjectBuilder">
      <span class="t-label lbl">Object files</span>
      <Select
        grow
        value={s.objectFormat}
        options={[
          { value: "otobj", label: "OTOBJ (.otobj)" },
          { value: "obd", label: "ObjectBuilder (.obd)" },
        ]}
        onchange={(v) => savePrefs({ objectFormat: v })}
      />
    </label>
    <label class="row">
      <span class="t-label lbl">Sheet background</span>
      <Select
        grow
        value={s.sheetBackground}
        options={[
          { value: "magenta", label: "Magenta (ObjectBuilder)" },
          { value: "transparent", label: "Transparent" },
        ]}
        onchange={(v) => savePrefs({ sheetBackground: v })}
      />
    </label>
    <p class="t-label note">BMP and JPG have no transparency; empty pixels become magenta.</p>

    <h4>Market</h4>
    <label class="row">
      <span class="t-label lbl">Admin token</span>
      <input
        class="t-input grow"
        type="password"
        autocomplete="off"
        placeholder="Only for market moderators"
        value={s.marketAdminToken}
        onchange={(e) => savePrefs({ marketAdminToken: e.currentTarget.value.trim() })}
      />
    </label>
    <p class="t-label note">With the admin token every market entry can be deleted.</p>

    <h4>Recent clients</h4>
    <div class="row">
      <span class="t-label grow">{recent ? `${recent} remembered` : "None yet"}</span>
      <button class="t-btn" disabled={!recent} onclick={() => removeRecent(null)}>Clear list</button>
    </div>
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  h4 {
    margin: 6px 0 0;
    color: var(--text-bright);
    font-size: var(--fs);
  }
  h4:first-child {
    margin-top: 0;
  }
  .lbl {
    width: 110px;
  }
  .note {
    margin: 0;
    font: var(--fs-small) / 10px var(--font-small);
  }
</style>
