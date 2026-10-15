<script lang="ts">
  // Loader over the whole window while a modal run (see run in state) works,
  // so a long operation never looks like a frozen editor.
  import { app } from "../state.svelte";
  import { backdrop, pop } from "./motion";
  import ProgressBar from "./ProgressBar.svelte";

  const percent = $derived(app.progress && app.progress.total > 0 ? (app.progress.done / app.progress.total) * 100 : 0);
</script>

{#if app.modal}
  <div class="overlay" transition:backdrop|global>
    <div class="t-window box" transition:pop|global role="alertdialog" aria-busy="true" aria-label={app.modal}>
      <div class="t-window-title">{app.modal}</div>
      <div class="row msg" role="status"><span class="spinner"></span>Please wait…</div>
      <ProgressBar percent={app.progress ? percent : 0} label={app.progress ? `${Math.floor(percent)}%` : ""} />
    </div>
  </div>
{/if}

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
  .box {
    width: 320px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 10px 12px 12px;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.6);
  }
  .msg {
    color: var(--text-bright);
  }
  .spinner {
    width: 12px;
    height: 12px;
    border: 2px solid rgba(232, 196, 106, 0.25);
    border-top-color: var(--gold);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
