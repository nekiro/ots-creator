<script lang="ts">
  // Icon of a map item (server id) or a creature outfit, from the client
  // thumbnails.
  import { Category, res } from "../../lib/api";
  import { map } from "../../lib/map/state.svelte";
  import { versions } from "../../lib/state/app.svelte";
  import { loading } from "../../lib/ui/loading";

  let { id = 0, outfit = 0, size = 32 }: { id?: number; outfit?: number; size?: number } = $props();

  const client = $derived(id && map.items ? map.items.client[id] : 0);
  const src = $derived(
    outfit ? res.thumb(Category.CategoryOutfit, outfit, versions.thing(Category.CategoryOutfit, outfit)) : client ? res.thumb(Category.CategoryItem, client, versions.thing(Category.CategoryItem, client)) : "",
  );
</script>

<span class="icon" style="width:{size}px;height:{size}px">
  {#if src}
    <img class="pixel" {src} alt="" loading="lazy" decoding="async" draggable="false" use:loading={src} style="max-width:{size}px;max-height:{size}px" />
  {:else if id}
    <span class="missing" title="Unknown item">?</span>
  {/if}
</span>

<style>
  .icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
    position: relative;
  }
  img {
    object-fit: contain;
  }
  .missing {
    color: #d66;
    font-weight: bold;
  }
</style>
