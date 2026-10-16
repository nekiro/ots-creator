<script lang="ts">
  import { DialogService, ProjectService, type OTBReport } from "../../lib/api";
  import { app, run, toast } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";

  const otb = $derived(app.project?.info.itemsOtb ?? null);
  let pending = $state<OTBReport | null>(null);

  // What the next compile changes; refreshed with every edit.
  $effect(() => {
    void app.project?.rev;
    if (!otb) {
      pending = null;
      return;
    }
    ProjectService.ItemsOTBPreview().then(
      (r) => (pending = r),
      () => (pending = null),
    );
  });

  async function choose() {
    const p = await DialogService.OpenFiles("itemsOtb", "items.otb", [{ name: "Server items (items.otb)", pattern: "*.otb" }], false);
    if (!p?.length) return;
    const st = await run("Loading items.otb", () => ProjectService.AttachItemsOTB(p[0]));
    if (st?.info.itemsOtb) toast(`items.otb attached: ${st.info.itemsOtb.items.toLocaleString()} server items.`, "success");
  }

  async function detach() {
    await run("Detaching items.otb", () => ProjectService.DetachItemsOTB());
  }

  const plural = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? "" : "s"}`;
</script>

<Dialog title="Server Items (items.otb)" width={480} onclose={() => (app.dialog = null)}>
  <div class="col">
    <p>
      Servers that map server ids to client ids with an items.otb can keep it in sync with this client. This is
      optional. Compiling then also updates the items.otb: server items follow moved client items, edited items get their
      client data reloaded, and new client items get new server ids. Server ids and server-only data are never changed.
    </p>
    {#if otb}
      <div class="info t-panel">
        <div class="path" title={otb.path}>{otb.path}</div>
        <div class="t-label">
          Version {otb.version} · {plural(otb.items, "server item")} · last server id {otb.maxServerId} · last client id {otb.maxClientId}
        </div>
      </div>
      {#if pending}
        {#if pending.remapped || pending.updated || pending.added}
          <div>
            Next compile:
            {[
              pending.remapped && `${pending.remapped} moved`,
              pending.updated && `${pending.updated} reloaded`,
              pending.added && `${pending.added} new`,
            ]
              .filter(Boolean)
              .join(", ")}.
          </div>
        {:else}
          <div class="t-label">The items.otb matches the client.</div>
        {/if}
        {#if pending.missing}
          <div class="warn">{plural(pending.missing, "server item")} point to client items that no longer exist.</div>
        {/if}
        {#if pending.error}<div class="warn">{pending.error}</div>{/if}
      {/if}
    {:else}
      <div class="t-label">No items.otb is attached; compiling only writes the client files.</div>
    {/if}
  </div>
  {#snippet footer()}
    {#if otb}
      <button class="t-btn" disabled={!!app.busy} onclick={detach}>Detach</button>
    {/if}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!!app.busy} onclick={choose}>{otb ? "Change…" : "Choose items.otb…"}</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  p {
    margin: 0;
  }
  .info {
    padding: 6px 8px;
    user-select: text;
  }
  .path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .warn {
    color: var(--gold);
  }
</style>
