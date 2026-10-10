<script lang="ts">
  import { commands } from "../lib/commands";
  import { app } from "../lib/state.svelte";

  const info = $derived(app.project?.info);
  const flags = $derived.by(() => {
    const f = info?.features;
    if (!f) return "";
    return [f.extended && "extended", f.transparency && "transparency", f.improvedAnimations && "animations", f.frameGroups && "frame groups", f.spriteSize !== 32 && `${f.spriteSize}px`]
      .filter(Boolean)
      .join(" · ");
  });
</script>

<footer class="status">
  <!-- The client info gives way first on a narrow window: the busy text and
       the stop button always stay visible. -->
  <div class="info">
    {#if info && app.open}
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
  <!-- A running operation takes the place of the client path. -->
  {#if app.busy || app.stoppable}
    <span class="busy" class:fixed={app.stoppable}>{app.busy ?? "Exporting"}…</span>
    {#if app.stoppable}<button class="t-btn danger stop" title="Stop the export" disabled={app.stopping} onclick={commands.stopTask}>{app.stopping ? "Stopping" : "Stop"}</button>{/if}
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
  .path {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 380px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
