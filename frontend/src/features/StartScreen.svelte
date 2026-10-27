<script lang="ts">
  import { onMount } from "svelte";
  import { commands } from "../lib/commands";
  import { MapService } from "../lib/map/data/api";
  import { loadMapData, map } from "../lib/map/state.svelte";
  import { mapCommands } from "../lib/map/view/commands";
  import { app } from "../lib/state/app.svelte";
  import { prefs, recentDir, removeRecent, shortPath } from "../lib/state/prefs.svelte";

  onMount(() => {
    void loadMapData();
  });

  const recent = $derived((prefs.settings.recent ?? []).slice(0, 6));
  // The map editor draws with the sprites of a client: maps open with the last one.
  const mapClient = $derived(recent[0] ?? null);

  async function startMap(action: () => Promise<unknown> | void) {
    if (!mapClient) return;
    await commands.openRecent(mapClient);
    if (!app.open) return;
    await mapCommands.setMode("map");
    await action();
  }
</script>

<div class="start">
  <section class="t-window card">
    <div class="t-window-title">Objects</div>
    <p>Edit the items, outfits, effects and sprites of a client (Tibia.dat + Tibia.spr).</p>
    <div class="row">
      <button class="t-btn primary" onclick={() => commands.browseOpen()}>Open client…</button>
      <button class="t-btn" onclick={commands.create}>New client…</button>
    </div>
    {#if recent.length}
      <div class="recent">
        <div class="t-label rtitle">Recent clients</div>
        <div class="t-panel rlist">
          {#each recent as recentClient (recentClient.datPath)}
            <div class="ritem">
              <button class="ropen" title={recentClient.datPath} onclick={() => commands.openRecent(recentClient)}>
                <span class="rver">{recentClient.version.name}</span>
                <span class="rdir">{shortPath(recentDir(recentClient), 40)}</span>
              </button>
              <button class="rdel" title="Remove from list" aria-label="Remove from list" onclick={() => removeRecent(recentClient)}>×</button>
            </div>
          {/each}
        </div>
      </div>
    {/if}
    <p class="t-label hint"><kbd>Ctrl+O</kbd> open · <kbd>Ctrl+N</kbd> new · drop a client folder, .dat, .otobj or .obd here</p>
  </section>

  <section class="t-window card">
    <div class="t-window-title">Map</div>
    <p>Draw maps with the sprites of a client.</p>
    <div class="row">
      <button class="t-btn primary" disabled={!mapClient} onclick={() => startMap(mapCommands.newMap)}>New map</button>
      <button class="t-btn" disabled={!mapClient} onclick={() => startMap(() => mapCommands.open())}>Open map…</button>
    </div>
    {#if map.recent.length}
      <div class="recent">
        <div class="t-label rtitle">Recent maps</div>
        <div class="t-panel rlist">
          {#each map.recent.slice(0, 6) as recentMap (recentMap.path)}
            <div class="ritem">
              <button class="ropen" title={recentMap.path} disabled={!mapClient} onclick={() => startMap(() => mapCommands.open(recentMap.path))}>
                <span class="rdir">{shortPath(recentMap.path, 48)}</span>
              </button>
              <button class="rdel" title="Remove from list" aria-label="Remove from list" onclick={() => MapService.RemoveRecent(recentMap.path).then(() => loadMapData())}>×</button>
            </div>
          {/each}
        </div>
      </div>
    {/if}
    {#if mapClient}
      <p class="t-label hint" title={mapClient.datPath}>With client {mapClient.version.name} · {shortPath(recentDir(mapClient), 36)}</p>
    {:else}
      <p class="t-label hint">Open a client first: the map editor needs its sprites.</p>
    {/if}
  </section>
</div>

<style>
  .start {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
  }
  .card {
    width: 400px;
    align-self: center;
    padding: 16px 20px 14px;
    align-items: center;
    text-align: center;
    gap: 10px;
  }
  .card p {
    margin: 0;
  }
  .hint {
    font: var(--fs-small) / 10px var(--font-small);
  }
  .recent {
    width: 100%;
    text-align: left;
  }
  .rtitle {
    margin-bottom: 3px;
  }
  .rlist {
    padding: 2px;
  }
  .ritem {
    display: flex;
    align-items: center;
  }
  .ritem:hover {
    background: rgba(255, 255, 255, 0.06);
  }
  .ropen {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 8px;
    padding: 3px 6px;
    background: none;
    border: 0;
    font: inherit;
    color: var(--text);
    text-align: left;
    cursor: pointer;
  }
  .ropen:disabled {
    color: var(--text-dim);
    cursor: default;
  }
  .ropen:hover:not(:disabled) .rdir {
    color: var(--text-bright);
  }
  .rver {
    color: var(--gold);
    flex: none;
    min-width: 34px;
  }
  .rdir {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rdel {
    flex: none;
    width: 18px;
    background: none;
    border: 0;
    color: var(--text-dim);
    font: inherit;
    cursor: pointer;
    visibility: hidden;
  }
  .ritem:hover .rdel {
    visibility: visible;
  }
  .rdel:hover {
    color: #ff9c9c;
  }
</style>
