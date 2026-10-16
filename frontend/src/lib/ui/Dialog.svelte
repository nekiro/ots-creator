<script lang="ts">
  import type { Snippet } from "svelte";
  import { backdrop, pop } from "./motion";

  let {
    title,
    width = 420,
    onclose,
    children,
    footer,
  }: { title: string; width?: number; onclose: () => void; children: Snippet; footer?: Snippet } = $props();

  let box: HTMLDivElement;

  $effect(() => {
    const first = box.querySelector<HTMLElement>("input, select, button.primary, button");
    first?.focus();
  });

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      event.stopPropagation();
      onclose();
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" transition:backdrop|global onkeydown={onkeydown} onmousedown={(event) => event.target === event.currentTarget && onclose()}>
  <div class="t-window dialog" transition:pop|global style="width:{width}px" bind:this={box} role="dialog" aria-modal="true" aria-label={title}>
    <div class="t-window-title">{title}</div>
    <div class="content">
      {@render children()}
    </div>
    {#if footer}
      <div class="t-sep"></div>
      <div class="footer">{@render footer()}</div>
    {/if}
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.45);
  }
  .dialog {
    max-height: calc(100vh - 40px);
    padding: 10px 12px 10px;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.6);
  }
  .content {
    overflow: auto;
    min-height: 0;
  }
  .footer {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }
</style>
