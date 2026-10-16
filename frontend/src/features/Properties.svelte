<script lang="ts">
  import { frameGroupAt } from "../lib/thing";
  import { CATEGORY_NAMES, Category, Format, type FrameGroup } from "../lib/api";
  import { clampField, FLAG_GROUPS, flagVisible, getPath, setPath, type Field } from "../lib/flags";
  import { setAllDurations, setLayout, type LayoutKey } from "../lib/render/frameGroups";
  import { app, applyDraft, revertDraft } from "../lib/state/app.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import NumberField from "../lib/ui/NumberField.svelte";
  import Select from "../lib/ui/Select.svelte";
  import Icon from "../lib/ui/Icon.svelte";

  let { group = $bindable(0) }: { group?: number } = $props();

  const draft = $derived(app.draft);
  const frameGroup = $derived(frameGroupAt(draft, group));
  let filter = $state("");
  let collapsed = $state<Record<string, boolean>>({});

  function changeLayout(key: LayoutKey, value: number) {
    if (!frameGroup || !draft) return;
    setLayout(frameGroup, key, value, draft.category);
  }

  function copyFirstDuration() {
    if (!frameGroup?.durations) return;
    setAllDurations(frameGroup, frameGroup.durations[0].min, frameGroup.durations[0].max);
  }

  function addWalkingGroup() {
    if (!draft || draft.frameGroups.length !== 1 || !draft.frameGroups[0]) return;
    const idle = draft.frameGroups[0]!;
    const walk: FrameGroup = JSON.parse(JSON.stringify(idle));
    walk.type = 1;
    idle.type = 0;
    draft.frameGroups = [idle, walk];
    group = 1;
  }

  function removeGroup(index: number) {
    if (!draft || draft.frameGroups.length < 2) return;
    draft.frameGroups = draft.frameGroups.filter((_, k) => k !== index);
    group = 0;
  }

  function fieldValue(field: Field): any {
    return getPath(draft!.props, field.key);
  }

  function setField(field: Field, raw: string | number) {
    if (!draft) return;
    const value = field.kind === "text" ? String(raw) : clampField(field.kind, Number(raw));
    setPath(draft.props, field.key, value);
  }

  function selectOptions(field: Field) {
    const options = (field.options ?? []).map(([value, label]) => ({ value, label }));
    const current = fieldValue(field);
    if (!options.some((option) => option.value === current)) options.push({ value: current, label: `Unknown (${current})` });
    return options;
  }

  function matches(label: string, key: string) {
    const query = filter.trim().toLowerCase();
    return !query || label.toLowerCase().includes(query) || key.toLowerCase().includes(query);
  }

  const isOutfit = $derived(draft?.category === Category.CategoryOutfit);
  // Asset sprites are at most 64x64 pixels.
  // Only asset clients store names and descriptions.
  const hasTexts = $derived(app.project?.info.format === Format.FormatAssets);
  const maxTiles = $derived(app.project?.info.format === Format.FormatAssets ? 2 : 8);
  const outfitGroups = $derived(isOutfit && !!app.project?.info.features.frameGroups);
</script>

