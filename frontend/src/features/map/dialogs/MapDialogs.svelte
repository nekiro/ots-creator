<script lang="ts">
  import { map } from "../../../lib/map/state.svelte";
  import BrushEditorDialog from "./BrushEditorDialog.svelte";
  import DataDialog from "./DataDialog.svelte";
  import ImportMapDialog from "./ImportMapDialog.svelte";
  import MapToolsDialog from "./MapToolsDialog.svelte";
  import MinimapExportDialog from "./MinimapExportDialog.svelte";
  import ShortcutsDialog from "./ShortcutsDialog.svelte";
  import TextDialog from "./TextDialog.svelte";
  import WarningsDialog from "./WarningsDialog.svelte";

  const has = $derived(map.activeId !== null);
</script>

{#if map.dialog === "data"}<DataDialog />{/if}
{#if map.dialog === "shortcuts"}<ShortcutsDialog />{/if}
{#if map.dialog === "brush"}<BrushEditorDialog />{/if}
{#if has}
  {#if map.dialog === "import"}<ImportMapDialog />{/if}
  {#if map.dialog === "minimap"}<MinimapExportDialog />{/if}
  {#if map.dialog === "tools"}<MapToolsDialog />{/if}
  {#if map.dialog === "warnings"}<WarningsDialog />{/if}
  {#if map.dialog === "text" && map.textItem}<TextDialog />{/if}
{/if}
