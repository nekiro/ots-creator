<script lang="ts">
  // The values of a set flag (light level and color, cloth slot…), shared by
  // the properties panel and the bulk editor. The owner reads and stores them.
  import type { Field } from "../lib/flags";
  import NumberField from "../lib/ui/NumberField.svelte";
  import Select from "../lib/ui/Select.svelte";

  let {
    fields,
    value,
    onset,
    options,
    showAny = false,
  }: {
    fields: Field[];
    /** The current value of a field. */
    value: (field: Field) => any;
    onset: (field: Field, value: string | number) => void;
    /** Choices of a select field. */
    options: (field: Field) => { value: number; label: string }[];
    /** "(any)" next to a mask with no bit set. */
    showAny?: boolean;
  } = $props();
</script>

<div class="fields">
  {#each fields as field}
    <label class="row">
      <span class="t-label">{field.label}</span>
      {#if field.kind === "mask"}
        <span class="mask">
          {#each field.options ?? [] as [bit, name]}
            <label class="row"
              ><input
                class="t-check"
                type="checkbox"
                checked={(value(field) & bit) !== 0}
                onchange={(event) => onset(field, event.currentTarget.checked ? value(field) | bit : value(field) & ~bit)}
              />{name}</label
            >
          {/each}
          {#if showAny && !value(field)}<span class="t-label">(any)</span>{/if}
        </span>
      {:else if field.kind === "select"}
        <Select grow value={value(field)} options={options(field)} onchange={(selected) => onset(field, selected)} />
      {:else if field.kind === "text"}
        <input class="t-input grow" value={value(field)} onchange={(event) => onset(field, event.currentTarget.value)} />
      {:else}
        <NumberField value={value(field)} min={field.kind === "i16" ? -32768 : 0} max={field.kind === "i16" ? 32767 : 65535} onchange={(next) => onset(field, next)} />
      {/if}
    </label>
  {/each}
</div>

<style>
  .fields {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin: 3px 0 4px 18px;
    padding-left: 6px;
    border-left: 1px solid #4a4a4a;
  }
  .fields .t-label {
    min-width: 54px;
  }
  .mask {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 8px;
  }
</style>
