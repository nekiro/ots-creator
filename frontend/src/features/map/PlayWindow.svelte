<script lang="ts">
  // The play view in its own window (WindowService.OpenPlayWindow): the
  // map and start come in the URL, edits made in the editor window come as
  // map:changed events.
  import { onMount } from "svelte";
  import { Events, Window } from "@wailsio/runtime";
  import { initState } from "../../lib/state/app.svelte";
  import { loadMapData, map } from "../../lib/map/state.svelte";
  import { spriteSize } from "../../lib/map/view/commands";
  import type { Position } from "../../lib/map/play/player";
  import PlayView from "./PlayView.svelte";

  const params = new URLSearchParams(location.search);
  const mapId = Number(params.get("play"));
  const start: Position = { x: Number(params.get("x")), y: Number(params.get("y")), z: Number(params.get("z")) };

  let ready = $state(false);
  const listeners = new Set<(keys: number[]) => void>();

  function changes(changed: (keys: number[]) => void): () => void {
    listeners.add(changed);
    return () => listeners.delete(changed);
  }

  onMount(() => {
    const off = Events.On("map:changed", (event: { data: { id: number; chunks: number[] | null } | null }) => {
      if (event.data?.id !== mapId) return;
      for (const changed of listeners) changed(event.data.chunks ?? []);
    });
    void (async () => {
      await initState();
      await loadMapData();
      ready = true;
    })();
    return off;
  });
</script>

<div class="window">
  {#if ready && map.items}
    <PlayView {mapId} {start} items={map.items} spriteSize={spriteSize()} onexit={() => void Window.Close()} {changes} />
  {:else}
    <div class="loading t-label">Loading the map…</div>
  {/if}
</div>

<style>
  .window {
    position: fixed;
    inset: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg, #161616);
  }
  .loading {
    margin: auto;
  }
</style>
