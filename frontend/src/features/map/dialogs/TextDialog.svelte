<script lang="ts">
  import { onMount } from "svelte";
  import { errorMessage } from "../../../lib/api";
  import { toast } from "../../../lib/state.svelte";
  import { MapService, fmtPos, type TileItem } from "../../../lib/map/api";
  import { edit, map } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";
  import ItemIcon from "../ItemIcon.svelte";

  // The text window of the game: the item, a line about it and the text,
  // always editable.
  const target = map.textItem!;
  let item = $state<TileItem | null>(null);
  let text = $state("");
  let area = $state<HTMLTextAreaElement>();

  onMount(async () => {
    try {
      const tile = await MapService.Tile(map.activeId!, target.pos);
      item = tile.items?.[target.index] ?? null;
      if (!item) return close();
      text = item.ext?.text ?? "";
      queueMicrotask(() => area?.focus());
    } catch (err) {
      toast(errorMessage(err), "error");
      close();
    }
  });

  const writable = $derived(!!item && !!map.items?.is(item.id, "writable"));

  function close() {
    map.dialog = null;
    map.textItem = null;
  }

  function save() {
    if (item && text !== (item.ext?.text ?? "")) {
      void edit((id) => MapService.UpdateItem(id, { pos: target.pos, index: target.index, path: [] }, { text }));
    }
    close();
  }

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      save();
    }
  }
</script>

<Dialog title={writable ? "Edit Text" : "Show Text"} width={340} onclose={close}>
  <div class="head">
    <span class="slot">{#if item}<ItemIcon id={item.id} />{/if}</span>
    <span class="col">
      <span>{item ? (text ? "You read the following:" : writable ? "You can enter new text." : "Nothing is written here.") : "Loading…"}</span>
      {#if item}<span class="t-label sub">{item.name} · {item.id} · {fmtPos(target.pos)}</span>{/if}
    </span>
  </div>
  <textarea class="t-input paper" bind:this={area} bind:value={text} {onkeydown} disabled={!item} spellcheck="false"></textarea>
  {#snippet footer()}
    <span class="t-label grow">Ctrl+Enter saves</span>
    <button class="t-btn primary" disabled={!item} onclick={save}>OK</button>
    <button class="t-btn" onclick={close}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .head {
    display: flex;
    gap: 8px;
    align-items: center;
    margin-bottom: 6px;
  }
  .slot {
    width: 34px;
    height: 34px;
    display: grid;
    place-items: center;
    flex: none;
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid #111;
    box-shadow: inset 1px 1px 0 #2a2a2a;
  }
  .col {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .sub {
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .paper {
    display: block;
    box-sizing: border-box;
    width: 100%;
    height: 260px;
    resize: vertical;
    padding: 4px 6px;
    font-weight: normal;
    line-height: 15px;
    white-space: pre-wrap;
  }
  .grow {
    flex: 1;
  }
</style>
