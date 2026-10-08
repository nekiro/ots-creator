<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import { app } from "../../lib/state.svelte";
  import { REPO_URL, updates } from "../../lib/updates.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
</script>

<Dialog title="About" width={380} onclose={() => (app.dialog = null)}>
  <div class="col about">
    <img class="pixel" src="/icon.png" alt="" width="48" height="48" />
    <strong>OTS Creator</strong>
    <span class="t-label">Version {updates.version || "dev"}</span>
    <span>Object editor for Open Tibia clients (Tibia.dat / Tibia.spr, OBD).</span>
    <span>© 2026 Marcin Jałocha (nekiro)</span>
    <button class="link" onclick={() => Browser.OpenURL(REPO_URL)}>{REPO_URL.replace("https://", "")}</button>
    <span class="t-label">Apache License 2.0 with the Commons Clause: free to use and modify, not for sale. Derived works must keep these credits.</span>
    <span class="t-label">UI graphics and fonts are Tibia assets, © CipSoft GmbH.</span>
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" onclick={() => (app.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .about {
    align-items: center;
    text-align: center;
    padding: 6px 0;
  }
  strong {
    color: var(--text-bright);
    font-size: var(--fs);
  }
  .link {
    font: inherit;
    color: var(--gold);
    background: none;
    border: 0;
    padding: 0;
    cursor: pointer;
    text-decoration: underline;
  }
</style>