<MiniWindow title={draft ? `${CATEGORY_NAMES[draft.category]} #${draft.id}` : "Properties"} class="props">
  {#snippet actions()}
    {#if app.dirty}<span class="dirty" title="Unapplied changes">modified</span>{/if}
  {/snippet}
  {#if draft && frameGroup}
    <div class="scroll">
      {#if hasTexts}
        <section>
          <h4>Object</h4>
          <label class="text"><span class="t-label">Name</span><input class="t-input" bind:value={draft.name} spellcheck="false" /></label>
          <label class="text"><span class="t-label">Description</span><textarea class="t-input" rows="2" bind:value={draft.description} spellcheck="false"></textarea></label>
        </section>
        {#if draft.category === Category.CategoryItem}
          <section>
            <h4>NPC trade{#if draft.npcSales.length}&nbsp;· {draft.npcSales.length}{/if}</h4>
            {#each draft.npcSales as sale, i}
              <div class="npc t-panel">
                <div class="row">
                  <input class="t-input grow" placeholder="NPC name" bind:value={sale.name} spellcheck="false" />
                  <button class="t-icon-btn danger" title="Remove this NPC" onclick={() => draft.npcSales.splice(i, 1)}><Icon name="trash" /></button>
                </div>
                <input class="t-input" placeholder="Location" bind:value={sale.location} spellcheck="false" />
                <div class="row wrap">
                  <NumberField label="Sells for" title="Price the player pays; 0 = does not sell" value={sale.salePrice} min={0} max={4294967295} width={70} onchange={(value) => (sale.salePrice = value)} />
                  <NumberField label="Buys for" title="Price the NPC pays; 0 = does not buy" value={sale.buyPrice} min={0} max={4294967295} width={70} onchange={(value) => (sale.buyPrice = value)} />
                </div>
                <div class="row wrap">
                  <NumberField label="Currency id" title="Item paid with; 0 = gold" value={sale.currencyObjectId} min={0} max={4294967295} width={60} onchange={(value) => (sale.currencyObjectId = value)} />
                  <input class="t-input grow" placeholder="Quest currency" title="Name of a quest currency shown instead of an item" bind:value={sale.currencyQuestFlag} spellcheck="false" />
                </div>
              </div>
            {/each}
            <button
              class="t-btn"
              onclick={() => draft.npcSales.push({ name: "", location: "", salePrice: 0, buyPrice: 0, currencyObjectId: 0, currencyQuestFlag: "" })}>Add NPC</button
            >
          </section>
        {/if}
      {/if}
      <section>
        <h4>Texture{#if draft.frameGroups.length > 1}&nbsp;· {group === 0 ? "Idle" : "Walking"}{/if}</h4>
        <div class="grid">
          <NumberField label="Width" value={frameGroup.width} min={1} max={maxTiles} onchange={(value) => changeLayout("width", value)} width={44} />
          <NumberField label="Height" value={frameGroup.height} min={1} max={maxTiles} onchange={(value) => changeLayout("height", value)} width={44} />
          <NumberField label="Exact size" value={frameGroup.exactSize} min={1} max={255} onchange={(value) => changeLayout("exactSize", value)} width={44} />
          <NumberField label="Layers" value={frameGroup.layers} min={1} max={8} onchange={(value) => changeLayout("layers", value)} width={44} />
          <NumberField label="Pattern X" value={frameGroup.patternX} min={1} max={255} onchange={(value) => changeLayout("patternX", value)} width={44} />
          <NumberField label="Pattern Y" value={frameGroup.patternY} min={1} max={255} onchange={(value) => changeLayout("patternY", value)} width={44} />
          <NumberField label="Pattern Z" value={frameGroup.patternZ} min={1} max={255} onchange={(value) => changeLayout("patternZ", value)} width={44} />
          <NumberField label="Frames" value={frameGroup.frames} min={1} max={255} onchange={(value) => changeLayout("frames", value)} width={44} />
        </div>
        <div class="t-label small">{frameGroup.sprites.length} sprite slots</div>
        {#if outfitGroups}
          <div class="row wrap">
            {#if draft.frameGroups.length === 1}
              <button class="t-btn" onclick={addWalkingGroup}>Add walking group</button>
            {:else}
              <button class="t-btn danger" onclick={() => removeGroup(group)}>Remove {group === 0 ? "idle" : "walking"} group</button>
            {/if}
          </div>
        {/if}
      </section>

      {#if frameGroup.frames > 1 && frameGroup.durations.length}
        <section>
          <h4>Animation</h4>
          <div class="row wrap">
            <label class="row">
              <span class="t-label">Mode</span>
              <Select value={Number(frameGroup.mode)} options={[{ value: 0, label: "Asynchronous" }, { value: 1, label: "Synchronous" }]} onchange={(value) => (frameGroup.mode = value)} />
            </label>
            <NumberField label="Loops" title="0 = infinite, -1 = ping-pong" value={frameGroup.loopCount} min={-1} max={1000} onchange={(value) => (frameGroup.loopCount = value)} width={44} />
            <NumberField label="Start" title="-1 = random" value={frameGroup.startFrame} min={-1} max={frameGroup.frames - 1} onchange={(value) => (frameGroup.startFrame = value)} width={44} />
          </div>
          {#if !app.project?.info.features.improvedAnimations}
            <div class="note">This client has no per-frame durations; values are only used for preview and OBD.</div>
          {/if}
          <div class="durations">
            {#each frameGroup.durations as duration, i}
              <div class="row">
                <span class="t-label idx">#{i + 1}</span>
                <NumberField value={duration.min} min={0} max={1000000} width={60} onchange={(value) => (frameGroup.durations[i] = { min: value, max: Math.max(value, duration.max) })} />
                <span class="t-label">-</span>
                <NumberField value={duration.max} min={duration.min} max={1000000} width={60} onchange={(value) => (frameGroup.durations[i] = { min: duration.min, max: value })} />
                <span class="t-label">ms</span>
              </div>
            {/each}
          </div>
          <div class="row">
            <button class="t-btn" onclick={copyFirstDuration}>Copy #1 to all</button>
          </div>
        </section>
      {/if}

      <section>
        <h4>Flags</h4>
        <input class="t-input filter" placeholder="Filter flags…" bind:value={filter} />
        {#each FLAG_GROUPS as flagGroup}
          {@const flags = flagGroup.flags.filter((flag) => flagVisible(flag.key, draft.category, !!(draft.props as any)[flag.key]) && matches(flag.label, flag.key))}
          {#if flags.length}
            <div class="group">
              <button class="ghead" onclick={() => (collapsed[flagGroup.title] = !collapsed[flagGroup.title])}>
                <span class="caret" class:closed={collapsed[flagGroup.title] && !filter}>▾</span>{flagGroup.title}
              </button>
              {#if !collapsed[flagGroup.title] || filter}
                {#each flags as flag}
                  {@const on = !!(draft.props as any)[flag.key]}
                  {@const supported = app.supported.has(flag.key)}
                  <div class="flag" class:unsupported={!supported}>
                    <label class="row" title={supported ? (flag.hint ?? "") : `Not stored by ${app.project?.info.formatLabel} clients`}>
                      <input class="t-check" type="checkbox" checked={on} onchange={(event) => ((draft.props as any)[flag.key] = event.currentTarget.checked)} />
                      <span class:on>{flag.label}</span>
                    </label>
                    {#if on && flag.fields}
                      <div class="fields">
                        {#each flag.fields as field}
                          <label class="row">
                            <span class="t-label">{field.label}</span>
                            {#if field.kind === "mask"}
                              <span class="mask">
                                {#each field.options ?? [] as [bit, name]}
                                  <label class="row"
                                    ><input
                                      class="t-check"
                                      type="checkbox"
                                      checked={(fieldValue(field) & bit) !== 0}
                                      onchange={(event) => setField(field, event.currentTarget.checked ? fieldValue(field) | bit : fieldValue(field) & ~bit)}
                                    />{name}</label
                                  >
                                {/each}
                                {#if !fieldValue(field)}<span class="t-label">(any)</span>{/if}
                              </span>
                            {:else if field.kind === "select"}
                              <Select grow value={fieldValue(field)} options={selectOptions(field)} onchange={(value) => setField(field, value)} />
                            {:else if field.kind === "text"}
                              <input class="t-input grow" value={fieldValue(field)} onchange={(event) => setField(field, event.currentTarget.value)} />
                            {:else}
                              <NumberField value={fieldValue(field)} min={field.kind === "i16" ? -32768 : 0} max={field.kind === "i16" ? 32767 : 65535} onchange={(value) => setField(field, value)} />
                            {/if}
                          </label>
                        {/each}
                      </div>
                    {/if}
                    {#if on && flag.key === "hasBones"}
                      <div class="fields">
                        {#each ["North", "East", "South", "West"] as dir, dirIndex}
                          <div class="row">
                            <span class="t-label dir">{dir}</span>
                            <NumberField label="X" value={draft.props.bones[dirIndex].x} min={-32768} max={32767} width={50} onchange={(value) => (draft.props.bones[dirIndex].x = value)} />
                            <NumberField label="Y" value={draft.props.bones[dirIndex].y} min={-32768} max={32767} width={50} onchange={(value) => (draft.props.bones[dirIndex].y = value)} />
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/each}
              {/if}
            </div>
          {/if}
        {/each}
      </section>
    </div>
    <div class="t-sep"></div>
    <div class="row apply">
      <button class="t-btn" disabled={!app.dirty} onclick={revertDraft}>Revert</button>
      <button class="t-btn primary" disabled={!app.dirty} onclick={applyDraft} title="Ctrl+Enter">Apply</button>
    </div>
  {:else}
    <div class="none t-label">No object selected</div>
  {/if}
</MiniWindow>

<style>
  .dirty {
    color: var(--gold);
    font-weight: normal;
  }
  .scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 2px 6px 6px 4px;
  }
  section {
    margin-bottom: 10px;
  }
  .text {
    display: grid;
    grid-template-columns: 62px 1fr;
    align-items: start;
    gap: 6px;
    margin-bottom: 4px;
  }
  .text .t-label {
    padding-top: 3px;
    text-align: right;
  }
  textarea {
    resize: vertical;
    min-height: 34px;
    font: inherit;
  }
  h4 {
    margin: 4px 0 6px;
    font-size: var(--fs);
    color: var(--text-bright);
    border-bottom: 1px solid #1a1a1a;
    box-shadow: 0 1px 0 #4a4a4a;
    padding-bottom: 3px;
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4px 8px;
    justify-items: end;
  }
  .small {
    font: var(--fs-small) / 10px var(--font-small);
    margin-top: 4px;
  }
  .wrap {
    flex-wrap: wrap;
    margin-top: 6px;
  }
  .note {
    color: var(--gold);
    font: var(--fs-small) / 10px var(--font-small);
    margin: 4px 0;
  }
  .durations {
    max-height: 160px;
    overflow-y: auto;
    margin: 6px 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .idx {
    width: 26px;
  }
  .filter {
    width: 100%;
    margin-bottom: 6px;
  }
  .ghead {
    font: bold var(--fs) var(--font);
    color: var(--text);
    background: none;
    border: 0;
    padding: 4px 0 2px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .caret {
    display: inline-block;
    transition: transform 0.1s;
  }
  .caret.closed {
    transform: rotate(-90deg);
  }
  .flag {
    padding: 2px 0 2px 10px;
  }
  .flag .on {
    color: var(--text-bright);
  }
  .npc {
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 4px;
    margin-bottom: 4px;
  }
  .mask {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 8px;
  }
  .flag.unsupported {
    opacity: 0.45;
  }
  .fields {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin: 3px 0 4px 18px;
    padding-left: 6px;
    border-left: 1px solid #4a4a4a;
  }
  .fields .t-label {
    min-width: 54px;
  }
  .dir {
    min-width: 36px !important;
  }
  .apply {
    justify-content: flex-end;
    padding: 0 4px 2px;
  }
  .none {
    padding: 12px;
  }
</style>
