<script lang="ts">
  import { answerText, promptState } from "../confirm.svelte";
  import Dialog from "./Dialog.svelte";

  const q = $derived(promptState.question);
  let value = $state("");
  $effect.pre(() => {
    value = q?.value ?? "";
  });
  const ok = () => answerText(value.trim() ? value.trim() : null);
</script>

{#if q}
  <Dialog title={q.title} width={360} onclose={() => answerText(null)}>
    <label class="col">
      <span class="t-label">{q.label}</span>
      <input class="t-input" bind:value onkeydown={(event) => event.key === "Enter" && ok()} />
    </label>
    {#snippet footer()}
      <span class="grow"></span>
      <button class="t-btn primary" disabled={!value.trim()} onclick={ok}>{q.ok ?? "OK"}</button>
      <button class="t-btn" onclick={() => answerText(null)}>Cancel</button>
    {/snippet}
  </Dialog>
{/if}
