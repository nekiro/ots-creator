<script lang="ts">
  // Points every use of the selected sprites at another sprite (or clears
  // them with 0). The sprites themselves stay; Optimize removes unused ones.
  import { CATEGORY_NAMES, SpriteService, res } from "../../lib/api";
  import { app, run, toast, versions } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";

  const from = [...app.selectedSprites];
  const total = app.project?.info.counts.sprites ?? 0;
  let to = $state(0);
  let users = $state<string[] | null>(null);

  // Objects that use the sprites (looked up for a reasonable selection only).
  $effect(() => {
    if (from.length > 200) return;
    Promise.all(from.map((id) => SpriteService.Users(id))).then((lists) => {
      const names = new Set<string>();
      for (const list of lists) for (const u of list ?? []) names.add(`${CATEGORY_NAMES[u.category]} ${u.id}`);
      users = [...names];
    }, () => (users = []));
  });

  const fromLabel = from.length > 6 ? `${from.slice(0, 6).join(", ")} and ${from.length - 6} more` : from.join(", ");

  async function replace() {
    const n = await run("Replacing", () => SpriteService.ReplaceRefs(from, to));
    if (n === undefined) return;
    toast(n ? `Updated ${n} object(s).` : "No object uses these sprites.", n ? "success" : "info");
    app.dialog = null;
  }
</script>

<Dialog title="Replace Sprite Uses" width={420} onclose={() => (app.dialog = null)}>
  <div class="col">
    <p>
      Every object that uses sprite <strong>{fromLabel}</strong> will use the sprite below instead. Undo reverts it.
    </p>
    <div class="t-label users">
      {#if from.length > 200}
        Too many sprites to list their objects.
      {:else if users === null}
        Looking for objects…
      {:else if users.length === 0}
        No object uses these sprites.
      {:else}
        Used by {users.length} object(s): {users.slice(0, 8).join(", ")}{users.length > 8 ? "…" : ""}
      {/if}
    </div>
    <div class="row target">
      <span class="t-label">Replace with sprite</span>
      <NumberField value={to} min={0} max={total} width={70} title="0 clears the slots" onchange={(v) => (to = v)} />
      <span class="t-slot">{#if to > 0}<img class="pixel" src={res.spritePng(to, versions.sprite(to))} alt="" />{/if}</span>
      {#if to === 0}<span class="t-label">(empty)</span>{/if}
    </div>
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!!app.busy || from.includes(to)} onclick={replace}>Replace</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .users {
    min-height: 2.4em;
  }
  .target {
    gap: 6px;
  }
  .t-slot img {
    max-width: 32px;
    max-height: 32px;
  }
</style>
