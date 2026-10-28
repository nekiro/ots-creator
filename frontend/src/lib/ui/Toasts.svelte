<script lang="ts">
  import { app } from "../state/app.svelte";
</script>

<div class="toasts" aria-live="polite">
  {#each app.toasts as shown (shown.id)}
    <div class="toast t-frame-up {shown.kind}">
      {shown.text}
      {#if shown.action}
        {@const action = shown.action}
        <button class="t-btn act" onclick={() => ((app.toasts = app.toasts.filter((other) => other.id !== shown.id)), action.run())}>{action.label}</button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    left: 50%;
    bottom: 34px;
    transform: translateX(-50%);
    z-index: 100;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    width: max-content;
    max-width: min(520px, calc(100vw - 28px));
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
      transform: translateY(10px);
      opacity: 0;
    }
  }
</style>
