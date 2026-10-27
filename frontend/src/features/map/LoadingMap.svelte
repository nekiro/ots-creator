<script lang="ts">
  import { map } from "../../lib/map/state.svelte";
  import { backdrop, pop } from "../../lib/ui/motion";
  import ProgressBar from "../../lib/ui/ProgressBar.svelte";

  // The last state stays shown while the window fades out.
  let last = map.loading!;
  const loading = $derived.by(() => (last = map.loading ?? last));
</script>

<div class="overlay" transition:backdrop|global>
  <div class="t-window dialog" transition:pop|global role="dialog" aria-modal="true" aria-label={loading.title ?? "Opening map"}>
    <div class="t-window-title">{loading.title ?? "Opening map"}</div>
    <div class="body">
      <div class="name" title={loading.name}>{loading.name}</div>
      <ProgressBar percent={loading.percent} label="{Math.floor(loading.percent)}%" />
      <div class="t-label step">{loading.step}…</div>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 60;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.45);
  }
  .dialog {
    width: 360px;
    padding: 10px 12px 12px;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.6);
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .name {
    color: var(--text-bright);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .step {
    font-size: 12px;
  }
</style>
