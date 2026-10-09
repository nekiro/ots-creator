<script lang="ts">
  import { CATEGORY_NAMES, Category, Format, type FrameGroup } from "../lib/api";
  import { clampField, FLAG_GROUPS, flagVisible, getPath, setPath, type Field } from "../lib/flags";
  import { resizeSprites, type Layout } from "../lib/render/layout";
  import { app, applyDraft, revertDraft } from "../lib/state.svelte";
  import MiniWindow from "../lib/ui/MiniWindow.svelte";
  import NumberField from "../lib/ui/NumberField.svelte";
  import Select from "../lib/ui/Select.svelte";
  import Icon from "../lib/ui/Icon.svelte";

  let { group = $bindable(0) }: { group?: number } = $props();

  const t = $derived(app.draft);
  const g = $derived(t?.frameGroups[Math.min(group, (t?.frameGroups.length ?? 1) - 1)] ?? null);
  let filter = $state("");
  let collapsed = $state<Record<string, boolean>>({});

  const DEFAULT_DURATION: Record<number, number> = { 1: 500, 2: 300, 3: 100, 4: 75 };

  function setLayout(key: keyof Layout | "exactSize", value: number) {
    if (!g || !t) return;
    if (key === "exactSize") {
      g.exactSize = value;
      return;
    }
    const old: Layout = { ...g };
    const next: Layout = { ...old, [key]: value };
    g.sprites = resizeSprites(old, g.sprites, next);
    (g as any)[key] = value;
    if (key === "frames") syncDurations(g);
  }

  function syncDurations(grp: FrameGroup) {
    if (grp.frames <= 1) {
      grp.durations = [];
      return;
    }
    const d = DEFAULT_DURATION[t!.category] ?? 100;
    const list = [...(grp.durations ?? [])];
    while (list.length < grp.frames) list.push({ min: d, max: d });
    grp.durations = list.slice(0, grp.frames);
    if (grp.startFrame >= grp.frames) grp.startFrame = 0;
  }

  function setAllDurations(min: number, max: number) {
    if (!g?.durations) return;
    g.durations = g.durations.map(() => ({ min, max: Math.max(min, max) }));
  }

  function addWalkingGroup() {
    if (!t || t.frameGroups.length !== 1 || !t.frameGroups[0]) return;
    const idle = t.frameGroups[0]!;
    const walk: FrameGroup = JSON.parse(JSON.stringify(idle));
    walk.type = 1;
    idle.type = 0;
    t.frameGroups = [idle, walk];
    group = 1;
  }

  function removeGroup(i: number) {
    if (!t || t.frameGroups.length < 2) return;
    t.frameGroups = t.frameGroups.filter((_, k) => k !== i);
    group = 0;
  }

  function fieldValue(f: Field): any {
    return getPath(t!.props, f.key);
  }

  function setField(f: Field, raw: string | number) {
    if (!t) return;
    const v = f.kind === "text" ? String(raw) : clampField(f.kind, Number(raw));
    setPath(t.props, f.key, v);
  }

  function selectOptions(fd: Field) {
    const opts = (fd.options ?? []).map(([value, label]) => ({ value, label }));
    const cur = fieldValue(fd);
    if (!opts.some((o) => o.value === cur)) opts.push({ value: cur, label: `Unknown (${cur})` });
    return opts;
  }

  function matches(label: string, key: string) {
    const q = filter.trim().toLowerCase();
    return !q || label.toLowerCase().includes(q) || key.toLowerCase().includes(q);
  }

  const isOutfit = $derived(t?.category === Category.CategoryOutfit);
  // Asset sprites are at most 64x64 pixels.
  // Only asset clients store names and descriptions.
  const hasTexts = $derived(app.project?.info.format === Format.FormatAssets);
  const maxTiles = $derived(app.project?.info.format === Format.FormatAssets ? 2 : 8);
  const outfitGroups = $derived(isOutfit && !!app.project?.info.features.frameGroups);
</script>

