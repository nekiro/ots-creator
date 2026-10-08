<script lang="ts">
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
    <span class="grow"></span>
    {#if app.busy}<span class="busy">{app.busy}…</span>{/if}
    <span class="t-label path" title={info.datPath}>{info.datPath || "not saved"}</span>
  {:else}
    <span class="t-label">No client loaded</span>
    <span class="grow"></span>
    {#if app.busy}<span class="busy">{app.busy}…</span>{/if}
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
  .busy {
    color: var(--gold);
  }
  .path {
    max-width: 380px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
