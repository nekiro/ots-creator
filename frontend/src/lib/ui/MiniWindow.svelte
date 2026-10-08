<script lang="ts">
  import type { Snippet } from "svelte";

  let {
    title,
    actions,
    children,
    class: cls = "",
    style = "",
  }: { title: string; actions?: Snippet; children: Snippet; class?: string; style?: string } = $props();
</script>

<section class="t-mini {cls}" {style}>
  <header class="t-mini-header">
    <span class="title">{title}</span>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </header>
  <div class="t-mini-body">
    {@render children()}
  </div>
</section>

<style>
  .title {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .actions {
    display: flex;
    gap: 2px;
    font-weight: normal;
  }
</style>