<MiniWindow title={t ? `${CATEGORY_NAMES[t.category]} #${t.id}` : "Properties"} class="props">
  {#snippet actions()}
    {#if app.dirty}<span class="dirty" title="Unapplied changes">modified</span>{/if}
  {/snippet}
  {#if t && g}
    <div class="scroll">
      {#if hasTexts}
        <section>
          <h4>Object</h4>
          <label class="text"><span class="t-label">Name</span><input class="t-input" bind:value={t.name} spellcheck="false" /></label>
          <label class="text"><span class="t-label">Description</span><textarea class="t-input" rows="2" bind:value={t.description} spellcheck="false"></textarea></label>
        </section>
        {#if t.category === Category.CategoryItem}
          <section>
            <h4>NPC trade{#if t.npcSales.length}&nbsp;· {t.npcSales.length}{/if}</h4>
            {#each t.npcSales as n, i}
              <div class="npc t-panel">
                <div class="row">
                  <input class="t-input grow" placeholder="NPC name" bind:value={n.name} spellcheck="false" />
                  <button class="t-icon-btn" title="Remove this NPC" onclick={() => t.npcSales.splice(i, 1)}><Icon name="trash" /></button>
                </div>
                <input class="t-input" placeholder="Location" bind:value={n.location} spellcheck="false" />
                <div class="row wrap">
                  <NumberField label="Sells for" title="Price the player pays; 0 = does not sell" value={n.salePrice} min={0} max={4294967295} width={70} onchange={(v) => (n.salePrice = v)} />
                  <NumberField label="Buys for" title="Price the NPC pays; 0 = does not buy" value={n.buyPrice} min={0} max={4294967295} width={70} onchange={(v) => (n.buyPrice = v)} />
                </div>
                <div class="row wrap">
                  <NumberField label="Currency id" title="Item paid with; 0 = gold" value={n.currencyObjectId} min={0} max={4294967295} width={60} onchange={(v) => (n.currencyObjectId = v)} />
                  <input class="t-input grow" placeholder="Quest currency" title="Name of a quest currency shown instead of an item" bind:value={n.currencyQuestFlag} spellcheck="false" />
                </div>
              </div>
            {/each}
            <button
              class="t-btn"
              onclick={() => t.npcSales.push({ name: "", location: "", salePrice: 0, buyPrice: 0, currencyObjectId: 0, currencyQuestFlag: "" })}>Add NPC</button
            >
          </section>
        {/if}
      {/if}
      <section>
        <h4>Texture{#if t.frameGroups.length > 1}&nbsp;· {group === 0 ? "Idle" : "Walking"}{/if}</h4>
        <div class="grid">
          <NumberField label="Width" value={g.width} min={1} max={maxTiles} onchange={(v) => setLayout("width", v)} width={44} />
          <NumberField label="Height" value={g.height} min={1} max={maxTiles} onchange={(v) => setLayout("height", v)} width={44} />
          <NumberField label="Exact size" value={g.exactSize} min={1} max={255} onchange={(v) => setLayout("exactSize", v)} width={44} />
          <NumberField label="Layers" value={g.layers} min={1} max={8} onchange={(v) => setLayout("layers", v)} width={44} />
          <NumberField label="Pattern X" value={g.patternX} min={1} max={255} onchange={(v) => setLayout("patternX", v)} width={44} />
          <NumberField label="Pattern Y" value={g.patternY} min={1} max={255} onchange={(v) => setLayout("patternY", v)} width={44} />
          <NumberField label="Pattern Z" value={g.patternZ} min={1} max={255} onchange={(v) => setLayout("patternZ", v)} width={44} />
          <NumberField label="Frames" value={g.frames} min={1} max={255} onchange={(v) => setLayout("frames", v)} width={44} />
        </div>
        <div class="t-label small">{g.sprites.length} sprite slots</div>
        {#if outfitGroups}
          <div class="row wrap">
            {#if t.frameGroups.length === 1}
              <button class="t-btn" onclick={addWalkingGroup}>Add walking group</button>
            {:else}
              <button class="t-btn" onclick={() => removeGroup(group)}>Remove {group === 0 ? "idle" : "walking"} group</button>
            {/if}
          </div>
        {/if}
      </section>

      {#if g.frames > 1 && g.durations.length}
        <section>
          <h4>Animation</h4>
          <div class="row wrap">
            <label class="row">
              <span class="t-label">Mode</span>
              <Select value={Number(g.mode)} options={[{ value: 0, label: "Asynchronous" }, { value: 1, label: "Synchronous" }]} onchange={(v) => (g.mode = v)} />
            </label>
            <NumberField label="Loops" title="0 = infinite, -1 = ping-pong" value={g.loopCount} min={-1} max={1000} onchange={(v) => (g.loopCount = v)} width={44} />
            <NumberField label="Start" title="-1 = random" value={g.startFrame} min={-1} max={g.frames - 1} onchange={(v) => (g.startFrame = v)} width={44} />
          </div>
          {#if !app.project?.info.features.improvedAnimations}
            <div class="note">This client has no per-frame durations; values are only used for preview and OBD.</div>
          {/if}
          <div class="durations">
            {#each g.durations as d, i}
              <div class="row">
                <span class="t-label idx">#{i + 1}</span>
                <NumberField value={d.min} min={0} max={1000000} width={60} onchange={(v) => (g.durations[i] = { min: v, max: Math.max(v, d.max) })} />
                <span class="t-label">-</span>
                <NumberField value={d.max} min={d.min} max={1000000} width={60} onchange={(v) => (g.durations[i] = { min: d.min, max: v })} />
                <span class="t-label">ms</span>
              </div>
            {/each}
          </div>
          <div class="row">
            <button class="t-btn" onclick={() => setAllDurations(g.durations[0].min, g.durations[0].max)}>Copy #1 to all</button>
          </div>
        </section>
      {/if}

      <section>
        <h4>Flags</h4>
        <input class="t-input filter" placeholder="Filter flags…" bind:value={filter} />
        {#each FLAG_GROUPS as grp}
          {@const flags = grp.flags.filter((f) => flagVisible(f.key, t.category, !!(t.props as any)[f.key]) && matches(f.label, f.key))}
          {#if flags.length}
            <div class="group">
              <button class="ghead" onclick={() => (collapsed[grp.title] = !collapsed[grp.title])}>
                <span class="caret" class:closed={collapsed[grp.title] && !filter}>▾</span>{grp.title}
              </button>
              {#if !collapsed[grp.title] || filter}
                {#each flags as f}
                  {@const on = !!(t.props as any)[f.key]}
                  {@const supported = app.supported.has(f.key)}
                  <div class="flag" class:unsupported={!supported}>
                    <label class="row" title={supported ? (f.hint ?? "") : `Not stored by ${app.project?.info.formatLabel} clients`}>
                      <input class="t-check" type="checkbox" checked={on} onchange={(e) => ((t.props as any)[f.key] = e.currentTarget.checked)} />
                      <span class:on>{f.label}</span>
                    </label>
                    {#if on && f.fields}
                      <div class="fields">
                        {#each f.fields as fd}
                          <label class="row">
                            <span class="t-label">{fd.label}</span>
                            {#if fd.kind === "mask"}
                              <span class="mask">
                                {#each fd.options ?? [] as [bit, name]}
                                  <label class="row"
                                    ><input
                                      class="t-check"
                                      type="checkbox"
                                      checked={(fieldValue(fd) & bit) !== 0}
                                      onchange={(e) => setField(fd, e.currentTarget.checked ? fieldValue(fd) | bit : fieldValue(fd) & ~bit)}
                                    />{name}</label
                                  >
                                {/each}
                                {#if !fieldValue(fd)}<span class="t-label">(any)</span>{/if}
                              </span>
                            {:else if fd.kind === "select"}
                              <Select grow value={fieldValue(fd)} options={selectOptions(fd)} onchange={(v) => setField(fd, v)} />
                            {:else if fd.kind === "text"}
                              <input class="t-input grow" value={fieldValue(fd)} onchange={(e) => setField(fd, e.currentTarget.value)} />
                            {:else}
                              <NumberField value={fieldValue(fd)} min={fd.kind === "i16" ? -32768 : 0} max={fd.kind === "i16" ? 32767 : 65535} onchange={(v) => setField(fd, v)} />
                            {/if}
                          </label>
                        {/each}
                      </div>
                    {/if}
                    {#if on && f.key === "hasBones"}
                      <div class="fields">
                        {#each ["North", "East", "South", "West"] as dir, di}
                          <div class="row">
                            <span class="t-label dir">{dir}</span>
                            <NumberField label="X" value={t.props.bones[di].x} min={-32768} max={32767} width={50} onchange={(v) => (t.props.bones[di].x = v)} />
                            <NumberField label="Y" value={t.props.bones[di].y} min={-32768} max={32767} width={50} onchange={(v) => (t.props.bones[di].y = v)} />
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
