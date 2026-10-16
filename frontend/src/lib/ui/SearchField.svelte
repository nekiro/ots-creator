<script lang="ts">
  // Tibia client search field: a text edit with the 20x20 clear button
  // inside at its right end. Esc clears too.
  let { value = $bindable(""), placeholder = "Search by name" }: { value?: string; placeholder?: string } = $props();
  let input: HTMLInputElement;
</script>

<div class="sf">
  <input
    bind:this={input}
    class="t-input"
    {placeholder}
    spellcheck="false"
    bind:value
    onkeydown={(event) => {
      if (event.key === "Escape" && value) {
        event.stopPropagation();
        value = "";
      }
    }}
  />
  <button
    class="clear"
    title="Clear"
    aria-label="Clear"
    onclick={() => {
      value = "";
      input.focus();
    }}
  ></button>
</div>

<style>
  /* The clear button sits inside the edit at its right end; text stops before it. */
  .sf {
    position: relative;
    display: flex;
    flex: none;
  }
  .sf .t-input {
    flex: 1;
    height: 22px;
    padding-right: 24px;
  }
  .sf .t-input::placeholder {
    font-weight: bold;
  }
  .clear {
    position: absolute;
    top: 1px;
    right: 1px;
    width: 20px;
    height: 20px;
    flex: none;
    padding: 0;
    border: 0;
    background: url("../../assets/ui/button-clear-20x20-up.png");
    image-rendering: pixelated;
    cursor: pointer;
  }
  .clear:hover {
    filter: brightness(1.15);
  }
  .clear:active {
    background-image: url("../../assets/ui/button-clear-20x20-down.png");
    filter: none;
  }
</style>
