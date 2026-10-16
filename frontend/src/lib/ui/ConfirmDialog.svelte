<script lang="ts">
  import { answer, confirmState, DESTRUCTIVE } from "../confirm.svelte";
  import Dialog from "./Dialog.svelte";

  const question = $derived(confirmState.question);
</script>

{#if question}
  <Dialog title={question.title} width={380} onclose={() => answer(false)}>
    <p>{question.message}</p>
    {#snippet footer()}
      <span class="grow"></span>
      <button class="t-btn primary" class:danger={question.danger ?? DESTRUCTIVE.test(question.ok)} onclick={() => answer(true)}>{question.ok}</button>
      {#if question.alt}<button class="t-btn" onclick={() => answer("alt")}>{question.alt}</button>{/if}
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
