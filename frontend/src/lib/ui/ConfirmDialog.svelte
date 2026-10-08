<script lang="ts">
  import { answer, confirmState } from "../confirm.svelte";
  import Dialog from "./Dialog.svelte";

  const q = $derived(confirmState.question);
</script>

{#if q}
  <Dialog title={q.title} width={380} onclose={() => answer(false)}>
    <p>{q.message}</p>
    {#snippet footer()}
      <span class="grow"></span>
      <button class="t-btn primary" onclick={() => answer(true)}>{q.ok}</button>
      <button class="t-btn" onclick={() => answer(false)}>Cancel</button>
    {/snippet}
  </Dialog>
{/if}

<style>
  p {
    margin: 0;
    white-space: pre-line;
  }
</style>
