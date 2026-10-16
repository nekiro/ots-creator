<script lang="ts">
  import { app } from "../state/app.svelte";
</script>

<div class="toasts" aria-live="polite">
  {#each app.toasts as t (t.id)}
    <div class="toast t-frame-up {t.kind}">
      {t.text}
      {#if t.action}
        {@const a = t.action}
        <button class="t-btn act" onclick={() => ((app.toasts = app.toasts.filter((x) => x.id !== t.id)), a.run())}>{a.label}</button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    right: 14px;
    bottom: 34px;
    z-index: 100;
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-width: 420px;
    pointer-events: none;
  }
  .toast {
    padding: 7px 10px;
    color: var(--text-bright);
    background-color: #2b2b2b;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.5);
    animation: slide 0.16s ease-out;
    user-select: text;
    word-break: break-word;
  }
  .act {
    display: block;
    margin-top: 6px;
    pointer-events: auto;
  }
  .toast.success {
    color: #9fe08c;
  }
  .toast.error {
    color: #ff9c9c;
  }
  @keyframes slide {
    from {
      transform: translateX(20px);
      opacity: 0;
    }
  }
</style>
