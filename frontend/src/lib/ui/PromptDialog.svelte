<script lang="ts">
  import { answerText, promptState } from "../confirm.svelte";
  import Dialog from "./Dialog.svelte";

  const question = $derived(promptState.question);
  let value = $state("");
  $effect.pre(() => {
    value = question?.value ?? "";
  });
  const ok = () => answerText(value.trim() ? value.trim() : null);
</script>

{#if question}
  <Dialog title={question.title} width={360} onclose={() => answerText(null)}>
    <label class="col">
      <span class="t-label">{question.label}</span>
      <input class="t-input" bind:value onkeydown={(event) => event.key === "Enter" && ok()} />
    </label>
    {#snippet footer()}
      <span class="grow"></span>
      <button class="t-btn primary" disabled={!value.trim()} onclick={ok}>{question.ok ?? "OK"}</button>
      <button class="t-btn" onclick={() => answerText(null)}>Cancel</button>
    {/snippet}
  </Dialog>
{/if}
