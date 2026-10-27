<script lang="ts">
  import { commands } from "../lib/commands";
  import { app } from "../lib/state/app.svelte";
  import { map, warningCount } from "../lib/map/state.svelte";
  import { plural } from "../lib/util/format";

  let { mapHover = "" }: { mapHover?: string } = $props();
  const activeMap = $derived(map.active);
  const warnings = $derived(map.activeId !== null ? warningCount(map.loadWarnings[map.activeId]) : 0);

  const info = $derived(app.project?.info);
  const flags = $derived.by(() => {
    const features = info?.features;
    if (!features) return "";
    return [features.extended && "extended", features.transparency && "transparency", features.improvedAnimations && "animations", features.frameGroups && "frame groups", features.spriteSize !== 32 && `${features.spriteSize}px`]
      .filter(Boolean)
      .join(" · ");
  });
</script>

<footer class="status">
  <div class="info">
    {#if map.mode === "map" && activeMap}
      <span class="v">{activeMap.name}</span>
      {#if activeMap.changed}<span class="changed">● unsaved changes</span>{/if}
      <span>Floor <b>{map.floor}</b></span>
      <span>Tiles <b>{activeMap.tiles.toLocaleString()}</b></span>
      {#if activeMap.selection}<span>Selected <b>{activeMap.selection.toLocaleString()}</b>{#if activeMap.selection > 1} <span class="t-label">({activeMap.selMax.x - activeMap.selMin.x + 1}×{activeMap.selMax.y - activeMap.selMin.y + 1})</span>{/if}</span>{/if}
      <span class="sep"></span>
      <span class="hover">{mapHover}</span>
    {:else if map.mode === "map"}
      <span class="t-label">{app.open ? "No map open" : "No client loaded"}</span>
    {:else if info && app.open}
      <span class="v">{info.version.name}</span>
      {#if info.changed}<span class="changed" title="The client has changes that are not compiled yet">● uncompiled changes</span>{/if}
      <span class="t-label">{info.formatLabel}</span>
      {#if flags}<span class="t-label">{flags}</span>{/if}
      <span class="sep"></span>
      <span>Items <b>{(info.counts.items - 99).toLocaleString()}</b></span>
      <span>Outfits <b>{info.counts.outfits.toLocaleString()}</b></span>
      <span>Effects <b>{info.counts.effects.toLocaleString()}</b></span>
      <span>Missiles <b>{info.counts.missiles.toLocaleString()}</b></span>
      <span>Sprites <b>{info.counts.sprites.toLocaleString()}</b></span>
    {:else}
      <span class="t-label">No client loaded</span>
    {/if}
  </div>
  {#if app.busy || app.stoppable}
    <span class="busy" class:fixed={app.stoppable}>{app.busy ?? "Exporting"}…</span>
    {#if app.stoppable}<button class="t-btn danger stop" title="Stop the export" disabled={app.stopping} onclick={commands.stopTask}>{app.stopping ? "Stopping" : "Stop"}</button>{/if}
  {:else if map.mode === "map" && activeMap}
    {#if warnings}
      <button class="warn" title="Problems found while loading the map: click to see them" onclick={() => (map.dialog = "warnings")}>⚠ {plural(warnings, "warning")}</button>
    {/if}
    <span class="t-label path" title={activeMap.path}>{activeMap.path || "not saved"}</span>
  {:else if info && app.open}
    <span class="t-label path" title={info.datPath}>{info.datPath || "not saved"}</span>
  {/if}
</footer>

<style>
  .changed {
    color: var(--gold);
  }
  .status {
    height: 22px;
    flex: none;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 10px;
    border-top: 1px solid #111;
    box-shadow: inset 0 1px 0 #444;
    background: url("../assets/ui/panel_map.png");
    white-space: nowrap;
    overflow: hidden;
  }
  .warn {
    flex: none;
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    color: #e8c45e;
    cursor: pointer;
  }
  .warn:hover {
    text-decoration: underline;
  }
  .v {
    color: var(--gold);
    font-weight: bold;
  }
  b {
    color: var(--text-bright);
    font-weight: normal;
  }
  .sep {
    width: 1px;
    height: 12px;
    background: #555;
  }
  .info {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 12px;
    overflow: hidden;
  }
  .info > * {
    flex: none;
  }
  .busy {
    flex: none;
    color: var(--gold);
  }
  /* The progress text changes width with every update: a fixed box keeps
     the stop button in place. */
  .busy.fixed {
    min-width: 270px;
    text-align: right;
  }
  .stop {
    flex: none;
    height: 18px;
    min-width: 64px;
  }
.hover {
    color: var(--text-bright);
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .path {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 380px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
